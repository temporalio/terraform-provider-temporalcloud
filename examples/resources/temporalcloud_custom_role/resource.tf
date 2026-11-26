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
        resource_type = "accounts"
        resource_ids  = []
        allow_all     = true
      }
    }
  ]
}

resource "temporalcloud_project" "example_project" {
  display_name = "terraform-example-project"
}

// A project-scoped custom role: it can only grant access to resources in its
// project. Assigning it to a principal gives that principal access to the
// project, even if they have no project role. project_id cannot be changed
// after creation.
resource "temporalcloud_custom_role" "example_project_custom_role" {
  name        = "terraform-example-project-custom-role"
  description = "Read access to every namespace in the project."
  project_id  = temporalcloud_project.example_project.id

  permissions = [
    {
      actions = ["cloud.namespace.get"]
      resources = {
        resource_type = "namespaces"
        resource_ids  = []
        allow_all     = true
      }
    }
  ]
}
