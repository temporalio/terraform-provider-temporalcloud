package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSource_Namespace(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-basic-namespace", randomString(10))
	description := "test-description"
	config := func(name string, retention int, description string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-ca-central-1"]
  api_key_auth 	 = true
  retention_days     = %d
  description        = %q
}

data "temporalcloud_namespace" "terraform" {
  id = temporalcloud_namespace.terraform.id
}

output "namespace" {
  value = data.temporalcloud_namespace.terraform
}
`, name, retention, description)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, 14, description),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["namespace"]
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
						return fmt.Errorf("expected namespace name to be: %s, got: %s", name, outputName)
					}

					outputAPIKey, ok := outputValue["api_key_auth"].(bool)
					if !ok {
						return fmt.Errorf("expected api_key_auth to be a boolean")
					}
					if !outputAPIKey {
						return fmt.Errorf("expected api_key_auth to be true")
					}

					outputRegion, ok := outputValue["active_region"].(string)
					if !ok {
						return fmt.Errorf("expected active_region to be a string")
					}
					if outputRegion != "aws-ca-central-1" {
						return fmt.Errorf("exptect active regon to match provided region")
					}

					outputDescription, ok := outputValue["description"].(string)
					if !ok {
						return fmt.Errorf("expected description to be a string")
					}
					if outputDescription != description {
						return fmt.Errorf("expected namespace description to be: %s, got: %s", description, outputDescription)
					}

					return nil
				},
			},
		},
	})
}
