# Search Attributes can be imported to incorporate existing Namespace Search Attributes into your Terraform pipeline. 
# To import a Search Attribute, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported Search Attribute. In the example below, the placeholder is "temporalcloud_namespace_search_attribute" "saimport"
# - the Namespace ID, which includes the Namespace Name and Account ID available at the top of the Namespace's page in the Temporal Cloud UI. In the example below, this is namespaceid.acctid
# - the name of the Search Attribute, which is available in the Search Attribute configuration of Namespace's page in the Temporal Cloud UI. In the example below, this is searchAttr


terraform import temporalcloud_namespace_search_attribute.saimport namespaceid.acctid/searchAttr