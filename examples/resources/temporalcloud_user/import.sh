# Users can be imported to incorporate existing Users into your Terraform pipeline. 
# To import a User, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported User. In the example below, the placeholder is "temporalcloud_user" "user"
# - the User's ID, which is found using the Temporal Cloud CLI tcld u l. In the example below, this is 72360058153949edb2f1d47019c1e85f

terraform import temporalcloud_user.user 72360058153949edb2f1d47019c1e85f