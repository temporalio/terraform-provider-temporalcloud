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
	//"crypto/tls"
	"errors"
	"fmt"

	//"net/url"
	//"strings"
	"time"

	//grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	operationv1 "go.temporal.io/api/cloud/operation/v1"
	"go.temporal.io/sdk/client"
)

const TemporalCloudAPIVersionHeader = "temporal-cloud-api-version"

var TemporalCloudAPIVersion = "2023-10-01-00"

// Client is a client for the Temporal Cloud API.
type Client struct {
	client.CloudOperationsClient
}

var (
	// _ cloudservicev1.CloudServiceClient = &Client{}
	_ client.CloudOperationsClient = &Client{}
)

// func NewConnectionWithAPIKey(addrStr string, allowInsecure bool, apiKey string, opts ...grpc.DialOption) (*Client, error) {
func NewConnectionWithAPIKey(addrStr string, allowInsecure bool, apiKey string) (*Client, error) {
	/*defaultOpts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(NewAPIKeyRPCCredential(apiKey, allowInsecure)),
		grpc.WithChainUnaryInterceptor(
			grpcretry.UnaryClientInterceptor(
				grpcretry.WithBackoff(
					grpcretry.BackoffExponentialWithJitter(250*time.Millisecond, 0.1),
				),
				grpcretry.WithMax(5),
			),
		),
	}

	opts = append(defaultOpts, opts...)

	return newConnection(
		addrStr,
		allowInsecure,
		opts...,
	)*/
	var cClient client.CloudOperationsClient
	var err error
	cClient, err = client.DialCloudOperationsClient(context.Background(), client.CloudOperationsClientOptions{
		Version:     TemporalCloudAPIVersion,
		Credentials: client.NewAPIKeyStaticCredentials(apiKey),
		DisableTLS:  allowInsecure,
		HostPort:    addrStr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect `%s`: %v", client.DefaultHostPort, err)
	}

	return &Client{cClient}, nil
}

/*
func newConnection(addrStr string, allowInsecure bool, opts ...grpc.DialOption) (*Client, error) {
	addr, err := url.Parse(addrStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse server address: %s", err)
	}
	defaultOpts := defaultDialOptions(addr, allowInsecure)
	conn, err := grpc.Dial(
		addr.String(),
		append(defaultOpts, opts...)...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial `%s`: %v", addr.String(), err)
	}

	cloudClient := cloudservicev1.NewCloudServiceClient(conn)
	return &Client{CloudServiceClient: cloudClient}, nil
} */

func AwaitAsyncOperation(ctx context.Context, client client.CloudOperationsClient, op *operationv1.AsyncOperation) error {
	if op == nil {
		return fmt.Errorf("failed to await response: nil operation")
	}

	ctx = tflog.SetField(ctx, "operation_id", op.Id)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			status, err := client.CloudService().GetAsyncOperation(ctx, &cloudservicev1.GetAsyncOperationRequest{
				AsyncOperationId: op.Id,
			})
			if err != nil {
				return fmt.Errorf("failed to query async operation status: %w", err)
			}
			newOp := status.GetAsyncOperation()
			tflog.Debug(ctx, "responded with state", map[string]any{
				"state": newOp.GetState(),
			})

			// https://github.com/temporalio/api-cloud/blob/main/temporal/api/cloud/operation/v1/message.proto#L15
			switch newOp.GetState() {
			case "pending":
			case "in_progress":
				tflog.Debug(ctx, "retrying in 1 second", map[string]any{
					"state": newOp.GetState(),
				})
				continue
			case "failed":
				tflog.Debug(ctx, "request failed")
				return errors.New(newOp.GetFailureReason())
			case "cancelled":
				tflog.Debug(ctx, "request cancelled")
				return errors.New("request cancelled")
			case "fulfilled":
				tflog.Debug(ctx, "request fulfilled, terminating loop")
				return nil
			default:
				tflog.Warn(ctx, "unknown state, continuing", map[string]any{
					"state": newOp.GetState(),
				})
				continue
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

/*
func defaultDialOptions(addr *url.URL, allowInsecure bool) []grpc.DialOption {
	var opts []grpc.DialOption

	transport := credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: addr.Hostname(),
	})
	if allowInsecure {
		transport = insecure.NewCredentials()
	}

	opts = append(opts, grpc.WithTransportCredentials(transport))
	opts = append(opts, grpc.WithUnaryInterceptor(setAPIVersionInterceptor))
	return opts
}

func setAPIVersionInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	ctx = metadata.AppendToOutgoingContext(ctx, TemporalCloudAPIVersionHeader, strings.TrimSpace(TemporalCloudAPIVersion))
	return invoker(ctx, method, req, reply, cc, opts...)
}
*/
