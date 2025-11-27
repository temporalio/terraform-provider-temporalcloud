package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccDataSource_AccountAuditLogSink_Kinesis(t *testing.T) {
	t.Parallel()
	accountAuditLogSinkTestLocks.Lock("account")
	defer func() {
		_ = accountAuditLogSinkTestLocks.Unlock("account")
	}()

	sinkRegion := "us-east-1"
	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	config := func(name, region string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name = %[1]q
  enabled   = true
  kinesis = {
    role_name      = "test-role"
    destination_uri = "test-uri"
    region         = %[2]q
  }
}

data "temporalcloud_account_audit_log_sink" "test" {
  sink_name = temporalcloud_account_audit_log_sink.test.sink_name
}

output "account_audit_log_sink" {
  value = data.temporalcloud_account_audit_log_sink.test
}
`, name, region)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(sinkName, sinkRegion),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["account_audit_log_sink"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputSinkName, ok := outputValue["sink_name"].(string)
					if !ok {
						return fmt.Errorf("expected sink_name to be a string")
					}
					if outputSinkName != sinkName {
						return fmt.Errorf("expected sink_name to be %q, got: %q", sinkName, outputSinkName)
					}

					outputEnabled, ok := outputValue["enabled"].(bool)
					if !ok {
						return fmt.Errorf("expected enabled to be a bool")
					}
					if !outputEnabled {
						return fmt.Errorf("expected enabled to be true, got: false")
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected state to be a string")
					}
					if outputState == "" {
						return fmt.Errorf("expected state to not be empty")
					}

					// Check Kinesis configuration
					kinesisMap, ok := outputValue["kinesis"].(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected kinesis to be a map")
					}
					if kinesisMap == nil {
						return fmt.Errorf("expected kinesis to not be null")
					}

					roleName, ok := kinesisMap["role_name"].(string)
					if !ok {
						return fmt.Errorf("expected kinesis.role_name to be a string")
					}
					if roleName != "test-role" {
						return fmt.Errorf("expected kinesis.role_name to be 'test-role', got: %q", roleName)
					}

					destinationURI, ok := kinesisMap["destination_uri"].(string)
					if !ok {
						return fmt.Errorf("expected kinesis.destination_uri to be a string")
					}
					if destinationURI != "test-uri" {
						return fmt.Errorf("expected kinesis.destination_uri to be 'test-uri', got: %q", destinationURI)
					}

					region, ok := kinesisMap["region"].(string)
					if !ok {
						return fmt.Errorf("expected kinesis.region to be a string")
					}
					if region != sinkRegion {
						return fmt.Errorf("expected kinesis.region to be %q, got: %q", sinkRegion, region)
					}

					// For Kinesis sink, pubsub should be null/empty
					if pubsub, exists := outputValue["pubsub"]; exists && pubsub != nil {
						return fmt.Errorf("expected pubsub to be null for Kinesis sink")
					}

					return nil
				},
			},
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}

func TestAccDataSource_AccountAuditLogSink_PubSub(t *testing.T) {
	t.Parallel()
	accountAuditLogSinkTestLocks.Lock("account")
	defer func() {
		_ = accountAuditLogSinkTestLocks.Unlock("account")
	}()

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name = %[1]q
  enabled   = true
  pubsub = {
    service_account_id = "test-sa"
    topic_name         = "test-topic"
    gcp_project_id     = "test-project"
  }
}

data "temporalcloud_account_audit_log_sink" "test" {
  sink_name = temporalcloud_account_audit_log_sink.test.sink_name
}

output "account_audit_log_sink" {
  value = data.temporalcloud_account_audit_log_sink.test
}
`, name)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(sinkName),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["account_audit_log_sink"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputSinkName, ok := outputValue["sink_name"].(string)
					if !ok {
						return fmt.Errorf("expected sink_name to be a string")
					}
					if outputSinkName != sinkName {
						return fmt.Errorf("expected sink_name to be %q, got: %q", sinkName, outputSinkName)
					}

					outputEnabled, ok := outputValue["enabled"].(bool)
					if !ok {
						return fmt.Errorf("expected enabled to be a bool")
					}
					if !outputEnabled {
						return fmt.Errorf("expected enabled to be true, got: false")
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected state to be a string")
					}
					if outputState == "" {
						return fmt.Errorf("expected state to not be empty")
					}

					// Check PubSub configuration
					pubsubMap, ok := outputValue["pubsub"].(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected pubsub to be a map")
					}
					if pubsubMap == nil {
						return fmt.Errorf("expected pubsub to not be null")
					}

					serviceAccountID, ok := pubsubMap["service_account_id"].(string)
					if !ok {
						return fmt.Errorf("expected pubsub.service_account_id to be a string")
					}
					if serviceAccountID != "test-sa" {
						return fmt.Errorf("expected pubsub.service_account_id to be 'test-sa', got: %q", serviceAccountID)
					}

					topicName, ok := pubsubMap["topic_name"].(string)
					if !ok {
						return fmt.Errorf("expected pubsub.topic_name to be a string")
					}
					if topicName != "test-topic" {
						return fmt.Errorf("expected pubsub.topic_name to be 'test-topic', got: %q", topicName)
					}

					gcpProjectID, ok := pubsubMap["gcp_project_id"].(string)
					if !ok {
						return fmt.Errorf("expected pubsub.gcp_project_id to be a string")
					}
					if gcpProjectID != "test-project" {
						return fmt.Errorf("expected pubsub.gcp_project_id to be 'test-project', got: %q", gcpProjectID)
					}

					// For PubSub sink, kinesis should be null/empty
					if kinesis, exists := outputValue["kinesis"]; exists && kinesis != nil {
						return fmt.Errorf("expected kinesis to be null for PubSub sink")
					}

					return nil
				},
			},
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}
