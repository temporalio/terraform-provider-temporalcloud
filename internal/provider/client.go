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

package provider

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/temporalio/tcld/app/credentials/apikey"
	"github.com/temporalio/tcld/protogen/api/authservice/v1"
	"github.com/temporalio/tcld/protogen/api/namespaceservice/v1"
	"github.com/temporalio/tcld/protogen/api/request/v1"
	"github.com/temporalio/tcld/protogen/api/requestservice/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client is a client for the Temporal Cloud API.
type Client struct {
	conn *grpc.ClientConn
}

// NewClient creates a new client for the Temporal Cloud API using the given API key.
//
// The account ID parameter should be temporary until there is a way for this provider to discover the Account ID
// via API.
func NewClient(apiKey string) (*Client, error) {
	apiKeyCreds, err := apikey.NewCredential(apiKey)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial("saas-api.tmprl.cloud:443",
		grpc.WithPerRPCCredentials(apiKeyCreds),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: "saas-api.tmprl.cloud",
		})))
	if err != nil {
		return nil, err
	}

	return &Client{conn: conn}, nil
}

func (c *Client) NamespaceService() namespaceservice.NamespaceServiceClient {
	return namespaceservice.NewNamespaceServiceClient(c.conn)
}

func (c *Client) RequestService() requestservice.RequestServiceClient {
	return requestservice.NewRequestServiceClient(c.conn)
}

func (c *Client) AuthService() authservice.AuthServiceClient {
	return authservice.NewAuthServiceClient(c.conn)
}

func (c *Client) AwaitResponse(ctx context.Context, requestID string) error {
	ctx = tflog.SetField(ctx, "request_id", requestID)
	svc := c.RequestService()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	tflog.Debug(ctx, "awaiting response")
	for {
		select {
		case <-ticker.C:
			status, err := svc.GetRequestStatus(ctx, &requestservice.GetRequestStatusRequest{
				RequestId: requestID,
			})
			if err != nil {
				return fmt.Errorf("failed to query request status: %w", err)
			}

			tflog.Debug(ctx, "responded with state", map[string]any{
				"state": status.RequestStatus.State.String(),
			})
			switch status.RequestStatus.State {
			case request.STATE_PENDING:
			case request.STATE_IN_PROGRESS:
			case request.STATE_UNSPECIFIED:
				tflog.Debug(ctx, "retrying in 1 second", map[string]any{
					"state": status.RequestStatus.State.String(),
				})
				continue
			case request.STATE_FAILED:
				tflog.Debug(ctx, "request failed")
				return errors.New(status.RequestStatus.FailureReason)
			case request.STATE_CANCELLED:
				tflog.Debug(ctx, "request cancelled")
				return errors.New("request cancelled")
			case request.STATE_FULFILLED:
				tflog.Debug(ctx, "request fulfilled, terminating loop")
				return nil
			}
			// check response
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
