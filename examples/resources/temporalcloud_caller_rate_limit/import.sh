# Caller Rate Limits can be imported to incorporate existing Caller Rate Limits into your Terraform pipeline.
# To import a Caller Rate Limit, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported Caller Rate Limit. In the example below, the placeholder is "temporalcloud_caller_rate_limit" "crlimport"
# - the ID of the target namespace, which includes the Namespace Name and Account ID available at the top of the Namespace's page in the Temporal Cloud UI. In the example below, this is namespaceid.acctid
# - the Caller Rate Limit's ID, which is found using the Temporal Cloud CLI command temporal cloud namespace caller-rate-limit list --namespace namespaceid.acctid. Add --caller-namespace-default or --caller-namespace callernamespaceid.acctid to find a specific rule. In the example below, this is 405f7da4224a43d99c211904ed9b3819

terraform import temporalcloud_caller_rate_limit.crlimport namespaceid.acctid/405f7da4224a43d99c211904ed9b3819
