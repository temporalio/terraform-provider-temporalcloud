terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.terraform.id
  sink_name    = "testsink"
  enabled = false
  s3 = {
    bucket_name    = "test-export-bucket"
    region         = "us-west-2"
    role_name      = "test-iam-role"
    aws_account_id = "123456789013"
    kms_arn        = "arn:aws:kms:us-east-1:123456789013:key/test-export-key"
  }
}

data "temporalcloud_namespace_export_sink" "my_sink"{
    id = temporalcloud_namespace_export_sink.test
}

output "exportsink"{
    value = data.temporalcloud_namespace_export_sink.my_sink
}
