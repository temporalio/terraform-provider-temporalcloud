package client

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StatusCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	return status.Code(err)
}
