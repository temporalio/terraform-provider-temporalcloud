provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "terraform"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
  retention_days     = 45
}
