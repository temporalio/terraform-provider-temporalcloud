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
  name           = "terraform"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
}

data "temporalcloud_namespace" "my_namespace" {
  id = temporalcloud_namespace.namespace
}

output "namespace" {
  value = data.temporalcloud_namespaces.my_namespace
}
