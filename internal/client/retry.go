package client

import (
	"context"
	"time"

	"github.com/cloudflare/backoff"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	DefaultMaxDuration = 5 * time.Minute
	DefaultInterval    = 5 * time.Second
)

type (
	retryConfig struct {
		maxDuration time.Duration
		interval    time.Duration
	}
	retryOption func(*retryConfig)
)

func WithMaxDuration(d time.Duration) retryOption {
	return func(c *retryConfig) {
		c.maxDuration = d
	}
}

func WithInterval(d time.Duration) retryOption {
	return func(c *retryConfig) {
		c.interval = d
	}
}

func Retry[Request any, Response any](
	op func(context.Context, *Request, ...grpc.CallOption) (*Response, error),
	ctx context.Context,
	request *Request,
	opts ...retryOption,
) (*Response, error) {

	var (
		resp   *Response
		err    error
		config = &retryConfig{
			maxDuration: DefaultMaxDuration,
			interval:    DefaultInterval,
		}
	)
	for _, opt := range opts {
		opt(config)
	}

	b := backoff.New(config.maxDuration, config.interval)
	for {
		resp, err = op(ctx, request)
		if err == nil {
			return resp, nil
		}
		code := status.Code(err)
		if code != codes.Unavailable && code != codes.ResourceExhausted {
			return resp, err
		}
		<-time.After(b.Duration())
	}
}
