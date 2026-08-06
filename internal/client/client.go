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

	"github.com/hashicorp/terraform-plugin-log/tflog"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"go.temporal.io/cloud-sdk/cloudclient"
)

const DevelopmentAPIVersion = "development"

// Client is a cloudclient for the Temporal Cloud API.
type Client struct {
	*cloudclient.Client
	developmentClient *cloudclient.Client
}

func NewConnectionWithAPIKey(addrStr string, allowInsecure bool, apiKey string, version string) (*Client, error) {
	userAgentProject := "terraform-provider-temporalcloud"
	if version != "" {
		userAgentProject = fmt.Sprintf("%s/%s", userAgentProject, version)
	}

	cClient, err := newCloudClient(addrStr, allowInsecure, apiKey, userAgentProject, "")
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	developmentClient, err := newCloudClient(addrStr, allowInsecure, apiKey, userAgentProject, DevelopmentAPIVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to connect with development API version: %v", err)
	}

	return &Client{
		Client:            cClient,
		developmentClient: developmentClient,
	}, nil
}

func (c *Client) DevelopmentCloudService() cloudservicev1.CloudServiceClient {
	return c.developmentClient.CloudService()
}

func newCloudClient(addrStr string, allowInsecure bool, apiKey string, userAgent string, apiVersion string) (*cloudclient.Client, error) {
	return cloudclient.New(cloudclient.Options{
		HostPort:      addrStr,
		APIKey:        apiKey,
		AllowInsecure: allowInsecure,
		UserAgent:     userAgent,
		APIVersion:    apiVersion,
	})
}

func AwaitAsyncOperation(ctx context.Context, cloudclient *Client, op *operationv1.AsyncOperation) error {
	if op == nil {
		return fmt.Errorf("failed to await response: nil operation")
	}

	ctx = tflog.SetField(ctx, "operation_id", op.Id)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			status, err := cloudclient.CloudService().GetAsyncOperation(ctx, &cloudservicev1.GetAsyncOperationRequest{
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
