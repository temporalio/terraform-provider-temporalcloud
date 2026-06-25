package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

// TestMain runs the test suite and, when TEMPORALCLOUD_TRACE is set, prints an
// aggregated summary of all gRPC calls made during the run. This is useful for
// profiling why acceptance tests are slow.
func TestMain(m *testing.M) {
	code := m.Run()
	client.LogTraceSummary()
	os.Exit(code)
}

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"temporalcloud": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
}

func TestProviderSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := provider.SchemaRequest{}
	schemaResponse := &provider.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	new(TerraformCloudProvider).Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}
