# Namespace can be imported to incorporate existing Namespaces into your Terraform pipeline. 
# To import a Namespace, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported Namespace. In the example below, the placeholder is "temporalcloud_namespace" "terraform"
# - the Namespace ID, which includes the Namespace Name and Account ID available at the top of the Namespace's page in the Temporal Cloud UI. In the example below, this is namespaceid.acctid

terraform import temporalcloud_namespace.terraform namespaceid.acctid