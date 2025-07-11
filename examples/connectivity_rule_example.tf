# Example: Public Connectivity Rule
resource "temporalcloud_connectivity_rule" "public_rule" {
  connectivity_type = "public"

  timeouts {
    create = "10m"
    delete = "5m"
  }
}

# Example: Private Connectivity Rule for AWS
resource "temporalcloud_connectivity_rule" "private_aws" {
  connectivity_type = "private"

  private_rule {
    connection_id  = "vpce-12345678"
    region         = "us-west-2"
    cloud_provider = "aws"
  }

  timeouts {
    create = "10m"
    delete = "5m"
  }
}

# Example: Private Connectivity Rule for GCP
resource "temporalcloud_connectivity_rule" "private_gcp" {
  connectivity_type = "private"

  private_rule {
    connection_id  = "projects/my-project/regions/us-central1/serviceAttachments/my-attachment"
    region         = "us-central1"
    cloud_provider = "gcp"
    gcp_project_id = "my-gcp-project-id"
  }

  timeouts {
    create = "10m"
    delete = "5m"
  }
}

# Output the connectivity rule information
output "public_rule_info" {
  value = {
    id       = temporalcloud_connectivity_rule.public_rule.id
    state    = temporalcloud_connectivity_rule.public_rule.state
    endpoint = temporalcloud_connectivity_rule.public_rule.endpoint
  }
}

output "private_aws_rule_info" {
  value = {
    id       = temporalcloud_connectivity_rule.private_aws.id
    state    = temporalcloud_connectivity_rule.private_aws.state
    endpoint = temporalcloud_connectivity_rule.private_aws.endpoint
  }
}

output "private_gcp_rule_info" {
  value = {
    id       = temporalcloud_connectivity_rule.private_gcp.id
    state    = temporalcloud_connectivity_rule.private_gcp.state
    endpoint = temporalcloud_connectivity_rule.private_gcp.endpoint
  }
} 