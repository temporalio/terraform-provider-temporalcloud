package provider

import (
	"context"
	"errors"
	"fmt"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	identityv1 "go.temporal.io/api/cloud/identity/v1"
)

func TestAPIKeySchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewApiKeyResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func createRandomApiKeyName() string {
	return fmt.Sprintf("key-terraformprovider-%s", randomString())
}

func getExpiryTime() string {
	return time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)
}

func TestAccBasicApiKey(t *testing.T) {
	apiKeyName := createRandomApiKeyName()
	serviceAccountName := createRandomName()
	description := "TEST API Key"
	config := func(displayName string, ownerType string, serviceAccountName string, description *string) string {
		tmpConfig := fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform1" {
	name = "%s"
	account_access = "Admin"
}

resource "temporalcloud_apikey" "terraform2" {
	display_name = "%s"
	owner_type = "%s"
	owner_id = temporalcloud_service_account.terraform1.id
	expiry_time = "%s"`, serviceAccountName, displayName, ownerType, getExpiryTime())

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
				Config: config(apiKeyName, "service-account", serviceAccountName, nil),
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["temporalcloud_apikey.terraform2"].Primary.Attributes["id"]
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
					if spec.GetOwnerType() != identityv1.OWNER_TYPE_SERVICE_ACCOUNT {
						return fmt.Errorf("expected owner type to be service-account, got %s", spec.GetOwnerType())
					}
					serviceAccountID := s.RootModule().Resources["temporalcloud_service_account.terraform1"].Primary.Attributes["id"]
					if spec.GetOwnerId() != serviceAccountID {
						return fmt.Errorf("expected owner id to be d6d0d3ff3f8c400e82ffe58d15d79fa5, got %s", spec.GetOwnerId())
					}
					return nil
				},
			},
			{
				Config: config(apiKeyName, "service-account", serviceAccountName, &description),
			},
			{
				Config: config(apiKeyName, "service-account", serviceAccountName, &description),
			},
		},
	})
}

func TestAccDisableApiKey(t *testing.T) {
	apiKeyName := createRandomApiKeyName()
	serviceAccountName := createRandomName()
	config := func(displayName string, ownerType string, serviceAccountName string, disable bool) string {
		tmpConfig := fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
	name = "%s"
	account_access = "Admin"
}

resource "temporalcloud_apikey" "test" {
	display_name = "%s"
	owner_type = "%s"
	owner_id = temporalcloud_service_account.terraform.id
	expiry_time = "%s"`, serviceAccountName, displayName, ownerType, getExpiryTime())

		if !disable {
			tmpConfig += `
	disabled = false`
		} else {
			tmpConfig += `
	disabled = true`
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
				Config: config(apiKeyName, "service-account", serviceAccountName, false),
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
				Config: config(apiKeyName, "service-account", serviceAccountName, true),
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
				Config: config(apiKeyName, "service-account", serviceAccountName, false),
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
