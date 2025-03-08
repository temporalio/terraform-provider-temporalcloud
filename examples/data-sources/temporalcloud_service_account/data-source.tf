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
  account_access = "admin"
}

data "temporalcloud_service_account" "admin" {
  id = temporalcloud_service_account.global_service_account
}

output "service_account" {
  value = data.temporalcloud_service_account.admin
}
