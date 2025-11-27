terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

# Example with Kinesis
resource "temporalcloud_account_audit_log_sink" "kinesis_sink" {
  sink_name = "my-kinesis-sink"
  enabled   = true
  kinesis = {
    role_name       = "arn:aws:iam::123456789012:role/TemporalCloudKinesisRole"
    destination_uri = "arn:aws:kinesis:us-east-1:123456789012:stream/my-audit-stream"
    region          = "us-east-1"
  }
}

# Example with PubSub
resource "temporalcloud_account_audit_log_sink" "pubsub_sink" {
  sink_name = "my-pubsub-sink"
  enabled   = true
  pubsub = {
    service_account_id = "my-service-account-id"
    topic_name         = "temporal-audit-logs"
    gcp_project_id     = "my-gcp-project"
  }
}
