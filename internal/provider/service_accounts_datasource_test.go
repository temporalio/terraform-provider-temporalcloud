package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServiceAccounts(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountsConfig(),
			},
		},
	})
}

func testAccServiceAccountsConfig() string {
	return `
provider "temporalcloud" {

}

data "temporalcloud_service_accounts" "example" {}
`
}
