# Namespace Tags can be imported to incorporate existing Namespace Tags into your Terraform pipeline.
# To import Namespace Tags, you need:
# - a resource configuration in your Terraform configuration file/module to accept the imported Namespace Tags. In the example below, the placeholder is "temporalcloud_namespace_tags" "tags_import"
# - the Namespace ID, which includes the Namespace Name and Account ID available at the top of the Namespace's page in the Temporal Cloud UI. In the example below, this is namespaceid.acctid
# The import ID format is: namespaceid.acctid/tags

terraform import temporalcloud_namespace_tags.tags_import namespaceid.acctid/tags
