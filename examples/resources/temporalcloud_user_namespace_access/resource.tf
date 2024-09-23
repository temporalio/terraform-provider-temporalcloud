terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {
}

resource "temporalcloud_namespace" "terraform" {
  name               = "terraform-users"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
  retention_days     = 14
}

resource "temporalcloud_namespace" "second_ns" {
  name               = "terraform-users-2"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
  retention_days     = 14
}

resource "temporalcloud_user" "namespace_admin" {
  email          = "ns_admin@foobar.example"
  account_access = "developer"
}

resource "temporalcloud_user" "namespace_write" {
  email          = "ns_write@foobar.example"
  account_access = "developer"
}

resource "temporalcloud_user" "namespace_read" {
  email          = "ns_read@foobar.example"
  account_access = "developer"
}

resource "temporalcloud_user_namespace_access" "admin" {
  user_id      = temporalcloud_user.namespace_admin.id
  namespace_id = temporalcloud_namespace.terraform.id
  permission   = "admin"
}

resource "temporalcloud_user_namespace_access" "write" {
  user_id      = temporalcloud_user.namespace_write.id
  namespace_id = temporalcloud_namespace.terraform.id
  permission   = "write"
}

resource "temporalcloud_user_namespace_access" "read" {
  user_id      = temporalcloud_user.namespace_read.id
  namespace_id = temporalcloud_namespace.terraform.id
  permission   = "read"
}

resource "temporalcloud_user_namespace_access" "read_second_ns" {
  user_id      = temporalcloud_user.namespace_read.id
  namespace_id = temporalcloud_namespace.second_ns.id
  permission   = "read"
}
