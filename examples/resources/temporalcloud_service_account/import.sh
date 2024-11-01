# Service Accounts can be imported to incorporate existing Service Accounts into your Terraform pipeline. 
# To import a Service Account, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported Service Account. In the example below, the placeholder is "temporalcloud_service_account" "saimport"
# - the Service Accounts's ID, which is found using the Temporal Cloud CLI tcld sa l. In the example below, this is e3cb94fbdbb845f480044d053d00665b

terraform import temporalcloud_service_account.saimport e3cb94fbdbb845f480044d053d00665b