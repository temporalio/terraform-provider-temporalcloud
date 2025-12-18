package provider

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSource_ServiceAccount(t *testing.T) {
	name := createRandomName()
	config := func(name string, role string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "%s"
  account_access = "%s"
}

data "temporalcloud_service_account" "terraform" {
  id = temporalcloud_service_account.terraform.id
}

output "service_account" {
  value = data.temporalcloud_service_account.terraform
}
`, name, role)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, "read"),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["service_account"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputName, ok := outputValue["name"].(string)
					if !ok {
						return fmt.Errorf("expected value to be a string")
					}
					if outputName != name {
						return fmt.Errorf("expected service account name to be %s, got %s", name, outputName)
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected value to be a string")
					}
					if outputState != "active" {
						return fmt.Errorf("expected service account state to be active, got %s", outputState)
					}

					return nil
				},
			},
		},
	})
}

func TestAccDataSource_NamespaceScopedServiceAccount(t *testing.T) {
	type configArgs struct {
		Name          string
		NamespaceName string
		Permission    string
	}

	name := createRandomName()
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

resource "temporalcloud_service_account" "terraform" {
  name = "{{ .Name }}"
  namespace_scoped_access = {
    namespace_id = temporalcloud_namespace.test.id
    permission   = "{{ .Permission }}"
  }

  depends_on = [temporalcloud_namespace.test]
}

data "temporalcloud_service_account" "terraform" {
  id = temporalcloud_service_account.terraform.id
}

output "service_account" {
  value = data.temporalcloud_service_account.terraform
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
					Name:          name,
					NamespaceName: namespaceName,
					Permission:    "write",
				}),
				Check: func(s *terraform.State) error {
					output, ok := s.RootModule().Outputs["service_account"]
					if !ok {
						return fmt.Errorf("missing expected output")
					}

					outputValue, ok := output.Value.(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected value to be map")
					}

					outputName, ok := outputValue["name"].(string)
					if !ok {
						return fmt.Errorf("expected name to be a string")
					}
					if outputName != name {
						return fmt.Errorf("expected service account name to be %s, got %s", name, outputName)
					}

					outputState, ok := outputValue["state"].(string)
					if !ok {
						return fmt.Errorf("expected state to be a string")
					}
					if outputState != "active" {
						return fmt.Errorf("expected service account state to be active, got %s", outputState)
					}

					// Verify namespace_scoped_access is present
					namespaceScopedAccess, ok := outputValue["namespace_scoped_access"].(map[string]interface{})
					if !ok {
						return fmt.Errorf("expected namespace_scoped_access to be present and be a map")
					}

					nsID, ok := namespaceScopedAccess["namespace_id"].(string)
					if !ok || nsID == "" {
						return fmt.Errorf("expected namespace_id to be a non-empty string")
					}

					permission, ok := namespaceScopedAccess["permission"].(string)
					if !ok || permission != "write" {
						return fmt.Errorf("expected permission to be 'write', got %v", permission)
					}

					// Verify account_access is not set for namespace-scoped SA
					accountAccess, _ := outputValue["account_access"].(string)
					if accountAccess != "" {
						return fmt.Errorf("expected account_access to be empty for namespace-scoped service account, got %s", accountAccess)
					}

					return nil
				},
			},
		},
	})
}
