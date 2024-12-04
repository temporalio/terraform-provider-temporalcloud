terraform {
  required_providers {
    tls = {
      source  = "hashicorp/tls"
      version = ">= 2.0.0"
    }
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

// the following example demonstrates how to manage a metrics endpoint resource with a CA cert generated outside of Terraform

resource "temporalcloud_metrics_endpoint" "terraform" {
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
}

// the following example demonstrates how to use the HashiCorp tls provider to generate certs for use in a metric endpoint
// for more information see the provider's documentation here https://registry.terraform.io/providers/hashicorp/tls/latest/docs
provider "tls" {
}

// root CA example - the metrics endpoint cert
// This cert is not stored anywhere locally. 
// If new certificates are needed you need to regenerate all of them (including the client end-entity certs).
resource "tls_self_signed_cert" "ca" {
  private_key_pem = tls_private_key.ca.private_key_pem
  allowed_uses = [
    "cert_signing",
    "server_auth",
    "client_auth",
  ]
  validity_period_hours = 8760 // 1 year
  is_ca_certificate     = true
}

resource "tls_private_key" "default" {
  algorithm = "RSA"
}

resource "tls_cert_request" "default" {
  private_key_pem = tls_private_key.default.private_key_pem
  dns_names       = []
}

// This is the end-entity cert that is used to authorize the workers connecting to temporal cloud.
// Store this cert in KMS as a best practice
// Reference your KMS's provider documentation for details on how to store a cert in KMS
resource "tls_locally_signed_cert" "default" {
  cert_request_pem      = tls_cert_request.default.cert_request_pem
  ca_private_key_pem    = tls_private_key.ca.private_key_pem
  ca_cert_pem           = tls_self_signed_cert.ca.cert_pem
  validity_period_hours = var.certificate_expiration_hours
  allowed_uses = [
    "client_auth",
    "digital_signature"
  ]
  is_ca_certificate = false
}

// example endpoint that uses the CA cert generated in this example
resource "temporalcloud_metrics_endpoint" "terraform2" {
  accepted_client_ca = base64encode(tls_self_signed_cert.ca.cert_pem)
}