package provider

import (
	"context"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestMetricsEndpointSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewMetricsEndpointResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)
	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}

	if schemaResponse.Schema.DeprecationMessage != metricsEndpointDeprecationMessage {
		t.Fatalf("expected the resource to be deprecated, got DeprecationMessage %q", schemaResponse.Schema.DeprecationMessage)
	}
}

func TestMetricsEndpointCreateFailsWithoutCallingAPI(t *testing.T) {
	t.Parallel()

	r := &metricsEndpointResource{}
	resp := &fwresource.CreateResponse{}

	r.Create(context.Background(), fwresource.CreateRequest{}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected Create to return an error")
	}
	errs := resp.Diagnostics.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %d: %+v", len(errs), errs)
	}
	if got := errs[0].Summary(); got != "The PromQL metrics endpoint is no longer supported." {
		t.Fatalf("unexpected error summary %q", got)
	}
	if got := errs[0].Detail(); got != metricsEndpointDeprecationMessage {
		t.Fatalf("unexpected error detail %q", got)
	}
}

func TestMetricsEndpointUpdateFailsWithoutCallingAPI(t *testing.T) {
	t.Parallel()

	r := &metricsEndpointResource{}
	resp := &fwresource.UpdateResponse{}

	r.Update(context.Background(), fwresource.UpdateRequest{}, resp)

	errs := resp.Diagnostics.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %d: %+v", len(errs), errs)
	}
	if got := errs[0].Summary(); got != "The PromQL metrics endpoint is no longer supported." {
		t.Fatalf("unexpected error summary %q", got)
	}
	if got := errs[0].Detail(); got != metricsEndpointDeprecationMessage {
		t.Fatalf("unexpected error detail %q", got)
	}
}
