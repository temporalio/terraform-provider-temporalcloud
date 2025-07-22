terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

// Create Private Connectivity Rule for AWS
resource "temporalcloud_connectivity_rule" "test_private" {
  connectivity_type = "private"
  connection_id     = "vpce-12345678"
  region            = "aws-us-west-2"
}

data "temporalcloud_connectivity_rule" "test_private" {
  id = temporalcloud_connectivity_rule.test_private.id
}

output "connectivity_rule" {
  value = data.temporalcloud_connectivity_rule.test_private
}