package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"regexp"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	identityv1 "go.temporal.io/api/cloud/identity/v1"
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
	return fmt.Sprintf("%s-terraformprovider-name", randomString())
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
				Config: config(name, "Read"),
			},
			{
				Config: config(name, "Developer"),
			},
			{
				Config: config(name, "Admin"),
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
MIIBxjCCAU2gAwIBAgIRAlyZ5KUmunPLeFAupDwGL8AwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNDA4MTMyMzQ2NThaFw0yNTA4MTMyMzQ3NTha
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARG+EuL
uKRsNWs7Rbz6ciaJQB7QINTRLmTgGGE8H/wAs+KjvctjPdDdqFPZrxShRY3PUdk2
pgQKRugMTe3N52pxBx4Iablz8felfdv4kyLQbdsJzY9XmCYX3D68/9Hxsl2jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSYC5/u
K78bK1M8Fv1M6ELMjF2ZMDAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjBycDUwCgYIKoZIzj0EAwMDZwAwZAIwSycjxxmYTgV5eSJbaGMINr5LQgyKQUHQ
ryBKSGLKASa/e2ntyhsqRhj77gJ8DmkZAjAIlpDacF+Sq1kpZ5tMV7ZLElcujzj4
US8pEmNuIiCguEGwi+pb5CWfabETEHApxmo=
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
					NamespaceName: randomString(),
					NamespacePerm: "Write",
					AccountPerm:   "Read",
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

func TestAccBasicServiceAccountOrderingNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Name               string
		NamespaceName      string
		NamespaceName2     string
		AccountPerm        string
		NamespaceAccessStr string
	}

	name := createRandomName()
	namespaceName := fmt.Sprintf("%s-%s", "tf-sa-namespace", randomString())
	namespaceName2 := fmt.Sprintf("%s-%s", "tf-sa-namespace", randomString())

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
