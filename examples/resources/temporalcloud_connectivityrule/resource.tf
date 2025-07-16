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

