terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {
  # Also can be set by environment variable `TEMPORAL_CLOUD_API_KEY`
  api_key = "my-temporalcloud-api-key"

  # Also can be set by environment variable `TEMPORAL_CLOUD_ENDPOINT`
  endpoint = "saas-api.tmprl.cloud:443"

  # Also can be set by environment variable `TEMPORAL_CLOUD_ALLOW_INSECURE`
  allow_insecure = false

  # Also can be set by environment variable `TEMPORAL_CLOUD_ALLOWED_ACCOUNT_ID`
  allowed_account_id = "my-temporalcloud-account-id"
}
