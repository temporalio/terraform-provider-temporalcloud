package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSource_NexusEndpoints(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNexusEndpointsConfig(),
			},
		},
	})
}

func testAccNexusEndpointsConfig() string {
	return `
provider "temporalcloud" {

}

data "temporalcloud_nexus_endpoints" "my_nexus_endpoints" {}

output "nexus_endpoints" {
  value = data.temporalcloud_nexus_endpoints.my_nexus_endpoints.nexus_endpoints
  sensitive = true
}
`
}
