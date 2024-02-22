provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "terraform-with-search-attributes"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
  retention_days     = 14
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "CustomSearchAttribute"
  type         = "Text"
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute2" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "CustomSearchAttribute2"
  type         = "Text"
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute3" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "CustomSearchAttribute3"
  type         = "Text"
}
