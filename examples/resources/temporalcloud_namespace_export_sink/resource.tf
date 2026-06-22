terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_namespace_export_sink" "s3test" {
  namespace = temporalcloud_namespace.terraform.id
  sink_name    = "testsink"
  enabled = true
  s3 = {
    bucket_name    = "test-export-bucket"
    region         = "us-west-2"
    role_name      = "test-iam-role"
    aws_account_id = "123456789013"
    kms_arn        = "arn:aws:kms:us-east-1:123456789013:key/test-export-key"
  }
}


resource "temporalcloud_namespace_export_sink" "gcstest" {
  namespace = temporalcloud_namespace.terraform.id
  sink_name    = "testsink"
  enabled = true
  gcs = {
    bucket_name         = "updated-bucket"
    region              = "us-central1"
    service_account_id  = "test-updated-sa"
    gcp_project_id      = "test-updated-project"
  }
}
 

