terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_custom_role" "example_custom_role" {
  name        = "terraform-example-custom-role"
  description = "Example custom role created with Terraform."

  permissions = [
    {
      actions = ["cloud.account.get"]
      resources = {
        resource_type = "account"
        resource_ids  = []
        allow_all     = true
      }
    }
  ]
}
