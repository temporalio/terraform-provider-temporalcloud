package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	"testing"
	"time"
)

func createRandomApiKeyName() string {
	return fmt.Sprintf("key-terraformprovider-%s", randomString())
}

func getExpiryTime() string {
	return time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
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
	expiry_time = "%s"`, displayName, ownerType, ownerId, getExpiryTime())

		if description != nil {
			tmpConfig += fmt.Sprintf(`
	description = "%s"`, *description)
		}

		tmpConfig += `
}`
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
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["temporalcloud_apikey.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					apiKey, err := conn.GetApiKey(context.Background(), &cloudservicev1.GetApiKeyRequest{
						KeyId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := apiKey.ApiKey.GetSpec()
					if spec.GetDisabled() {
						return errors.New("expected disabled to be false")
					}
					if spec.GetDisplayName() != apiKeyName {
						return fmt.Errorf("expected display name to be %s, got %s", apiKeyName, spec.GetDisplayName())
					}
					if spec.GetOwnerType() != "service-account" {
						return fmt.Errorf("expected owner type to be service-account, got %s", spec.GetOwnerType())
					}
					if spec.GetOwnerId() != "d6d0d3ff3f8c400e82ffe58d15d79fa5" {
						return fmt.Errorf("expected owner id to be d6d0d3ff3f8c400e82ffe58d15d79fa5, got %s", spec.GetOwnerId())
					}
					return nil
				},
			},
			{
				Config: config(apiKeyName, "service-account", "d6d0d3ff3f8c400e82ffe58d15d79fa5", &description),
			},
			{
				Config: config(apiKeyName, "service-account", "d6d0d3ff3f8c400e82ffe58d15d79fa5", &description),
			},
		},
	})
}

func TestAccDisableApiKey(t *testing.T) {
	apiKeyName := createRandomApiKeyName()
	config := func(displayName string, ownerType string, ownerId string, disable bool) string {
		tmpConfig := fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_apikey" "test" {
	display_name = "%s"
	owner_type = "%s"
	owner_id = "%s"
	expiry_time = "%s"`, displayName, ownerType, ownerId, getExpiryTime())

		if !disable {
			tmpConfig += fmt.Sprintf(`
	disabled = false`)
		} else {
			tmpConfig += fmt.Sprintf(`
	disabled = true`)
		}

		tmpConfig += `
}`
		return tmpConfig
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// do nothing
				Config: config(apiKeyName, "service-account", "d6d0d3ff3f8c400e82ffe58d15d79fa5", false),
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["temporalcloud_apikey.test"].Primary.Attributes["id"]
					conn := newConnection(t)
					apiKey, err := conn.GetApiKey(context.Background(), &cloudservicev1.GetApiKeyRequest{
						KeyId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := apiKey.ApiKey.GetSpec()
					if spec.GetDisabled() {
						return errors.New("expected disabled to be false")
					}
					return nil
				},
			},
			{
				// disable
				Config: config(apiKeyName, "service-account", "d6d0d3ff3f8c400e82ffe58d15d79fa5", true),
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["temporalcloud_apikey.test"].Primary.Attributes["id"]
					conn := newConnection(t)
					apiKey, err := conn.GetApiKey(context.Background(), &cloudservicev1.GetApiKeyRequest{
						KeyId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := apiKey.ApiKey.GetSpec()
					if !spec.GetDisabled() {
						return errors.New("expected disabled to be true")
					}
					return nil
				},
			},
			{
				// enable back again
				Config: config(apiKeyName, "service-account", "d6d0d3ff3f8c400e82ffe58d15d79fa5", false),
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["temporalcloud_apikey.test"].Primary.Attributes["id"]
					conn := newConnection(t)
					apiKey, err := conn.GetApiKey(context.Background(), &cloudservicev1.GetApiKeyRequest{
						KeyId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := apiKey.ApiKey.GetSpec()
					if spec.GetDisabled() {
						return errors.New("expected disabled to be false")
					}
					return nil
				},
			},
		},
	})
}
