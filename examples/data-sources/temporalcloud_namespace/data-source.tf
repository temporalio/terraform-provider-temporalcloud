terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

data "temporalcloud_namespace" "my_namespace" {
  id = temporalcloud_namespace.namespace
}

output "namespace" {
  value = data.temporalcloud_namespaces.my_namespace
}
