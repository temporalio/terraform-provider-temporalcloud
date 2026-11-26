terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

data "temporalcloud_scim_group" "my_scim_group" {
  idp_id = "usually-group-name"
}

resource "temporalcloud_project" "payments" {
  display_name = "payments"
}

resource "temporalcloud_namespace" "test" {
  name         = "my-namespace"
  regions      = ["aws-us-east-1"]
  api_key_auth = true

  retention_days = 7
}

// A project custom role grants specific actions in its project.
resource "temporalcloud_custom_role" "payments_namespace_reader" {
  name       = "payments-namespace-reader"
  project_id = temporalcloud_project.payments.id

  permissions = [
    {
      actions = ["cloud.namespace.get"]
      resources = {
        resource_type = "namespaces"
        resource_ids  = []
        allow_all     = true
      }
    }
  ]
}

resource "temporalcloud_group_access" "my_group_access" {
  id             = temporalcloud_scim_group.my_scim_group.id
  account_access = "read"
  namespace_accesses = [
    {
      namespace_id = temporalcloud_namespace.test.id
      permission   = "write"
    }
  ]
  project_accesses = [
    {
      project_id   = temporalcloud_project.payments.id
      role         = "read"
      custom_roles = [temporalcloud_custom_role.payments_namespace_reader.id]
    }
  ]
}

