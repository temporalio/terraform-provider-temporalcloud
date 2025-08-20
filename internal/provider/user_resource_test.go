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

const (
	emailDomain   = "temporal.io"
	emailBaseAddr = "saas-cicd-prod"
)

func TestUserSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewUserResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func createRandomEmail() string {
	return fmt.Sprintf("%s+terraformprovider-%s@%s", emailBaseAddr, randomString(10), emailDomain)
}

func TestAccBasicUser(t *testing.T) {
	emailAddr := createRandomEmail()
	config := func(email string, role string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_user" "terraform" {
  email = "%s"
  account_access = "%s"
}`, email, role)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(emailAddr, "read"),
			},
			{
				Config: config(emailAddr, "developer"),
			},
			{
				Config: config(emailAddr, "admin"),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_user.terraform",
			},
		},
	})
}

func TestAccBasicUserWithNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Email         string
		NamespaceName string
		AccountPerm   string
		NamespacePerm string
	}

	emailAddr := createRandomEmail()

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

resource "temporalcloud_user" "terraform" {
  email = "{{ .Email }}"
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
					Email:         emailAddr,
					NamespaceName: randomString(10),
					NamespacePerm: "write",
					AccountPerm:   "read",
				}),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_user.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					user, err := conn.GetUser(context.Background(), &cloudservicev1.GetUserRequest{
						UserId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get user: %v", err)
					}
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: nsID,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}
					spec := user.User.GetSpec()
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
				ResourceName:      "temporalcloud_user.terraform",
			},
		},
	})
}

func TestAccBasicUserWithEmptyNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Email string
	}

	emailAddr := createRandomEmail()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_user" "terraform" {
  email = "{{ .Email }}"
  account_access = "read"
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
					Email: emailAddr,
				}),
				ExpectError: regexp.MustCompile("namespace_accesses set must contain at least 1 elements, got: 0"),
			},
		},
	})
}

func TestAccBasicUserWithDuplicateNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Email string
	}

	emailAddr := createRandomEmail()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_user" "terraform" {
  email = "{{ .Email }}"
  account_access = "read"
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
					Email: emailAddr,
				}),
				ExpectError: regexp.MustCompile("namespace_id must be unique across all set entries"),
			},
		},
	})
}

func TestAccBasicUserWithMultipleNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Email         string
		NamespaceName string
	}

	emailAddr := createRandomEmail()
	nsName := randomString(10)

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "{{ .NamespaceName }}"
  regions            = ["aws-us-east-1"]
  api_key_auth       = true

  retention_days = 7
}

resource "temporalcloud_namespace" "test2" {
  name               = "{{ .NamespaceName }}2"
  regions            = ["aws-us-east-1"]
  api_key_auth       = true

  retention_days = 7
}

resource "temporalcloud_user" "terraform" {
  email = "{{ .Email }}"
  account_access = "read"
  namespace_accesses = [
    {
      namespace_id = temporalcloud_namespace.test.id
      permission = "Read"
    },
    {
      namespace_id = temporalcloud_namespace.test2.id
      permission = "Write"
    },
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
					Email:         emailAddr,
					NamespaceName: nsName,
				}),
			},
		},
	})
}
