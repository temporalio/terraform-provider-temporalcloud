package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"text/template"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
)

func TestServiceAccountSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewServiceAccountResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func createRandomName() string {
	return fmt.Sprintf("%s-terraformprovider-name", randomString(10))
}

func TestAccBasicServiceAccount(t *testing.T) {
	name := createRandomName()
	config := func(name string, role string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "%s"
  account_access = "%s"
}`, name, role)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, "read"),
			},
			{
				Config: config(name, "developer"),
			},
			{
				Config: config(name, "admin"),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_service_account.terraform",
			},
		},
	})
}

func TestAccBasicServiceAccount_Description(t *testing.T) {
	name := createRandomName()
	config := func(name string, descriptionStr string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "%s"
  account_access = "read"
  %s
}`, name, descriptionStr)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, ""),
			},
			{
				Config: config(name, "description = \"\""),
			},
			{
				Config: config(name, "description = \"This is a test description\""),
			},
			{
				Config: config(name, ""),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_service_account.terraform",
			},
		},
	})
}

func TestAccBasicServiceAccountWithNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Name          string
		NamespaceName string
		AccountPerm   string
		NamespacePerm string
	}

	name := createRandomName()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "{{ .NamespaceName }}"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxzCCAU2gAwIBAgIRAnkbVL6hHp218oB9UlQtN7wwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDQxMDBaFw0yNjA4MjAxNDQyMDBa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAQ0zwd8
FIaSYahXebDHEd3EywawWF087ZKz9Tbg6Qp+JQ8YhjJI4/QWrem/6cGnBchmnstC
WrR1g7D5EKU7HGh4xaJm06vnl1xzOfL7kcehulgQKTqCi7dquu8WkPRoh22jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRasbRA
UFbkQyc1p9G45bDXcwQrSzAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LlV2MWcwCgYIKoZIzj0EAwMDaAAwZQIwFv/1HkzCYi3rGcWWW5NNnEoBvdASedAQ
21CJqdeZp58YDadO7bWX3ov62kg3ocLIAjEAqMFQCFGow9iPDzgCpv2U1kpvWlMp
fhMROGKW4FAw96+jvpcXwbHYgOHN0pf1Bde1
-----END CERTIFICATE-----
PEM
  )

  retention_days = 7
}

resource "temporalcloud_service_account" "terraform" {
	  name = "{{ .Name }}"
  account_access = "{{ .AccountPerm }}"
  namespace_accesses = [
    {
      namespace_id = temporalcloud_namespace.test.id
      permission = "{{ .NamespacePerm }}"
    }
  ]
  
  depends_on = [temporalcloud_namespace.test]
}`))

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					Name:          name,
					NamespaceName: randomString(10),
					NamespacePerm: "write",
					AccountPerm:   "read",
				}),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_service_account.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					serviceAccount, err := conn.GetServiceAccount(context.Background(), &cloudservicev1.GetServiceAccountRequest{
						ServiceAccountId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get Service Account: %v", err)
					}
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: nsID,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}
					spec := serviceAccount.ServiceAccount.GetSpec()
					if spec.GetAccess().GetAccountAccess().GetRole() != identityv1.AccountAccess_ROLE_READ {
						return errors.New("expected account role to be read")
					}
					nsPerm, ok := spec.GetAccess().GetNamespaceAccesses()[ns.Namespace.GetNamespace()]
					if !ok {
						return fmt.Errorf("expected entry in NamespaceAccesses for namespace %s", ns.Namespace.GetNamespace())
					}
					if nsPerm.GetPermission() != identityv1.NamespaceAccess_PERMISSION_WRITE {
						return errors.New("expected namespace access permission to be write")
					}
					return nil
				},
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_service_account.terraform",
			},
		},
	})
}

func TestAccBasicServiceAccountWithEmptyNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Name        string
		AccountPerm string
	}

	name := createRandomName()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "{{ .Name }}"
  account_access = "{{ .AccountPerm }}"
  namespace_accesses = []
}`))

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					Name:        name,
					AccountPerm: "Read",
				}),
				ExpectError: regexp.MustCompile("namespace_accesses set must contain at least 1 elements"),
			},
		},
	})
}

func TestAccBasicServiceAccountWithDuplicateNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Name        string
		AccountPerm string
	}

	name := createRandomName()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "{{ .Name }}"
  account_access = "{{ .AccountPerm }}"
  namespace_accesses = [
    {
       namespace_id = "NS1"
       permission = "Read"
    },
    {
       namespace_id = "NS1"
       permission = "Write"
    }
  ]
}`))

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					Name:        name,
					AccountPerm: "Read",
				}),
				ExpectError: regexp.MustCompile("namespace_id must be unique across all set entries"),
			},
		},
	})
}

func TestAccNamespaceScopedServiceAccount(t *testing.T) {
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
}`))

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

	resource.ParallelTest(t, resource.TestCase{
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
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_service_account.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					serviceAccount, err := conn.GetServiceAccount(context.Background(), &cloudservicev1.GetServiceAccountRequest{
						ServiceAccountId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get Service Account: %v", err)
					}
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: nsID,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}
					spec := serviceAccount.ServiceAccount.GetSpec()

					// Verify it's namespace-scoped
					if spec.GetNamespaceScopedAccess() == nil {
						return errors.New("expected namespace-scoped access to be set")
					}
					if spec.GetAccess() != nil {
						return errors.New("expected account access to be nil")
					}

					// Verify namespace and permission
					nsa := spec.GetNamespaceScopedAccess()
					if nsa.GetNamespace() != ns.Namespace.GetNamespace() {
						return fmt.Errorf("expected namespace %s, got %s", ns.Namespace.GetNamespace(), nsa.GetNamespace())
					}
					if nsa.GetAccess().GetPermission() != identityv1.NamespaceAccess_PERMISSION_WRITE {
						return errors.New("expected namespace access permission to be write")
					}
					return nil
				},
			},
			{
				// Update permission (mutable field)
				Config: config(configArgs{
					Name:          name,
					NamespaceName: namespaceName,
					Permission:    "read",
				}),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_service_account.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					serviceAccount, err := conn.GetServiceAccount(context.Background(), &cloudservicev1.GetServiceAccountRequest{
						ServiceAccountId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get Service Account: %v", err)
					}
					spec := serviceAccount.ServiceAccount.GetSpec()
					nsa := spec.GetNamespaceScopedAccess()
					if nsa.GetAccess().GetPermission() != identityv1.NamespaceAccess_PERMISSION_READ {
						return errors.New("expected namespace access permission to be read after update")
					}
					return nil
				},
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_service_account.terraform",
			},
		},
	})
}

func TestAccNamespaceScopedServiceAccountMutualExclusivity(t *testing.T) {
	name := createRandomName()

	config := fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "%s"
  account_access = "read"
  namespace_scoped_access = {
    namespace_id = "test-namespace"
    permission   = "write"
  }
}`, name)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestAccNamespaceScopedServiceAccountNamespaceAccessesConflict(t *testing.T) {
	name := createRandomName()

	config := fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "%s"
  namespace_accesses = [
    {
      namespace_id = "ns1"
      permission   = "read"
    }
  ]
  namespace_scoped_access = {
    namespace_id = "test-namespace"
    permission   = "write"
  }
}`, name)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
		},
	})
}

func TestAccNamespaceScopedServiceAccountConversionBlocked(t *testing.T) {
	type configArgs struct {
		Name          string
		NamespaceName string
		ConfigStr     string
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
  {{ .ConfigStr }}
  depends_on = [temporalcloud_namespace.test]
}`))

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create account-scoped service account
				Config: config(configArgs{
					Name:          name,
					NamespaceName: namespaceName,
					ConfigStr:     `account_access = "read"`,
				}),
			},
			{
				// Try to convert to namespace-scoped (should fail)
				Config: config(configArgs{
					Name:          name,
					NamespaceName: namespaceName,
					ConfigStr: `namespace_scoped_access = {
    namespace_id = temporalcloud_namespace.test.id
    permission   = "write"
  }`,
				}),
				ExpectError: regexp.MustCompile("(Cannot convert account-scoped service account to namespace-scoped|Invalid Attribute Combination)"),
			},
		},
	})
}

func TestAccServiceAccountMissingAccessConfiguration(t *testing.T) {
	name := createRandomName()

	config := fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_service_account" "terraform" {
  name = "%s"
  description = "This service account has no access configuration"
}`, name)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile("Missing access configuration"),
			},
		},
	})
}

func TestAccAccountScopedServiceAccountConversionBlocked(t *testing.T) {
	type configArgs struct {
		Name          string
		NamespaceName string
		ConfigStr     string
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
  {{ .ConfigStr }}
  depends_on = [temporalcloud_namespace.test]
}`))

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create namespace-scoped service account
				Config: config(configArgs{
					Name:          name,
					NamespaceName: namespaceName,
					ConfigStr: `namespace_scoped_access = {
    namespace_id = temporalcloud_namespace.test.id
    permission   = "write"
  }`,
				}),
			},
			{
				// Try to convert to account-scoped (should fail)
				Config: config(configArgs{
					Name:          name,
					NamespaceName: namespaceName,
					ConfigStr:     `account_access = "read"`,
				}),
				ExpectError: regexp.MustCompile("(Cannot convert namespace-scoped service account to account-scoped|Invalid Attribute Combination)"),
			},
		},
	})
}

func TestAccBasicServiceAccountOrderingNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Name               string
		NamespaceName      string
		NamespaceName2     string
		AccountPerm        string
		NamespaceAccessStr string
	}

	name := createRandomName()
	namespaceName := fmt.Sprintf("%s-%s", "tf-sa-namespace", randomString(10))
	namespaceName2 := fmt.Sprintf("%s-%s", "tf-sa-namespace", randomString(10))

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "{{ .NamespaceName }}"
  regions            = ["aws-us-east-1"]
  api_key_auth   = true 
  retention_days = 7
}

resource "temporalcloud_namespace" "test2" {
  name               = "{{ .NamespaceName2 }}"
  regions            = ["aws-us-east-1"]
  api_key_auth   = true 
  retention_days = 7
}

resource "temporalcloud_service_account" "terraform" {
  name = "{{ .Name }}"
  account_access = "{{ .AccountPerm }}"
  {{ .NamespaceAccessStr }}
}`))

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					Name:           name,
					NamespaceName:  namespaceName,
					NamespaceName2: namespaceName2,
					AccountPerm:    "Read",
					NamespaceAccessStr: `namespace_accesses = [
    {
       namespace_id = temporalcloud_namespace.test.id
       permission = "read"
    },
	{
	   namespace_id = temporalcloud_namespace.test2.id
	   permission = "write"
	}
  ]`,
				}),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_service_account.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					serviceAccount, err := conn.GetServiceAccount(context.Background(), &cloudservicev1.GetServiceAccountRequest{
						ServiceAccountId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get Service Account: %v", err)
					}
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: nsID,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}
					nsID2 := state.RootModule().Resources["temporalcloud_namespace.test2"].Primary.Attributes["id"]
					ns2, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: nsID2,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}
					spec := serviceAccount.ServiceAccount.GetSpec()
					if spec.GetAccess().GetAccountAccess().GetRole() != identityv1.AccountAccess_ROLE_READ {
						return errors.New("expected account role to be read")
					}
					nsPerm, ok := spec.GetAccess().GetNamespaceAccesses()[ns.Namespace.GetNamespace()]
					if !ok {
						return fmt.Errorf("expected entry in NamespaceAccesses for namespace %s", ns.Namespace.GetNamespace())
					}
					if nsPerm.GetPermission() != identityv1.NamespaceAccess_PERMISSION_READ {
						return errors.New("expected namespace access permission to be read")
					}
					nsPerm, ok = spec.GetAccess().GetNamespaceAccesses()[ns2.Namespace.GetNamespace()]
					if !ok {
						return fmt.Errorf("expected entry in NamespaceAccesses for namespace %s", ns.Namespace.GetNamespace())
					}
					if nsPerm.GetPermission() != identityv1.NamespaceAccess_PERMISSION_WRITE {
						return errors.New("expected namespace access permission to be write")
					}
					return nil
				},
			},
			{
				// Plan only and ensure there aren't any changes
				Config: config(configArgs{
					Name:           name,
					NamespaceName:  namespaceName,
					NamespaceName2: namespaceName2,
					AccountPerm:    "Read",
					NamespaceAccessStr: `namespace_accesses = [
					    {
					       namespace_id = temporalcloud_namespace.test.id
					       permission = "read"
					    },
					    {
					       namespace_id = temporalcloud_namespace.test2.id
					       permission = "write"
					    }
					  ]`,
				}),
				PlanOnly: true,
			},
			{
				// Refresh an ensure there still aren't any changes
				RefreshState: true,
			},
			{
				// Switch the order and make sure there's no planned changes
				Config: config(configArgs{
					Name:           name,
					NamespaceName:  namespaceName,
					NamespaceName2: namespaceName2,
					AccountPerm:    "Read",
					NamespaceAccessStr: `namespace_accesses = [
				    {
				       namespace_id = temporalcloud_namespace.test2.id
				       permission = "write"
				    },
				    {
				       namespace_id = temporalcloud_namespace.test.id
				       permission = "read"
				    }
				  ]`,
				}),
				PlanOnly: true,
			},
			{
				// Add duplicate entry and ensure there aren't any changes
				Config: config(configArgs{
					Name:           name,
					NamespaceName:  namespaceName,
					NamespaceName2: namespaceName2,
					AccountPerm:    "Read",
					NamespaceAccessStr: `namespace_accesses = [
				    {
				       namespace_id = temporalcloud_namespace.test2.id
				       permission = "write"
				    },
				    {
				       namespace_id = temporalcloud_namespace.test.id
				       permission = "read"
				    },
				    {
				       namespace_id = temporalcloud_namespace.test.id
				       permission = "read"
				    }
				  ]`,
				}),
				PlanOnly: true,
			},
		},
	})
}
