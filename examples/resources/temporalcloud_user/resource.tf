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

resource "temporalcloud_user" "global_admin" {
  email = "admin@yourdomain.com"
  account_access = "admin"
}

resource "temporalcloud_user" "namespace_admin" {
  email = "developer@yourdomain.com"
  account_access = "developer"
  namespace_accesses = [
    {
      namespace = temporalcloud_namespace.namespace.id
      permission = "admin"
    }
  ]
}