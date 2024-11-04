# API Keys can be imported to incorporate existing API Keys into your Terraform pipeline. 
# To import an API Key, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported API Key. In the example below, the placeholder is "temporalcloud_apikey" "tfapikey"
# - the API Key's ID, which is found when clicking into an API Key in the Temporal Cloud UI. In the example below, this is zJV5zQ3IhsAbw75dAkVNEMsAd3a5AemC

terraform import temporalcloud_apikey.tfapikey zJV5zQ3IhsAbw75dAkVNEMsAd3a5AemC