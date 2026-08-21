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

  // Set enable_delete_protection = true to block deletion. Destroying such a
  // Project then takes two steps: set it back to false, apply, then destroy.
  // enable_delete_protection = false
}
