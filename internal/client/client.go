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
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/temporalio/tcld/protogen/api/accountservice/v1"
	"github.com/temporalio/tcld/protogen/api/request/v1"
	"github.com/temporalio/tcld/protogen/api/requestservice/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	cloudservicev1 "github.com/temporalio/terraform-provider-temporalcloud/proto/go/temporal/api/cloud/cloudservice/v1"
	operationv1 "github.com/temporalio/terraform-provider-temporalcloud/proto/go/temporal/api/cloud/operation/v1"
)

const (
	TemporalCloudAPIVersionHeader = "temporal-cloud-api-version"
	LegacyTemporalCloudAPIVersion = "2024-03-18-00"
	TemporalCloudAPIVersion       = "2024-05-13-00"
)

var (
	TemporalCloudAPIMethodRegex = regexp.MustCompile(`^\/temporal\.api\.cloud\.cloudservice\.v1\.CloudService\/[^\/]*$`)
)

// Store is a client for the Temporal Cloud API.
type Store struct {
	c cloudservicev1.CloudServiceClient
	a accountservice.AccountServiceClient
	r requestservice.RequestServiceClient
}

func (c *Store) CloudServiceClient() cloudservicev1.CloudServiceClient {
	return c.c
}

func (c *Store) AccountServiceClient() accountservice.AccountServiceClient {
	return c.a
}

func (c *Store) RequestServiceClient() requestservice.RequestServiceClient {
	return c.r
}

func NewConnectionWithAPIKey(addrStr string, allowInsecure bool, apiKey string, opts ...grpc.DialOption) (*Store, error) {
	return newConnection(
		addrStr,
		allowInsecure,
		append(opts, grpc.WithPerRPCCredentials(NewAPIKeyRPCCredential(apiKey, allowInsecure)))...,
	)
}

func newConnection(addrStr string, allowInsecure bool, opts ...grpc.DialOption) (*Store, error) {
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
	accountClient := accountservice.NewAccountServiceClient(conn)
	reqClient := requestservice.NewRequestServiceClient(conn)
	return &Store{c: cloudClient, a: accountClient, r: reqClient}, nil
}

func AwaitAsyncOperation(ctx context.Context, client cloudservicev1.CloudServiceClient, op *operationv1.AsyncOperation) error {
	if op == nil {
		return fmt.Errorf("failed to await response: nil operation")
	}

	ctx = tflog.SetField(ctx, "operation_id", op.Id)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			status, err := client.GetAsyncOperation(ctx, &cloudservicev1.GetAsyncOperationRequest{
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

func AwaitRequestStatus(ctx context.Context, c requestservice.RequestServiceClient, op *request.RequestStatus) error {
	if op == nil {
		return fmt.Errorf("failed to await response: nil operation")
	}
	ctx = tflog.SetField(ctx, "operation_id", op.GetRequestId())
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			status, err := c.GetRequestStatus(ctx, &requestservice.GetRequestStatusRequest{
				RequestId: op.GetRequestId(),
			})
			if err != nil {
				return fmt.Errorf("failed to query request status: %w", err)
			}
			newOp := status.GetRequestStatus()
			tflog.Debug(ctx, "responded with state", map[string]any{
				"state": newOp.GetState().String(),
			})

			// https://github.com/temporalio/tcld/blob/main/protogen/api/request/v1/message.pb.go#L31
			switch newOp.GetState() {
			case request.STATE_PENDING:
			case request.STATE_IN_PROGRESS:
				tflog.Debug(ctx, "retrying in 1 second", map[string]any{
					"state": newOp.GetState().String(),
				})
				continue
			case request.STATE_FAILED:
				tflog.Debug(ctx, "request failed")
				return errors.New(newOp.GetFailureReason())
			case request.STATE_CANCELLED:
				tflog.Debug(ctx, "request cancelled")
				return errors.New("request cancelled")
			case request.STATE_FULFILLED:
				tflog.Debug(ctx, "request fulfilled, terminating loop")
				return nil
			default:
				tflog.Warn(ctx, "unknown state, continuing", map[string]any{
					"state": newOp.GetState().String(),
				})
				continue
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

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
	if TemporalCloudAPIMethodRegex.MatchString(method) {
		ctx = metadata.AppendToOutgoingContext(ctx, TemporalCloudAPIVersionHeader, TemporalCloudAPIVersion)
	} else {
		ctx = metadata.AppendToOutgoingContext(ctx, TemporalCloudAPIVersionHeader, LegacyTemporalCloudAPIVersion)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}
