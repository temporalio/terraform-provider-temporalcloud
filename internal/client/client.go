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
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc"

	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	operationv1 "go.temporal.io/api/cloud/operation/v1"
	"go.temporal.io/sdk/client"
)

var TemporalCloudAPIVersion = "2024-10-01-00"

// Client is a client for the Temporal Cloud API.
type Client struct {
	client.CloudOperationsClient
}

var (
	_ client.CloudOperationsClient = &Client{}
)

func NewConnectionWithAPIKey(addrStr string, allowInsecure bool, apiKey string) (*Client, error) {

	var cClient client.CloudOperationsClient
	var err error
	cClient, err = client.DialCloudOperationsClient(context.Background(), client.CloudOperationsClientOptions{
		Version:     TemporalCloudAPIVersion,
		Credentials: client.NewAPIKeyStaticCredentials(apiKey),
		DisableTLS:  allowInsecure,
		HostPort:    addrStr,
		ConnectionOptions: client.ConnectionOptions{
			DialOptions: []grpc.DialOption{
				// Make sure to keep this a chain interceptor and make this a inner interceptor.
				// This will make sure to exercise this retry before the retry interceptor in the SDK.
				// TODO (abhinav): Move this retry interceptor to the SDK.
				grpc.WithChainUnaryInterceptor(
					grpcretry.UnaryClientInterceptor(
						// max backoff = 64s (+/- 32s jitter)
						grpcretry.WithBackoff(
							grpcretry.BackoffExponentialWithJitter(500*time.Millisecond, 0.5),
						),
						grpcretry.WithMax(7),
					),
				),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect `%s`: %v", client.DefaultHostPort, err)
	}

	return &Client{cClient}, nil
}

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
			case operationv1.AsyncOperation_STATE_PENDING:
				fallthrough
			case operationv1.AsyncOperation_STATE_IN_PROGRESS:
				tflog.Debug(ctx, "retrying in 1 second", map[string]any{
					"state": newOp.GetState(),
				})
				continue
			case operationv1.AsyncOperation_STATE_FAILED:
				tflog.Debug(ctx, "request failed")
				return errors.New(newOp.GetFailureReason())
			case operationv1.AsyncOperation_STATE_CANCELLED:
				tflog.Debug(ctx, "request cancelled")
				return errors.New("request cancelled")
			case operationv1.AsyncOperation_STATE_FULFILLED:
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
