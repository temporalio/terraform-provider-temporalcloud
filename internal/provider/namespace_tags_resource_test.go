package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestNamespaceTagsSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewNamespaceTagsResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)
	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccNamespaceTagsResource(t *testing.T) {
	name := fmt.Sprintf("tf-namespace-tags-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNamespaceTagsConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_tags.test", "tags.%", "2"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_tags.test", "tags.environment", "test"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_tags.test", "tags.team", "platform"),
				),
			},
			{
				ResourceName:      "temporalcloud_namespace_tags.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccNamespaceTagsConfigUpdated(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_tags.test", "tags.%", "1"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_tags.test", "tags.environment", "production"),
				),
			},
		},
	})
}

func TestAccNamespaceTagsResource_ValidationErrors(t *testing.T) {
	name := fmt.Sprintf("tf-namespace-tags-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccNamespaceTagsConfigEmpty(name),
				ExpectError: regexp.MustCompile("Attribute tags map must contain at least 1 elements"),
			},
		},
	})
}

func testAccNamespaceTagsConfig(name string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" "test" {
  name           = "%s"
  regions        = ["aws-us-east-1"]
  retention_days = 1
  api_key_auth = true
}

resource "temporalcloud_namespace_tags" "test" {
  namespace_id = temporalcloud_namespace.test.id
  tags = {
    environment = "test"
    team        = "platform"
  }
}
`, name)
}

func testAccNamespaceTagsConfigUpdated(name string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" "test" {
  name           = "%s"
  regions        = ["aws-us-east-1"]
  retention_days = 1
  api_key_auth = true
}

resource "temporalcloud_namespace_tags" "test" {
  namespace_id = temporalcloud_namespace.test.id
  tags = {
    environment = "production"
  }
}
`, name)
}

func testAccNamespaceTagsConfigEmpty(name string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" "test" {
  name           = "%s"
  regions        = ["aws-us-east-1"]
  retention_days = 1
  api_key_auth = true
}

resource "temporalcloud_namespace_tags" "test" {
  namespace_id = temporalcloud_namespace.test.id
  tags = {}
}
`, name)
}
