package enums

// CustomRoleResourceTypes are the wire-format values accepted by the Cloud Ops API
// for CustomRoleSpec.Resources.resource_type.
//
// Source of truth: https://github.com/temporalio/saas-auth/blob/main/resources/resources.go
var CustomRoleResourceTypes = []string{
	"accounts",
	"projects",
	"namespaces",
	"nexus-endpoints",
	"connectivity-rules",
	"custom-roles",
}
