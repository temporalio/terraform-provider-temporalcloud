terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

// Create a namespace first
resource "temporalcloud_namespace" "example" {
  name           = "example-namespace"
  regions        = ["aws-us-west-2"]
  api_key_auth   = true
  retention_days = 14
}

// Basic namespace tags example, with a custom timeout
resource "temporalcloud_namespace_tags" "example" {
  namespace_id = temporalcloud_namespace.example.id
  tags = {
    "environment" = "production"
    "team"        = "backend"
    "project"     = "temporal-workflows"
  }
  timeouts {
    create = "10m"
    delete = "5m"
  }
}
