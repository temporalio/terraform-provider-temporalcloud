terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

provider "temporalcloud" {

}

# The target namespace, which the caller rate limits protect.
resource "temporalcloud_namespace" "mynamespace" {
  name           = "terraform-target-namespace"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
}

resource "temporalcloud_namespace" "caller_50" {
  name           = "terraform-caller-50"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
}

resource "temporalcloud_namespace" "caller_0" {
  name           = "terraform-caller-0"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
}

# Default rule: unmatched calling namespaces get 100 requests per second.
resource "temporalcloud_caller_rate_limit" "default" {
  namespace_id = temporalcloud_namespace.mynamespace.id

  namespace_default = {
    rps = 100
  }
}

# Exact rule: this calling namespace gets 50 requests per second.
resource "temporalcloud_caller_rate_limit" "caller_50" {
  namespace_id = temporalcloud_namespace.mynamespace.id

  namespace_limit = {
    namespace_id = temporalcloud_namespace.caller_50.id
    rps          = 50
  }
}

# Exact rule: this calling namespace gets blocked.
resource "temporalcloud_caller_rate_limit" "caller_0" {
  namespace_id = temporalcloud_namespace.mynamespace.id

  namespace_limit = {
    namespace_id = temporalcloud_namespace.caller_0.id
    rps          = 0
  }
}
