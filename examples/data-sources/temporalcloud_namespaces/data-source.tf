terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

data "temporalcloud_namespaces" "my_namespaces" {}

output "namespaces" {
  value = data.temporalcloud_namespaces.my_namespaces.namespaces
}
