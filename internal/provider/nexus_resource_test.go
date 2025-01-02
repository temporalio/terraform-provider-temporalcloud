package provider

import (
	"context"
	"fmt"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestNexusSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewNexusResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccBasicNexusResource(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-basic-nexus", randomString())
	config := func(name string, description string, namespaceID string, taskQueue string, policies []string) string {
		policyConfig := ""
		for _, policy := range policies {
			policyConfig += fmt.Sprintf(`
    {
      allowed_cloud_namespace_policy = {
        namespace_id = "%s"
      }
    }`, policy)
		}

		return fmt.Sprintf(`
provider "temporalcloud" {
  # Configuration options for the Temporal Cloud provider
}

resource "temporalcloud_nexus" "terraform" {
  name        = "%s"
  description = "%s"

  target_endpoint = {
    worker_target_endpoint = {
      target_namespace_id = "%s"
      target_task_queue   = "%s"
    }
  }

  endpoint_policy = [%s]
}`, name, description, namespaceID, taskQueue, policyConfig)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// New Nexus resource with initial configuration
				Config: config(name, "Initial description", "example-namespace-id", "example-task-queue", []string{"allowed-namespace-id-1"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "name", name),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "description", "Initial description"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "target_endpoint.0.worker_target_endpoint.0.target_namespace_id", "example-namespace-id"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "target_endpoint.0.worker_target_endpoint.0.target_task_queue", "example-task-queue"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "endpoint_policy.#", "1"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "endpoint_policy.0.allowed_cloud_namespace_policy.0.namespace_id", "allowed-namespace-id-1"),
				),
			},
			{
				// Update Nexus resource with new configuration
				Config: config(name, "Updated description", "updated-namespace-id", "updated-task-queue", []string{"allowed-namespace-id-2", "allowed-namespace-id-3"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "name", name),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "description", "Updated description"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "target_endpoint.0.worker_target_endpoint.0.target_namespace_id", "updated-namespace-id"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "target_endpoint.0.worker_target_endpoint.0.target_task_queue", "updated-task-queue"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "endpoint_policy.#", "2"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "endpoint_policy.0.allowed_cloud_namespace_policy.0.namespace_id", "allowed-namespace-id-2"),
					resource.TestCheckResourceAttr("temporalcloud_nexus.terraform", "endpoint_policy.1.allowed_cloud_namespace_policy.0.namespace_id", "allowed-namespace-id-3"),
				),
			},
			{
				// Import the Nexus resource and verify the state
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_nexus.terraform",
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}
