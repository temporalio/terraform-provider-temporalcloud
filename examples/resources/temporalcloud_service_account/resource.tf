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
  account_access = "Admin"
}

resource "temporalcloud_service_account" "namespace_admin" {
  name           = "developer"
  account_access = "Developer"
  namespace_accesses = [
    {
      namespace_id = temporalcloud_namespace.namespace.id
      permission   = "admin"
    }
  ]
}