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
  regions            = ["aws-ca-central-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxzCCAUygAwIBAgIQTDDt0NmKHovp4Wnapcup5zAKBggqhkjOPQQDAzASMRAw
DgYDVQQKEwd0ZXN0aW5nMB4XDTI2MDgyNTE5MjQzN1oXDTI3MDgyNTE5MjUzN1ow
EjEQMA4GA1UEChMHdGVzdGluZzB2MBAGByqGSM49AgEGBSuBBAAiA2IABEHNFYQ7
Sg2tw+zMv/iFjitL8NC/SV/y1h98hiKD8pkuEoWcrd7ZZAzM4xxJv0OIwOVrZT/U
9jA/y82W/SmWskjauStJPIkhSaO4CZdF2EbNJN4I2ae5iSuMXtQfRx1T8qNnMGUw
DgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFK/qJ3Kg
eXKG3wewJF9sf0AEy3TJMCMGA1UdEQQcMBqCGGNsaWVudC5yb290LnRlc3Rpbmcu
SnZkazAKBggqhkjOPQQDAwNpADBmAjEAmXCXw/w998v0nwx7JgG4awX9bm8joncQ
NCZ7EYQzibLk0gHCOy/i4hIt8EtqN4anAjEA55XoJnLEo+p65Gg2jc/xtY8t1O00
fKxp60bf8UE1MWAyXI/pXYrsFHLAbN64XZuY
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
  regions            = ["aws-ca-central-1"]
  api_key_auth       = true

  retention_days = 7
}

resource "temporalcloud_namespace" "test2" {
  name               = "{{ .NamespaceName }}2"
  regions            = ["aws-ca-central-1"]
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

func TestAccBasicUserWithProjectAccesses(t *testing.T) {
	type configArgs struct {
		Email           string
		ProjectName     string
		ProjectAccesses string
	}

	emailAddr := createRandomEmail()
	projectName := createRandomName()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {
}

resource "temporalcloud_project" "test" {
  display_name = "{{ .ProjectName }}"
}

resource "temporalcloud_user" "terraform" {
  email          = "{{ .Email }}"
  account_access = "read"
  {{ .ProjectAccesses }}
}`))

	config := func(args configArgs) string {
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		if err := tmpl.Execute(writer, args); err != nil {
			t.Errorf("failed to execute template: %v", err)
			t.FailNow()
		}
		writer.Flush()
		return buf.String()
	}

	withRole := func(role string) string {
		return fmt.Sprintf(`project_accesses = [
    {
      project_id = temporalcloud_project.test.id
      role       = "%s"
    }
  ]`, role)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{Email: emailAddr, ProjectName: projectName, ProjectAccesses: withRole("read")}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_user.terraform", "project_accesses.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("temporalcloud_user.terraform", "project_accesses.*", map[string]string{
						"role": "read",
					}),
				),
			},
			{
				// The role is mutable in place.
				Config: config(configArgs{Email: emailAddr, ProjectName: projectName, ProjectAccesses: withRole("admin")}),
				Check: resource.TestCheckTypeSetElemNestedAttrs("temporalcloud_user.terraform", "project_accesses.*", map[string]string{
					"role": "admin",
				}),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_user.terraform",
			},
			{
				// Configuration is authoritative: dropping the attribute revokes the grant.
				Config: config(configArgs{Email: emailAddr, ProjectName: projectName, ProjectAccesses: ""}),
				Check: resource.TestCheckNoResourceAttr(
					"temporalcloud_user.terraform", "project_accesses.#"),
			},
		},
	})
}
