package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func createRandomApiKeyName() string {
	return fmt.Sprintf("key-terraformprovider-%s", randomString())
}

func TestAccBasicApiKey(t *testing.T) {
	apiKeyName := createRandomApiKeyName()
	config := func(displayName string, ownerType string, ownerId string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_apikey" "terraform" {
  display_name = "%s"
	  owner_type = "%s"
	  owner_id = "%s"
	expiry_time = "2024-10-01T00:00:00Z"
	description = "Test API Key"
}`, displayName, ownerType, ownerId)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			//{
			//	Config: config(apiKeyName, "user", "user-id"),
			//},
			{
				Config: config(apiKeyName, "service-account", "12345678"),
			},
		},
	})
}
