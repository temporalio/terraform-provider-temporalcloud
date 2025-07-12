# Output the connectivity rule information
output "temporalcloud_connectivity_rule.public_rule_info" {
  value = {
    id       = temporalcloud_connectivity_rule.public_rule.id
    state    = temporalcloud_connectivity_rule.public_rule.state
  }
}

output "temporalcloud_connectivity_rule.private_aws_rule_info" {
  value = {
    id       = temporalcloud_connectivity_rule.private_aws.id
    state    = temporalcloud_connectivity_rule.private_aws.state
  }
}

output temporalcloud_connectivity_rule."private_gcp_rule_info" {
  value = {
    id       = temporalcloud_connectivity_rule.private_gcp.id
    state    = temporalcloud_connectivity_rule.private_gcp.state
  }
} 