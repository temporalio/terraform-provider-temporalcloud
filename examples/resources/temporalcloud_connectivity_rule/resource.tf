terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

// Create Public Connectivity Rule
resource "temporalcloud_connectivity_rule" "public_rule" {
  connectivity_type = "public"
}

// Create Private Connectivity Rule for AWS
resource "temporalcloud_connectivity_rule" "private_aws" {
  connectivity_type = "private"
  connection_id     = "vpce-12345678"
  region            = "aws-us-west-2"
}

// Create Private Connectivity Rule for GCP
resource "temporalcloud_connectivity_rule" "private_gcp" {
  connectivity_type = "private"
  connection_id     = "vpce-12345678"
  region            = "gcp-us-central1"
  gcp_project_id    = "my-gcp-project-id"
}

// Attaching connectivity rules to a namespace
resource "temporalcloud_namespace" "ns-with-cr" {
  name           = "ns-with-cr"
  regions        = ["aws-us-west-2"]
  api_key_auth   = true
  retention_days = 14
  connectivity_rule_ids = [
    public_rule.id, private_aws.id
  ]
}
