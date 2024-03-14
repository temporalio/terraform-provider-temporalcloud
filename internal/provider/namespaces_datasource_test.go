package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNamespaces(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNamespacesConfig(),
			},
		},
	})
}

func testAccNamespacesConfig() string {
	return `
provider "temporalcloud" {

}

data "temporalcloud_namespaces" "test" {}
`
}
