package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSource_ServiceAccount(t *testing.T) {
	name := createRandomName()
	config := func(name string, role string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "%s"
  account_access = "%s"
}

data "temporalcloud_service_account" "terraform" {
  id = temporalcloud_service_account.terraform.id
}

output "service_account" {
  value = data.temporalcloud_service_account.terraform
}
`, name, role)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, "read"),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["service_account"]
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
						return fmt.Errorf("expected service account name to be %s, got %s", name, outputName)
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected value to be a string")
					}
					if outputState != "active" {
						return fmt.Errorf("expected service account state to be active, got %s", outputState)
					}

					return nil
				},
			},
		},
	})
}
