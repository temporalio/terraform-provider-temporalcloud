// The MIT License
//
// Copyright (c) 2023 Temporal Technologies Inc.  All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"go.temporal.io/cloud-sdk/cloudclient"
)

// TraceEnabled reports whether gRPC call instrumentation is turned on via the
// TEMPORALCLOUD_TRACE environment variable. It is primarily intended to profile
// slow acceptance tests, where almost all wall-clock time is spent in
// server-side provisioning observed through these gRPC calls.
func TraceEnabled() bool {
	v := os.Getenv("TEMPORALCLOUD_TRACE")
	return v != "" && v != "0" && v != "false"
}

// Tracef writes a trace line to stderr, prefixed for easy grepping. It writes
// directly to os.Stderr rather than via the standard log package because the
// acceptance-test harness redirects the std logger to a TF_LOG-gated sink,
// which would otherwise swallow this output. It is a no-op when tracing is off.
func Tracef(format string, args ...any) {
	if !TraceEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, "[temporalcloud-trace] "+format+"\n", args...)
}

// Client is a cloudclient for the Temporal Cloud API.
type Client struct {
	*cloudclient.Client
}

func NewConnectionWithAPIKey(addrStr string, allowInsecure bool, apiKey string, version string) (*Client, error) {
	userAgentProject := "terraform-provider-temporalcloud"
	if version != "" {
		userAgentProject = fmt.Sprintf("%s/%s", userAgentProject, version)
	}

	opts := cloudclient.Options{
		HostPort:      addrStr,
		APIKey:        apiKey,
		AllowInsecure: allowInsecure,
		UserAgent:     userAgentProject,
	}
	if TraceEnabled() {
		opts.GRPCDialOptions = append(opts.GRPCDialOptions, grpc.WithUnaryInterceptor(traceUnaryInterceptor))
	}

	var cClient *cloudclient.Client
	var err error
	cClient, err = cloudclient.New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	return &Client{cClient}, nil
}

// traceStats accumulates per-method gRPC call counts and durations so a summary
// can be printed at the end of a run.
var traceStats sync.Map // method string -> *methodStat

type methodStat struct {
	calls   atomic.Int64
	totalNs atomic.Int64
}

// traceUnaryInterceptor logs the duration of every gRPC call and accumulates
// per-method aggregates. It is only installed when TEMPORALCLOUD_TRACE is set.
func traceUnaryInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	start := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	elapsed := time.Since(start)

	v, _ := traceStats.LoadOrStore(method, &methodStat{})
	if st, ok := v.(*methodStat); ok {
		st.calls.Add(1)
		st.totalNs.Add(int64(elapsed))
	}

	status := "ok"
	if err != nil {
		status = "err"
	}
	Tracef("grpc %-55s %8.3fs %s", method, elapsed.Seconds(), status)
	return err
}

// LogTraceSummary prints the aggregated per-method gRPC statistics gathered so
// far. It is a no-op when tracing is disabled. Safe to call multiple times.
func LogTraceSummary() {
	if !TraceEnabled() {
		return
	}
	Tracef("===== gRPC call summary =====")
	traceStats.Range(func(k, v any) bool {
		method, ok := k.(string)
		if !ok {
			return true
		}
		st, ok := v.(*methodStat)
		if !ok {
			return true
		}
		calls := st.calls.Load()
		total := time.Duration(st.totalNs.Load())
		avg := time.Duration(0)
		if calls > 0 {
			avg = total / time.Duration(calls)
		}
		Tracef("%-55s calls=%-5d total=%9.3fs avg=%7.3fs", method, calls, total.Seconds(), avg.Seconds())
		return true
	})
}

func AwaitAsyncOperation(ctx context.Context, cloudclient *Client, op *operationv1.AsyncOperation) error {
	if op == nil {
		return fmt.Errorf("failed to await response: nil operation")
	}

	ctx = tflog.SetField(ctx, "operation_id", op.Id)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	start := time.Now()
	polls := 0
	// traceAwait logs how long the operation took and how many times it was
	// polled, attributing wall-clock time to a specific operation type.
	traceAwait := func(outcome string) {
		Tracef("await op %-12s %s polls=%d elapsed=%.3fs", outcome, op.GetOperationType(), polls, time.Since(start).Seconds())
	}

	for {
		select {
		case <-ticker.C:
			polls++
			status, err := cloudclient.CloudService().GetAsyncOperation(ctx, &cloudservicev1.GetAsyncOperationRequest{
				AsyncOperationId: op.Id,
			})
			if err != nil {
				traceAwait("error")
				return fmt.Errorf("failed to query async operation status: %w", err)
			}
			newOp := status.GetAsyncOperation()
			tflog.Debug(ctx, "responded with state", map[string]any{
				"state": newOp.GetState(),
			})

			// https://github.com/temporalio/api-cloud/blob/main/temporal/api/cloud/operation/v1/message.proto#L15
			switch newOp.GetState() {
			case operationv1.AsyncOperation_STATE_PENDING:
				fallthrough
			case operationv1.AsyncOperation_STATE_IN_PROGRESS:
				tflog.Debug(ctx, "retrying in 1 second", map[string]any{
					"state": newOp.GetState(),
				})
				continue
			case operationv1.AsyncOperation_STATE_FAILED:
				tflog.Debug(ctx, "request failed")
				traceAwait("failed")
				return errors.New(newOp.GetFailureReason())
			case operationv1.AsyncOperation_STATE_CANCELLED:
				tflog.Debug(ctx, "request cancelled")
				traceAwait("cancelled")
				return errors.New("request cancelled")
			case operationv1.AsyncOperation_STATE_FULFILLED:
				tflog.Debug(ctx, "request fulfilled, terminating loop")
				traceAwait("fulfilled")
				return nil
			default:
				tflog.Warn(ctx, "unknown state, continuing", map[string]any{
					"state": newOp.GetState(),
				})
				continue
			}
		case <-ctx.Done():
			traceAwait("ctx-done")
			return ctx.Err()
		}
	}
}
