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
}

// Projects that should not be torn down by accident can enable delete protection.
// To destroy such a Project, set enable_delete_protection back to false and apply
// before running terraform destroy.
resource "temporalcloud_project" "production" {
  display_name             = "production"
  description              = "Production workloads."
  enable_delete_protection = true
}
