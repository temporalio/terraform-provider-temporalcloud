terraform {
  required_providers {
    temporalcloud = {
      source = "temporalio/temporalcloud"
    }
  }
}

resource "temporalcloud_namespace" "target_namespace" {
  name           = "terraform-target-namespace"
  regions        = ["aws-us-west-2"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_namespace" "caller_namespace" {
  name           = "terraform-caller-namespace"
  regions        = ["aws-us-east-1"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_namespace" "caller_namespace_2" {
  name           = "terraform-caller-namespace-2"
  regions        = ["gcp-us-central1"]
  api_key_auth   = true
  retention_days = 14
  timeouts {
    create = "10m"
    delete = "10m"
  }
}

resource "temporalcloud_nexus_endpoint" "nexus_endpoint" {
  name        = "terraform-nexus-endpoint"
  description = <<-EOT
    Service Name:
      my-hello-service
    Operation Names:
      echo
      say-hello

    Input / Output arguments are in the following repository:
    https://github.com/temporalio/samples-go/blob/main/nexus/service/api.go
  EOT
  worker_target = {
    namespace_id = temporalcloud_namespace.target_namespace.id
    task_queue   = "terraform-task-queue"
  }
  allowed_caller_namespaces = [
    temporalcloud_namespace.caller_namespace.id,
    temporalcloud_namespace.caller_namespace_2.id,
  ]
}
