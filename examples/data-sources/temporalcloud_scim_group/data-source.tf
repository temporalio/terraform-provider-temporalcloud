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

output "scim_group" {
  value = data.temporalcloud_scim_group.my_scim_group
}
