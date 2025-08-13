package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccDataSource_ConnectivityRule_Public(t *testing.T) {
	config := func() string {
		return `
provider "temporalcloud" {

}

resource "temporalcloud_connectivity_rule" "test_public" {
  connectivity_type = "public"
}

data "temporalcloud_connectivity_rule" "test_public" {
  id = temporalcloud_connectivity_rule.test_public.id
}

output "connectivity_rule" {
  value = data.temporalcloud_connectivity_rule.test_public
}
`
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["connectivity_rule"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputConnectivityType, ok := outputValue["connectivity_type"].(string)
					if !ok {
						return fmt.Errorf("expected connectivity_type to be a string")
					}
					if outputConnectivityType != "public" {
						return fmt.Errorf("expected connectivity_type to be 'public', got: %s", outputConnectivityType)
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected state to be a string")
					}
					if outputState == "" {
						return fmt.Errorf("expected state to not be empty")
					}

					outputCreatedAt, ok := outputValue["created_at"].(string)
					if !ok {
						return fmt.Errorf("expected created_at to be a string")
					}
					if outputCreatedAt == "" {
						return fmt.Errorf("expected created_at to not be empty")
					}

					// For public connectivity rules, these should be null/empty
					if connectionID, exists := outputValue["connection_id"]; exists && connectionID != nil {
						return fmt.Errorf("expected connection_id to be null for public connectivity rule")
					}

					if region, exists := outputValue["region"]; exists && region != nil {
						return fmt.Errorf("expected region to be null for public connectivity rule")
					}

					if gcpProjectID, exists := outputValue["gcp_project_id"]; exists && gcpProjectID != nil {
						return fmt.Errorf("expected gcp_project_id to be null for public connectivity rule")
					}

					return nil
				},
			},
		},
	})
}

func TestAccDataSource_ConnectivityRule_Private(t *testing.T) {
	config := func() string {
		return `
provider "temporalcloud" {

}

resource "temporalcloud_connectivity_rule" "test_private" {
  connectivity_type = "private"
  connection_id     = "vpce-12345678"
  region           = "aws-us-west-2"
}

data "temporalcloud_connectivity_rule" "test_private" {
  id = temporalcloud_connectivity_rule.test_private.id
}

output "connectivity_rule" {
  value = data.temporalcloud_connectivity_rule.test_private
}
`
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["connectivity_rule"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputConnectivityType, ok := outputValue["connectivity_type"].(string)
					if !ok {
						return fmt.Errorf("expected connectivity_type to be a string")
					}
					if outputConnectivityType != "private" {
						return fmt.Errorf("expected connectivity_type to be 'private', got: %s", outputConnectivityType)
					}

					outputConnectionID, ok := outputValue["connection_id"].(string)
					if !ok {
						return fmt.Errorf("expected connection_id to be a string")
					}
					if outputConnectionID != "vpce-12345678" {
						return fmt.Errorf("expected connection_id to be 'vpce-12345678', got: %s", outputConnectionID)
					}

					outputRegion, ok := outputValue["region"].(string)
					if !ok {
						return fmt.Errorf("expected region to be a string")
					}
					if outputRegion != "aws-us-west-2" {
						return fmt.Errorf("expected region to be 'aws-us-west-2', got: %s", outputRegion)
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected state to be a string")
					}
					if outputState == "" {
						return fmt.Errorf("expected state to not be empty")
					}

					outputCreatedAt, ok := outputValue["created_at"].(string)
					if !ok {
						return fmt.Errorf("expected created_at to be a string")
					}
					if outputCreatedAt == "" {
						return fmt.Errorf("expected created_at to not be empty")
					}

					// For AWS private connectivity rules, gcp_project_id should be null
					if gcpProjectID, exists := outputValue["gcp_project_id"]; exists && gcpProjectID != nil {
						return fmt.Errorf("expected gcp_project_id to be null for AWS private connectivity rule")
					}

					return nil
				},
			},
		},
	})
}
