# This example demonstrates using an ephemeral API key that is never stored in state.
# The API key is created when needed and automatically deleted when Terraform completes.

terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
    aws = {
      source = "hashicorp/aws"
    }
  }
}

# Create a service account to own the API key
resource "temporalcloud_service_account" "automation" {
  name           = "automation-sa"
  account_access = "developer"
}

# Create an ephemeral API key - this will NOT be stored in Terraform state
ephemeral "temporalcloud_apikey" "temp_key" {
  owner_type   = "service-account"
  owner_id     = temporalcloud_service_account.automation.id
  display_name = "Temporary automation key"
  description  = "Ephemeral key for secret rotation"
  expiry_time  = timeadd(timestamp(), "24h")
}

# Store the API key in AWS Secrets Manager using write-only attributes
# The secret value is never stored in Terraform state
resource "aws_secretsmanager_secret" "temporal_api_key" {
  name = "temporal/api-key"
}

resource "aws_secretsmanager_secret_version" "temporal_api_key" {
  secret_id = aws_secretsmanager_secret.temporal_api_key.id
  # Use write-only attribute (Terraform 1.11+) to avoid storing in state
  secret_string_wo         = ephemeral.temporalcloud_apikey.temp_key.token
  secret_string_wo_version = 1
}
