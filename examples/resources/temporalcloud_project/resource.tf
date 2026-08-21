terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_project" "payments" {
  display_name = "payments"
  description  = "Namespaces and resources owned by the Payments team."

  // Add enable_delete_protection = true to block deletion. Destroying a protected
  // Project then takes two steps: set it back to false, apply, then destroy.
}
