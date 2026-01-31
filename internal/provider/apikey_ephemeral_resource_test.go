package provider

import (
	"context"
	"testing"

	fwephemeral "github.com/hashicorp/terraform-plugin-framework/ephemeral"
)

func TestApiKeyEphemeralSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwephemeral.SchemaRequest{}
	schemaResponse := &fwephemeral.SchemaResponse{}

	NewApiKeyEphemeralResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate required attributes exist
	requiredAttrs := []string{"owner_type", "owner_id", "display_name", "expiry_time"}
	for _, attrName := range requiredAttrs {
		attr, ok := schemaResponse.Schema.Attributes[attrName]
		if !ok {
			t.Errorf("Expected attribute %q to exist", attrName)
			continue
		}
		if !attr.IsRequired() {
			t.Errorf("Expected attribute %q to be required", attrName)
		}
	}

	// Validate computed attributes exist
	computedAttrs := []string{"id", "state", "token"}
	for _, attrName := range computedAttrs {
		attr, ok := schemaResponse.Schema.Attributes[attrName]
		if !ok {
			t.Errorf("Expected attribute %q to exist", attrName)
			continue
		}
		if !attr.IsComputed() {
			t.Errorf("Expected attribute %q to be computed", attrName)
		}
	}

	// Validate token is sensitive
	tokenAttr, ok := schemaResponse.Schema.Attributes["token"]
	if !ok {
		t.Fatal("Expected token attribute to exist")
	}
	if !tokenAttr.IsSensitive() {
		t.Error("Expected token attribute to be sensitive")
	}
}
