# Projects can be imported to incorporate existing Projects into your Terraform pipeline.
# To import a Project, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported Project. In the example below, the placeholder is "temporalcloud_project" "projectimport"
# - the Project's ID. In the example below, this is 8d1e3f2b0c9a4d5e8f7a6b5c4d3e2f10

terraform import temporalcloud_project.projectimport 8d1e3f2b0c9a4d5e8f7a6b5c4d3e2f10
