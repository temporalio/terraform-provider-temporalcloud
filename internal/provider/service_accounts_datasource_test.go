package provider

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccServiceAccounts(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAccountsConfig(),
			},
		},
	})
}

func testAccServiceAccountsConfig() string {
	return `
provider "temporalcloud" {

}

data "temporalcloud_service_accounts" "example" {}
`
}

func TestAccServiceAccountsWithBothTypes(t *testing.T) {
	type configArgs struct {
		AccountScopedName   string
		NamespaceScopedName string
		NamespaceName       string
	}

	accountScopedName := createRandomName()
	namespaceScopedName := createRandomName()
	namespaceName := randomString(10)

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "{{ .NamespaceName }}"
  regions            = ["aws-us-east-1"]
  api_key_auth       = true
  retention_days     = 7
}

resource "temporalcloud_service_account" "account_scoped" {
  name           = "{{ .AccountScopedName }}"
  account_access = "read"
}

resource "temporalcloud_service_account" "namespace_scoped" {
  name = "{{ .NamespaceScopedName }}"
  namespace_scoped_access = {
    namespace_id = temporalcloud_namespace.test.id
    permission   = "write"
  }

  depends_on = [temporalcloud_namespace.test]
}

data "temporalcloud_service_accounts" "all" {
  depends_on = [
    temporalcloud_service_account.account_scoped,
    temporalcloud_service_account.namespace_scoped
  ]
}

output "service_accounts" {
  value = data.temporalcloud_service_accounts.all
}
`))

	config := func(args configArgs) string {
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		if err := tmpl.Execute(writer, args); err != nil {
			t.Errorf("failed to execute template:  %v", err)
			t.FailNow()
		}

		writer.Flush()
		return buf.String()
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					AccountScopedName:   accountScopedName,
					NamespaceScopedName: namespaceScopedName,
					NamespaceName:       namespaceName,
				}),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["service_accounts"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					serviceAccounts, ok := outputValue["service_accounts"].([]interface{})
					if !ok {
						return fmt.Errorf("expected service_accounts to be a list")
					}

					// Find our created service accounts
					var foundAccountScoped, foundNamespaceScoped bool
					for _, sa := range serviceAccounts {
						saMap, ok := sa.(map[string]interface{})
						if !ok {
							continue
						}

						name, _ := saMap["name"].(string)

						if name == accountScopedName {
							foundAccountScoped = true
							// Verify it's account-scoped
							accountAccess, ok := saMap["account_access"].(string)
							if !ok || accountAccess == "" {
								return fmt.Errorf("expected account_access to be set for account-scoped service account")
							}
							// Verify namespace_scoped_access is not set
							if namespaceScopedAccess, ok := saMap["namespace_scoped_access"].(map[string]interface{}); ok && namespaceScopedAccess != nil {
								return fmt.Errorf("expected namespace_scoped_access to be null for account-scoped service account")
							}
						}

						if name == namespaceScopedName {
							foundNamespaceScoped = true
							// Verify it's namespace-scoped
							namespaceScopedAccess, ok := saMap["namespace_scoped_access"].(map[string]interface{})
							if !ok || namespaceScopedAccess == nil {
								return fmt.Errorf("expected namespace_scoped_access to be set for namespace-scoped service account")
							}
							// Verify account_access is not set
							accountAccess, _ := saMap["account_access"].(string)
							if accountAccess != "" {
								return fmt.Errorf("expected account_access to be empty for namespace-scoped service account")
							}
						}
					}

					if !foundAccountScoped {
						return fmt.Errorf("did not find account-scoped service account '%s' in datasource results", accountScopedName)
					}
					if !foundNamespaceScoped {
						return fmt.Errorf("did not find namespace-scoped service account '%s' in datasource results", namespaceScopedName)
					}

					return nil
				},
			},
		},
	})
}
