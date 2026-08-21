package provider

import (
	"context"
	"fmt"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestProjectSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewProjectResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccBasicProject(t *testing.T) {
	config := func(displayName string, extraAttrs string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_project" "terraform" {
  display_name = "%s"
  %s
}`, displayName, extraAttrs)
	}

	name := createRandomName()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "display_name", name),
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "description", ""),
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "enable_delete_protection", "false"),
					resource.TestCheckResourceAttrSet("temporalcloud_project.terraform", "id"),
					resource.TestCheckResourceAttrSet("temporalcloud_project.terraform", "state"),
				),
			},
			{
				// display_name is mutable and must not force replacement.
				Config: config(name+"-renamed", `description = "This is a test description"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "display_name", name+"-renamed"),
					resource.TestCheckResourceAttr("temporalcloud_project.terraform", "description", "This is a test description"),
				),
			},
			{
				// Dropping description falls back to the "" default.
				Config: config(name+"-renamed", ""),
				Check: resource.TestCheckResourceAttr(
					"temporalcloud_project.terraform", "description", ""),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_project.terraform",
			},
		},
	})
}

func TestAccProject_DeleteProtection(t *testing.T) {
	config := func(displayName string, enableDeleteProtection bool) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_project" "terraform" {
  display_name             = "%s"
  enable_delete_protection = %t
}`, displayName, enableDeleteProtection)
	}

	name := createRandomName()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, true),
				Check: resource.TestCheckResourceAttr(
					"temporalcloud_project.terraform", "enable_delete_protection", "true"),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_project.terraform",
			},
			{
				// Protection must be lifted before the test framework can destroy the Project.
				Config: config(name, false),
				Check: resource.TestCheckResourceAttr(
					"temporalcloud_project.terraform", "enable_delete_protection", "false"),
			},
		},
	})
}
