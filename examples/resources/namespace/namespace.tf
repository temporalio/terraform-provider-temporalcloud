provider "temporalcloud" {

}

resource "temporalcloud_namespace" "swgillespie-dev" {
  name               = "swgillespie.a2dd6"
  region             = "us-west-2"
  accepted_client_ca = base64encode("not a real cert")
  retention_days     = 30
}
