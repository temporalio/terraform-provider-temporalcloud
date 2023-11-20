package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc/credentials"
)

const (
	AuthorizationHeader       = "Authorization"
	AuthorizationHeaderPrefix = "Bearer"
)

type (
	apikeyCredential struct {
		apikey        string
		allowInsecure bool
	}
)

func NewAPIKeyRPCCredential(apikey string, allowInsecure bool) credentials.PerRPCCredentials {
	return &apikeyCredential{
		apikey:        apikey,
		allowInsecure: allowInsecure,
	}
}

func (c apikeyCredential) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	ri, ok := credentials.RequestInfoFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve request info from context")
	}
	if !c.allowInsecure {
		// Ensure the API key, AKA bearer token, is sent over a secure connection - meaning TLS.
		if err := credentials.CheckSecurityLevel(ri.AuthInfo, credentials.PrivacyAndIntegrity); err != nil {
			return nil, fmt.Errorf("the connection's transport security level is too low for API keys: %v", err)
		}
	}
	return map[string]string{
		AuthorizationHeader: fmt.Sprintf("%s %s", AuthorizationHeaderPrefix, c.apikey),
	}, nil
}

func (c apikeyCredential) RequireTransportSecurity() bool {
	return !c.allowInsecure
}
