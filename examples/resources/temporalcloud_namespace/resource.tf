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

// the following example demonstrates how to manage a namespace resource with a CA cert generated outside of Terrafrom

resource "temporalcloud_namespace" "terraform" {
  name               = "terraform"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(file("${path.module}/ca.pem"))
  retention_days     = 14
}

// the following example demonstrates how to use the hashi tls provider to generate certs for use in a namespace and end-entity
// the hasicorp tls provider is used to generate the namespace's ca cert
// for more information see the provider's documentation here https://registry.terraform.io/providers/hashicorp/tls/latest/docs
provider "tls" {
}

// root CA example - the namespace cert
// This cert is not stored anywhere locally. 
// If new certificates are needed you need to regenerate all of them (including the client end-entity certs).
resource "tls_self_signed_cert" "ca" {
  private_key_pem = tls_private_key.ca.private_key_pem
  subject {
    // arguments to to supply for the format function are the namespace name, region, and account id
    common_name = format("%s-%s.%s", "terraform2", ["aws-us-east-1"], "terraform")
    // this should represent your organization name
    organization = "terraform"
  }
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
  subject {
    // arguments to to supply for the format function are the namespace name, region, and account id
    common_name = format("%s-%s.%s", "terraform2", ["aws-us-east-1"], "terraform")
    // this should represent your organization name
    organization = "terraform"
  }
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

// example namespace that uses the CA cert generated in this example
resource "temporalcloud_namespace" "terraform2" {
  name               = "terraform2"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(tls_self_signed_cert.ca.cert_pem)
  retention_days     = 14
}

// example namespace that uses API Key for authentication
resource "temporalcloud_namespace" "terraform3" {
  name           = "terraform3"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
}