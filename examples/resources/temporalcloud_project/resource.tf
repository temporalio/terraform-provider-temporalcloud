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

  project_lifecycle = {
    // Prevents the Project from being deleted accidentally. Destroying a protected
    // Project takes two steps: set this back to false, apply, then destroy.
    enable_delete_protection = false
  }
}
