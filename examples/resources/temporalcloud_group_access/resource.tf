terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_scim_group" "my_scim_group" {
  idp_id = "usually-group-name"
}

resource "temporalcloud_namespace" "test" {
  name         = "my-namespace"
  regions      = ["aws-us-east-1"]
  api_key_auth = true

  retention_days = 7
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
}

output "group_access" {
  value = temporalcloud_group_access.my_group_access
}
