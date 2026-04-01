terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_namespace" "namespace" {
  name               = "terraform"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
  retention_days     = 14
}

resource "temporalcloud_service_account" "global_service_account" {
  name           = "admin"
  account_access = "admin"
}

resource "temporalcloud_service_account" "namespace_admin" {
  name           = "developer"
  account_access = "developer"
  namespace_accesses = [
    {
      namespace_id = temporalcloud_namespace.namespace.id
      permission   = "admin"
    }
  ]
}

// the following example demonstrates how to create a service account for scraping the OpenMetrics endpoint
// for more information see https://docs.temporal.io/cloud/metrics/openmetrics

resource "temporalcloud_service_account" "metrics" {
  name           = "metrics-scraper"
  account_access = "metricsread"
}

resource "temporalcloud_apikey" "metrics" {
  display_name = "metrics-scraper"
  owner_type   = "service-account"
  owner_id     = temporalcloud_service_account.metrics.id
  expiry_time  = "2027-11-01T00:00:00Z"
  disabled     = false
}
