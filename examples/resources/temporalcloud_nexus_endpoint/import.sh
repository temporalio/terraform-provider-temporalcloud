# Nexus Endpoints can be imported to incorporate existing Nexus Endpoints into your Terraform pipeline. 
# To import a Nexus Endpoint, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported Nexus Endpoint. In the example below, the placeholder is "temporalcloud_nexus_endpoint" "nexus_endpoint"
# - the Nexus Endpoint's ID, which is found using the Temporal Cloud CLI tcld nexus endpoint list. In the example below, this is 405f7da4224a43d99c211904ed9b3819

terraform import temporalcloud_nexus_endpoint.nexus_endpoint 405f7da4224a43d99c211904ed9b3819