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
	description := "TEST API Key"
	config := func(displayName string, ownerType string, ownerId string, description *string) string {
		tmpConfig := fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_apikey" "terraform" {
  display_name = "%s"
	  owner_type = "%s"
	  owner_id = "%s"
	expiry_time = "2024-10-01T00:00:00Z"`, displayName, ownerType, ownerId)

		if description != nil {
			tmpConfig += fmt.Sprintf(`
	description = "%s"`, *description)
		}

		tmpConfig += `}`
		return tmpConfig
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(apiKeyName, "service-account", "d6d0d3ff3f8c400e82ffe58d15d79fa5", nil),
			},
			{
				Config: config(apiKeyName, "service-account", "d6d0d3ff3f8c400e82ffe58d15d79fa5", &description),
			},
		},
	})
}
