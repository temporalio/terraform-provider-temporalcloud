package client

import (
	"go.temporal.io/api/serviceerror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StatusCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}

	if serviceErr, ok := err.(serviceerror.ServiceError); ok {
		st := serviceErr.Status()
		if st != nil {
			return st.Code()
		}
	}

	return status.Code(err)
}
