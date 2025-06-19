package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSource_NexusEndpoint(t *testing.T) {
	name := createRandomName()
	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "target_namespace" {
  name           = "terraform-target-namespace"
  regions        = ["aws-us-west-2"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_namespace" "caller_namespace" {
  name           = "terraform-caller-namespace"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_namespace" "caller_namespace_2" {
  name           = "terraform-caller-namespace-2"
  regions        = ["gcp-us-central1"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_nexus_endpoint" "nexus_endpoint" {
  name        = "%s"
  description = <<-EOT
    Service Name:
      my-hello-service
    Operation Names:
      echo
      say-hello

    Input / Output arguments are in the following repository:
    https://github.com/temporalio/samples-go/blob/main/nexus/service/api.go
  EOT
  worker_target = {
    namespace_id = temporalcloud_namespace.target_namespace.id
    task_queue   = "terraform-task-queue"
  }
  allowed_caller_namespaces = [
    temporalcloud_namespace.caller_namespace.id,
    temporalcloud_namespace.caller_namespace_2.id,
  ]
}

data "temporalcloud_nexus_endpoint" "example" {
  id = temporalcloud_nexus_endpoint.nexus_endpoint.id
}

output "nexus_endpoint" {
  value = data.temporalcloud_nexus_endpoint.example
  sensitive = true
}
`, name)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["nexus_endpoint"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputName, ok := outputValue["name"].(string)
					if !ok {
						return fmt.Errorf("expected value to be a string")
					}
					if outputName != name {
						return fmt.Errorf("expected nexus endpoint name to be %s, got %s", name, outputName)
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected value to be a string")
					}
					if outputState != "active" {
						return fmt.Errorf("expected nexus endpoint state to be active, got %s", outputState)
					}

					return nil
				},
			},
		},
	})
}
