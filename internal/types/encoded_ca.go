package types

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ basetypes.StringTypable                    = EncodedCAType{}
	_ basetypes.StringValuable                   = EncodedCAValue{}
	_ basetypes.StringValuableWithSemanticEquals = EncodedCAValue{}
)

type EncodedCAType struct {
	basetypes.StringType
}

func (t EncodedCAType) Equal(o attr.Type) bool {
	other, ok := o.(EncodedCAType)
	if !ok {
		return false
	}

	return t.StringType.Equal(other.StringType)
}

func (t EncodedCAType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return EncodedCA(in.ValueString()), nil
}

func (t EncodedCAType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, errors.New("unexpected value type")
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable, nil
}

type EncodedCAValue struct {
	basetypes.StringValue
}

func (v EncodedCAValue) Equal(o attr.Value) bool {
	other, ok := o.(EncodedCAValue)
	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

func (v EncodedCAValue) Type(ctx context.Context) attr.Type {
	return EncodedCAType{}
}

func EncodedCA(val string) EncodedCAValue {
	return EncodedCAValue{
		StringValue: basetypes.NewStringValue(val),
	}
}

// normalizeCAString accepts a base64-encoded PEM string and normalizes it by removing extraneous whitespace. The
// Temporal API will do this and cause a mismatch in the state if we don't also do it here.
func normalizeCAString(certPEMBase64 string) (string, error) {
	certs, err := parseEncodedCertificates(certPEMBase64)
	if err != nil {
		return "", err
	}

	var result []byte
	for _, c := range certs {
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
		result = append(result, pemBytes...)
	}

	return base64.StdEncoding.EncodeToString(result), nil
}

func parseEncodedCertificates(certPEMBase64 string) ([]*x509.Certificate, error) {
	certPEMBytes, err := base64.StdEncoding.DecodeString(certPEMBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64-encoded PEM: %w", err)
	}

	if len(certPEMBytes) == 0 {
		return nil, fmt.Errorf("no certificates received")
	}

	var blocks []byte
	cursor := certPEMBytes
	for {
		var block *pem.Block
		block, cursor = pem.Decode(cursor)
		if block == nil {
			break
		}

		blocks = append(blocks, block.Bytes...)
	}

	// If p is greater than 0, then this means that there was a portion of the certificate that
	// is/was malformed.
	if len(cursor) > 0 && strings.TrimSpace(string(cursor)) != "" {
		return []*x509.Certificate{}, errors.New("malformed certificates")
	}
	return x509.ParseCertificates(blocks)
}

func (v EncodedCAValue) StringSemanticEquals(ctx context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(EncodedCAValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", newValuable),
		)

		return false, diags
	}

	// Normalize the certificate strings before comparing them.
	normalizedV, err := normalizeCAString(v.ValueString())
	if err != nil {
		diags.AddError("Certificate Normalization Error", "Failed to normalize the existing certificate: "+err.Error())
		return false, diags
	}

	normalizedNewValue, err := normalizeCAString(newValue.ValueString())
	if err != nil {
		// The new value may not be a valid CA string. This will get rejected elsewhere in the plan. Since this is just
		// an equality check, we should return false and continue.
		return false, diags
	}

	return normalizedV == normalizedNewValue, diags
}
