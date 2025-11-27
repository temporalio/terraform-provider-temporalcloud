package provider

import (
	"context"
	"fmt"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/jpillora/maplock"
)

// accountAuditLogSinkTestLocks is a per-account mutex that protects against concurrent sink operations
// across all test files, since an account can only have one audit log sink at a time.
var accountAuditLogSinkTestLocks = maplock.New()

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
	accountAuditLogSinkTestLocks.Lock("account")
	defer func() {
		_ = accountAuditLogSinkTestLocks.Unlock("account")
	}()

	sinkRegion := "us-east-1"
	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAccountAuditLogSinkKinesisConfig(sinkName, sinkRegion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.role_name", "test-role"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.destination_uri", "test-uri"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.region", sinkRegion),
				),
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing
			{
				Config: testAccAccountAuditLogSinkKinesisConfigUpdate(sinkName, sinkRegion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "false"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.role_name", "test-updated-role"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.destination_uri", "test-updated-uri"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "kinesis.region", sinkRegion),
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
	t.Parallel()
	accountAuditLogSinkTestLocks.Lock("account")
	defer func() {
		_ = accountAuditLogSinkTestLocks.Unlock("account")
	}()

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAccountAuditLogSinkPubSubConfig(sinkName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.service_account_id", "test-sa"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.topic_name", "test-topic"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.gcp_project_id", "test-project"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_account_audit_log_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing
			{
				Config: testAccAccountAuditLogSinkPubSubConfigUpdate(sinkName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "enabled", "false"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.service_account_id", "test-updated-sa"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.topic_name", "test-updated-topic"),
					resource.TestCheckResourceAttr("temporalcloud_account_audit_log_sink.test", "pubsub.gcp_project_id", "test-updated-project"),
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

func testAccAccountAuditLogSinkKinesisConfig(sinkName, sinkRegion string) string {
	return fmt.Sprintf(`
provider "temporalcloud" {
}

resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name    = %[1]q
  enabled = true
  kinesis = {
    role_name      = "test-role"
    destination_uri = "test-uri"
    region         = %[2]q
  }
}
`, sinkName, sinkRegion)
}

func testAccAccountAuditLogSinkKinesisConfigUpdate(sinkName, sinkRegion string) string {
	return fmt.Sprintf(`
resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name    = %[1]q
  enabled = false
  kinesis = {
    role_name      = "test-updated-role"
    destination_uri = "test-updated-uri"
    region         = %[2]q
  }
}
`, sinkName, sinkRegion)
}

func testAccAccountAuditLogSinkPubSubConfig(sinkName string) string {
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
`, sinkName)
}

func testAccAccountAuditLogSinkPubSubConfigUpdate(sinkName string) string {
	return fmt.Sprintf(`
resource "temporalcloud_account_audit_log_sink" "test" {
  sink_name = %[1]q
  enabled   = false
  pubsub = {
    service_account_id = "test-updated-sa"
    topic_name         = "test-updated-topic"
    gcp_project_id     = "test-updated-project"
  }
}
`, sinkName)
}
