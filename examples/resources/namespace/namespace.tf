provider "temporalcloud" {

}

resource "temporalcloud_namespace" "swgillespie-dev" {
  name               = "swgillespie-terraform"
  region             = "us-east-1"
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
  retention_days     = 45
}
