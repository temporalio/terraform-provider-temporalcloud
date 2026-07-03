package provider

import (
	"context"
	"fmt"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccountAuditLogSinkResource_Schema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewAccountAuditLogSinkResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccAccountAuditLogSink_Kinesis(t *testing.T) {
	t.Parallel()
	const (
		kinesisRoleName         = "cloud-cicd-audit-log-external-trust-prod"
		kinesisStreamArn        = "arn:aws:kinesis:us-west-2:471170916252:stream/cloud-cicd-audit-log-prod"
		kinesisStreamArnUpdated = "arn:aws:kinesis:us-west-2:471170916252:stream/cloud-cicd-audit-log-prod-updated"
		kinesisRegion           = "us-west-2"
	)
	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAccountAuditLogSinkKinesisConfig(sinkName, kinesisRoleName, kinesisStreamArn, kinesisRegion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.role_name", kinesisRoleName),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.destination_uri", kinesisStreamArn),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.region", kinesisRegion),
					// Verify datasource
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("data.temporalcloud_account_audit_log_sink.test", "state"),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "kinesis.role_name", kinesisRoleName),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "kinesis.destination_uri", kinesisStreamArn),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "kinesis.region", kinesisRegion),
				),
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing - only the Kinesis stream ARN changes
			{
				Config: testAccAccountAuditLogSinkKinesisConfigUpdate(sinkName, kinesisRoleName, kinesisStreamArnUpdated, kinesisRegion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.role_name", kinesisRoleName),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.destination_uri", kinesisStreamArnUpdated),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.region", kinesisRegion),
					// Verify datasource reflects updates
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "kinesis.role_name", kinesisRoleName),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "kinesis.destination_uri", kinesisStreamArnUpdated),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "kinesis.region", kinesisRegion),
				),
			},
			// Delete testing
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}

func TestAccAccountAuditLogSink_PubSub(t *testing.T) {
	const (
		pubsubServiceAccount   = "audit-log-cicd-prod"
		pubsubTopicName        = "cloud-cicd-audit-log-prod"
		pubsubTopicNameUpdated = "cloud-cicd-audit-log-prod-updated"
		pubsubGCPProjectID     = "prod-t44kcfvuqwuazy9s3vuc2syu7"
	)
	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAccountAuditLogSinkPubSubConfig(sinkName, pubsubServiceAccount, pubsubTopicName, pubsubGCPProjectID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.service_account_id", pubsubServiceAccount),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.topic_name", pubsubTopicName),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.gcp_project_id", pubsubGCPProjectID),
					// Verify datasource
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("data.temporalcloud_account_audit_log_sink.test", "state"),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "pubsub.service_account_id", pubsubServiceAccount),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "pubsub.topic_name", pubsubTopicName),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "pubsub.gcp_project_id", pubsubGCPProjectID),
				),
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing - only the topic name changes
			{
				Config: testAccAccountAuditLogSinkPubSubConfigUpdate(sinkName, pubsubServiceAccount, pubsubTopicNameUpdated, pubsubGCPProjectID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.service_account_id", pubsubServiceAccount),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.topic_name", pubsubTopicNameUpdated),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.gcp_project_id", pubsubGCPProjectID),
					// Verify datasource reflects updates
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "pubsub.service_account_id", pubsubServiceAccount),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "pubsub.topic_name", pubsubTopicNameUpdated),
					resource.TestCheckResourceAttr("data.temporalcloud_account_audit_log_sink.test", "pubsub.gcp_project_id", pubsubGCPProjectID),
				),
			},
			// Delete testing
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}

func testAccAccountAuditLogSinkKinesisConfig(sinkName, roleName, streamArn, region string) string {
	return fmt.Sprintf(`
provider "temporalcloud" {
}

resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name = %[1]q
  enabled   = true
  kinesis = {
    role_name       = %[2]q
    destination_uri = %[3]q
    region          = %[4]q
  }
}

data "temporalcloud_account_audit_log_sink" "test" {
  sink_name = temporalcloud_account_audit_log_sink.test.sink_name
}
`, sinkName, roleName, streamArn, region)
}

func testAccAccountAuditLogSinkKinesisConfigUpdate(sinkName, roleName, streamArn, region string) string {
	return fmt.Sprintf(`
resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name = %[1]q
  enabled   = true
  kinesis = {
    role_name       = %[2]q
    destination_uri = %[3]q
    region          = %[4]q
  }
}

data "temporalcloud_account_audit_log_sink" "test" {
  sink_name = temporalcloud_account_audit_log_sink.test.sink_name
}
`, sinkName, roleName, streamArn, region)
}

func testAccAccountAuditLogSinkPubSubConfig(sinkName, serviceAccountID, topicName, gcpProjectID string) string {
	return fmt.Sprintf(`
provider "temporalcloud" {
}

resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name = %[1]q
  enabled   = true
  pubsub = {
    service_account_id = %[2]q
    topic_name         = %[3]q
    gcp_project_id     = %[4]q
  }
}

data "temporalcloud_account_audit_log_sink" "test" {
  sink_name = temporalcloud_account_audit_log_sink.test.sink_name
}
`, sinkName, serviceAccountID, topicName, gcpProjectID)
}

func testAccAccountAuditLogSinkPubSubConfigUpdate(sinkName, serviceAccountID, topicName, gcpProjectID string) string {
	return fmt.Sprintf(`
resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name = %[1]q
  enabled   = true
  pubsub = {
    service_account_id = %[2]q
    topic_name         = %[3]q
    gcp_project_id     = %[4]q
  }
}

data "temporalcloud_account_audit_log_sink" "test" {
  sink_name = temporalcloud_account_audit_log_sink.test.sink_name
}
`, sinkName, serviceAccountID, topicName, gcpProjectID)
}
