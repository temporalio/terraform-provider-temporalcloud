package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccDataSource_SCIM_Group(t *testing.T) {
	name := "scim_test"
	idpID := "tf-basic-scim-group"
	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {
}

data "temporalcloud_scim_group" "terraform" {
	idp_id = "%s"
}
output "scim_group" {
	value = data.temporalcloud_scim_group.terraform
}
	`, idpID)
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

					output, ok := s.RootModule().Outputs["scim_group"]
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
						return fmt.Errorf("expected scim group name to be: %s, got: %s", name, outputName)
					}
					return nil
				},
			},
		},
	})
}
