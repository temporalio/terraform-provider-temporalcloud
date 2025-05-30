terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_group" "admin_group" {
  name = "admins"
}

resource "temporalcloud_user" "reader" {
  email          = "reader@yourdomain.com"
  account_access = "reader"
}

resource "temporalcloud_group_access" "admin_group_access" {
  group_id       = temporalcloud_group.admin_group.id
  account_access = "admin"
}

resource "temporalcloud_group_members" "admin_group_members" {
  group_id = temporalcloud_group.admin_group.id
  users = [
    temporalcloud_user.reader.id,
  ]
}