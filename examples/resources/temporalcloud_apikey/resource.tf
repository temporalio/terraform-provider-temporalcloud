terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_service_account" "global_service_account" {
  name           = "admin"
  account_access = "Admin"
}

resource "temporalcloud_apikey" "global_apikey" {
  display_name = "admin"
  owner_type   = "service-account"
  owner_id     = temporalcloud_service_account.global_service_account.id
  expiry_time  = "2024-11-01T00:00:00Z"
  disabled     = false
}