terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

data "temporalcloud_nexus_endpoints" "my_nexus_endpoints" {}

output "nexus_endpoints" {
  value     = data.temporalcloud_nexus_endpoints.my_nexus_endpoints.nexus_endpoints
  sensitive = true
}
