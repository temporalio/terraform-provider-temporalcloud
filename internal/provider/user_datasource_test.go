package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSource_User(t *testing.T) {
	email := createRandomEmail()
	config := func(email string, role string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_user" "terraform" {
  email = "%s"
  account_access = "%s"
}

data "temporalcloud_user" "terraform" {
  id = temporalcloud_user.terraform.id
}

output "user" {
  value = data.temporalcloud_user.terraform
}
`, email, role)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(email, "read"),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["user"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]any)
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputName, ok := outputValue["email"].(string)
					if !ok {
						return fmt.Errorf("expected value to be a string")
					}
					if outputName != email {
						return fmt.Errorf("expected user email to be %s, got %s", email, outputName)
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected value to be a string")
					}
					if outputState != "active" {
						return fmt.Errorf("expected user state to be active, got %s", outputState)
					}

					return nil
				},
			},
		},
	})
}
