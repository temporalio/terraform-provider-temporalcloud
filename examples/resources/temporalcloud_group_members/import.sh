# Group Members can be imported to incorporate existing Group Memberships into your Terraform pipeline.
# To import Group Members, you need
# - a resource configuration in your Terraform configuration file/module to accept the imported Group Members. In the example below, the placeholder is "temporalcloud_group_members" "group"
# - the Group's ID, which is found using the Temporal Cloud CLI tcld g l. In the example below, this is 72360058153949edb2f1d47019c1e85f

terraform import temporalcloud_group_members.group 72360058153949edb2f1d47019c1e85f