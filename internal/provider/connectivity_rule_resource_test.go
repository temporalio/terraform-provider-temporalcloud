package provider

import (
	"context"
	"regexp"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestConnectivityRuleSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewConnectivityRuleResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccConnectivityRuleResource_Public(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a public connectivity rule
			{
				Config: testAccConnectivityRuleResourceConfig_Public(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_connectivity_rule.test_public", "connectivity_type", "public"),
					resource.TestCheckResourceAttr("temporalcloud_connectivity_rule.test_public", "connection_id", ""),
					resource.TestCheckResourceAttr("temporalcloud_connectivity_rule.test_public", "region", ""),
					resource.TestCheckResourceAttr("temporalcloud_connectivity_rule.test_public", "gcp_project_id", ""),
				),
			},
			// Import state testing
			{
				ResourceName:      "temporalcloud_connectivity_rule.test_public",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete the public connectivity rule
			{
				ResourceName:      "temporalcloud_connectivity_rule.test_public",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}

func TestAccConnectivityRuleResource_AWS_Private(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectivityRuleResourceConfig_Private(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_connectivity_rule.test_aws_private", "connectivity_type", "private"),
					resource.TestCheckResourceAttr("temporalcloud_connectivity_rule.test_aws_private", "connection_id", "vpce-12345678"),
					resource.TestCheckResourceAttr("temporalcloud_connectivity_rule.test_aws_private", "region", "aws-us-west-2"),
				),
			},
			// Import state testing
			{
				ResourceName:      "temporalcloud_connectivity_rule.test_aws_private",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete the private connectivity rule
			{
				ResourceName:      "temporalcloud_connectivity_rule.test_aws_private",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}

func TestAccConnectivityRuleResource_ValidationErrors(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectivityRuleResourceConfig_InvalidConnectivityType(),
				// Should fail validation before reaching the API
				ExpectError: regexp.MustCompile("must be one of"),
			},
			{
				Config: testAccConnectivityRuleResourceConfig_PrivateWithoutConnectionId(),
				// Should fail at plan time due to missing required attribute
				ExpectError: regexp.MustCompile("private connection id is empty"),
			},
			{
				Config: testAccConnectivityRuleResourceConfig_PrivateWithoutRegion(),
				// Should fail at plan time due to missing required attribute
				ExpectError: regexp.MustCompile("region is empty"),
			},
			{
				Config: testAccConnectivityRuleResourceConfig_PrivateWithoutGcpProjectId(),
				// Should fail at plan time due to missing required attribute
				ExpectError: regexp.MustCompile("gcp project id is required"),
			},
		},
	})
}

// Test configuration functions.
func testAccConnectivityRuleResourceConfig_Public() string {
	return `
provider "temporalcloud" {

}

resource "temporalcloud_connectivity_rule" "test_public" {
  connectivity_type = "public"
}
`
}

func testAccConnectivityRuleResourceConfig_Private() string {
	return `
provider "temporalcloud" {

}

resource "temporalcloud_connectivity_rule" "test_aws_private" {
  connectivity_type = "private"
  connection_id     = "vpce-12345678"
  region           = "aws-us-west-2"
}
`
}

func testAccConnectivityRuleResourceConfig_InvalidConnectivityType() string {
	return `
provider "temporalcloud" {
}

resource "temporalcloud_connectivity_rule" "test" {
  connectivity_type = "invalid"
  connection_id     = "dummy-connection-id"
  region           = "aws-us-west-2"
}
`
}

func testAccConnectivityRuleResourceConfig_PrivateWithoutConnectionId() string {
	return `
provider "temporalcloud" {

}

resource "temporalcloud_connectivity_rule" "test" {
  connectivity_type = "private"
  region           = "aws-us-west-2"
}
`
}

func testAccConnectivityRuleResourceConfig_PrivateWithoutRegion() string {
	return `
provider "temporalcloud" {

}

resource "temporalcloud_connectivity_rule" "test" {
  connectivity_type = "private"
  connection_id     = "vpce-12345678"
}
`
}

func testAccConnectivityRuleResourceConfig_PrivateWithoutGcpProjectId() string {
	return `
provider "temporalcloud" {

}

resource "temporalcloud_connectivity_rule" "test" {
  connectivity_type = "private"
  connection_id     = "vpce-12345678"
  region           = "gcp-us-central1"
}
`
}
