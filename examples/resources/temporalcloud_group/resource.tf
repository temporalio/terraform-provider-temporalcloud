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

resource "temporalcloud_group" "namespace_admin_group" {
  name = "developers"
}

resource "temporalcloud_group_access" "namespace_admin_group_access" {
  group_id       = temporalcloud_group.namespace_admin_group.id
  account_access = "developer"
  namespace_accesses = [
    {
      namespace_id = temporalcloud_namespace.namespace.id
      permission   = "admin"
    }
  ]
}