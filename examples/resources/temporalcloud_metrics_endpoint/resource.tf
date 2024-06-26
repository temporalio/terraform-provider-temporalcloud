terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_metrics_endpoint" "terraform" {
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
}