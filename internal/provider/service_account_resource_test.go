package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/protobuf/proto"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
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
  regions            = ["aws-ca-central-1"]
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

func TestProjectScopedAccessRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for name, want := range map[string]*identityv1.ProjectScopedAccess{
		"without namespace accesses": {
			ProjectId: "project-1",
			Access:    &identityv1.ProjectAccess{Role: identityv1.ProjectAccess_PROJECT_ROLE_WRITE},
		},
		"with namespace accesses": {
			ProjectId: "project-1",
			Access:    &identityv1.ProjectAccess{Role: identityv1.ProjectAccess_PROJECT_ROLE_MEMBER},
			NamespaceAccesses: map[string]*identityv1.NamespaceAccess{
				"ns1.account": {Permission: identityv1.NamespaceAccess_PERMISSION_READ},
				"ns2.account": {Permission: identityv1.NamespaceAccess_PERMISSION_ADMIN},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			obj, diags := getProjectScopedAccessObjectFromSpec(ctx, want)
			if diags.HasError() {
				t.Fatalf("getProjectScopedAccessObjectFromSpec: %v", diags)
			}

			got, diags := getProjectScopedAccessFromModel(ctx, &serviceAccountResourceModel{ProjectScopedAccess: obj})
			if diags.HasError() {
				t.Fatalf("getProjectScopedAccessFromModel: %v", diags)
			}

			if !proto.Equal(got, want) {
				t.Errorf("round trip mismatch:\n got: %v\nwant: %v", got, want)
			}
		})
	}
}

func TestAccProjectScopedServiceAccount(t *testing.T) {
	type configArgs struct {
		Name              string
		ProjectName       string
		NamespaceName     string
		Role              string
		NamespaceAccesses string
	}

	name := createRandomName()
	projectName := createRandomName()
	namespaceName := randomString(10)

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_project" "test" {
  display_name = "{{ .ProjectName }}"
}

resource "temporalcloud_namespace" "test" {
  name           = "{{ .NamespaceName }}"
  regions        = ["aws-ca-central-1"]
  api_key_auth   = true
  retention_days = 1
  project_id     = temporalcloud_project.test.id
}

resource "temporalcloud_service_account" "terraform" {
  name = "{{ .Name }}"
  project_scoped_access = {
    project_id = temporalcloud_project.test.id
    role       = "{{ .Role }}"
    {{ .NamespaceAccesses }}
  }
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

	withNamespacePermission := func(permission string) string {
		return fmt.Sprintf(`namespace_accesses = [
      {
        namespace_id = temporalcloud_namespace.test.id
        permission   = "%s"
      }
    ]`, permission)
	}

	getSpec := func(state *terraform.State) (*identityv1.ServiceAccountSpec, error) {
		id := state.RootModule().Resources["temporalcloud_service_account.terraform"].Primary.Attributes["id"]
		conn := newConnection(t)
		serviceAccount, err := conn.GetServiceAccount(context.Background(), &cloudservicev1.GetServiceAccountRequest{
			ServiceAccountId: id,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get Service Account: %v", err)
		}
		return serviceAccount.ServiceAccount.GetSpec(), nil
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
					ProjectName:   projectName,
					NamespaceName: namespaceName,
					Role:          "write",
				}),
				Check: func(state *terraform.State) error {
					spec, err := getSpec(state)
					if err != nil {
						return err
					}

					// Verify it's project-scoped
					if spec.GetProjectScopedAccess() == nil {
						return errors.New("expected project-scoped access to be set")
					}
					if spec.GetAccess() != nil {
						return errors.New("expected account access to be nil")
					}
					if spec.GetNamespaceScopedAccess() != nil {
						return errors.New("expected namespace-scoped access to be nil")
					}

					// Verify project and role
					psa := spec.GetProjectScopedAccess()
					projectID := state.RootModule().Resources["temporalcloud_project.test"].Primary.Attributes["id"]
					if psa.GetProjectId() != projectID {
						return fmt.Errorf("expected project %s, got %s", projectID, psa.GetProjectId())
					}
					if psa.GetAccess().GetRole() != identityv1.ProjectAccess_PROJECT_ROLE_WRITE {
						return errors.New("expected project access role to be write")
					}
					if len(psa.GetNamespaceAccesses()) != 0 {
						return errors.New("expected no namespace accesses")
					}
					return nil
				},
			},
			{
				// Update role and add a namespace access (mutable fields)
				Config: config(configArgs{
					Name:              name,
					ProjectName:       projectName,
					NamespaceName:     namespaceName,
					Role:              "member",
					NamespaceAccesses: withNamespacePermission("read"),
				}),
				Check: func(state *terraform.State) error {
					spec, err := getSpec(state)
					if err != nil {
						return err
					}
					psa := spec.GetProjectScopedAccess()
					if psa.GetAccess().GetRole() != identityv1.ProjectAccess_PROJECT_ROLE_MEMBER {
						return errors.New("expected project access role to be member after update")
					}
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					if psa.GetNamespaceAccesses()[nsID].GetPermission() != identityv1.NamespaceAccess_PERMISSION_READ {
						return fmt.Errorf("expected read permission on namespace %s, got %v", nsID, psa.GetNamespaceAccesses())
					}
					return nil
				},
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_service_account.terraform",
			},
			{
				// Removing the namespace accesses revokes them
				Config: config(configArgs{
					Name:          name,
					ProjectName:   projectName,
					NamespaceName: namespaceName,
					Role:          "member",
				}),
				Check: func(state *terraform.State) error {
					spec, err := getSpec(state)
					if err != nil {
						return err
					}
					if len(spec.GetProjectScopedAccess().GetNamespaceAccesses()) != 0 {
						return errors.New("expected namespace accesses to be removed")
					}
					return nil
				},
			},
		},
	})
}

// validateServiceAccountConfig runs the provider's config validation for a service account whose
// unset attributes are null. It needs no API access, since validation happens before Configure.
func validateServiceAccountConfig(t *testing.T, attrs map[string]any) []*tfprotov6.Diagnostic {
	t.Helper()
	ctx := context.Background()

	schemaResponse := &fwresource.SchemaResponse{}
	NewServiceAccountResource().Schema(ctx, fwresource.SchemaRequest{}, schemaResponse)
	s := schemaResponse.Schema

	config := tfsdk.Plan{
		Schema: s,
		Raw:    tftypes.NewValue(s.Type().TerraformType(ctx), nil),
	}
	for name, value := range attrs {
		if diags := config.SetAttribute(ctx, path.Root(name), value); diags.HasError() {
			t.Fatalf("setting %s: %v", name, diags)
		}
	}

	dv, err := tfprotov6.NewDynamicValue(s.Type().TerraformType(ctx), config.Raw)
	if err != nil {
		t.Fatalf("NewDynamicValue: %v", err)
	}

	server, err := testAccProtoV6ProviderFactories["temporalcloud"]()
	if err != nil {
		t.Fatalf("creating provider server: %v", err)
	}
	resp, err := server.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{
		TypeName: "temporalcloud_service_account",
		Config:   &dv,
	})
	if err != nil {
		t.Fatalf("ValidateResourceConfig: %v", err)
	}
	return resp.Diagnostics
}

func TestProjectScopedServiceAccountValidation(t *testing.T) {
	t.Parallel()

	namespaceAccessType := types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}
	namespaceAccesses := types.SetValueMust(namespaceAccessType, []attr.Value{
		types.ObjectValueMust(serviceAccountNamespaceAccessAttrs, map[string]attr.Value{
			"namespace_id": types.StringValue("ns1.account"),
			"permission":   internaltypes.CaseInsensitiveString("read"),
		}),
	})
	projectScopedAccess := func(role string, namespaceAccesses types.Set) types.Object {
		return types.ObjectValueMust(serviceAccountProjectScopedAccessAttrs, map[string]attr.Value{
			"project_id":         types.StringValue("test-project"),
			"role":               internaltypes.CaseInsensitiveString(role),
			"namespace_accesses": namespaceAccesses,
		})
	}
	noNamespaceAccesses := types.SetNull(namespaceAccessType)

	for name, tc := range map[string]struct {
		attrs     map[string]any
		wantError string
	}{
		"valid": {
			attrs: map[string]any{"project_scoped_access": projectScopedAccess("write", namespaceAccesses)},
		},
		"admin without namespace accesses": {
			attrs: map[string]any{"project_scoped_access": projectScopedAccess("ADMIN", noNamespaceAccesses)},
		},
		"admin with namespace accesses": {
			attrs:     map[string]any{"project_scoped_access": projectScopedAccess("ADMIN", namespaceAccesses)},
			wantError: "Invalid Set Value",
		},
		"conflicts with account_access": {
			attrs: map[string]any{
				"project_scoped_access": projectScopedAccess("write", noNamespaceAccesses),
				"account_access":        internaltypes.CaseInsensitiveString("read"),
			},
			wantError: "Invalid Attribute Combination",
		},
		"conflicts with account_access_custom_roles": {
			attrs: map[string]any{
				"project_scoped_access":       projectScopedAccess("write", noNamespaceAccesses),
				"account_access_custom_roles": types.SetValueMust(types.StringType, []attr.Value{types.StringValue("custom-role")}),
			},
			wantError: "Invalid Attribute Combination",
		},
		"conflicts with namespace_accesses": {
			attrs: map[string]any{
				"project_scoped_access": projectScopedAccess("write", noNamespaceAccesses),
				"namespace_accesses":    namespaceAccesses,
			},
			wantError: "Invalid Attribute Combination",
		},
		"conflicts with project_accesses": {
			attrs: map[string]any{
				"project_scoped_access": projectScopedAccess("write", noNamespaceAccesses),
				"project_accesses": types.SetValueMust(types.ObjectType{AttrTypes: projectAccessAttrs}, []attr.Value{
					types.ObjectValueMust(projectAccessAttrs, map[string]attr.Value{
						"project_id": types.StringValue("p1"),
						"role":       internaltypes.CaseInsensitiveString("read"),
					}),
				}),
			},
			wantError: "Invalid Attribute Combination",
		},
		"conflicts with namespace_scoped_access": {
			attrs: map[string]any{
				"project_scoped_access": projectScopedAccess("write", noNamespaceAccesses),
				"namespace_scoped_access": types.ObjectValueMust(serviceAccountNamespaceAccessAttrs, map[string]attr.Value{
					"namespace_id": types.StringValue("ns1.account"),
					"permission":   internaltypes.CaseInsensitiveString("read"),
				}),
			},
			wantError: "Invalid Attribute Combination",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.attrs["name"] = types.StringValue("test")
			diags := validateServiceAccountConfig(t, tc.attrs)

			var errs []string
			for _, d := range diags {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					errs = append(errs, d.Summary+": "+d.Detail)
				}
			}
			if tc.wantError == "" {
				if len(errs) > 0 {
					t.Fatalf("unexpected errors: %v", errs)
				}
				return
			}
			for _, e := range errs {
				if strings.Contains(e, tc.wantError) {
					return
				}
			}
			t.Fatalf("expected an error containing %q, got %v", tc.wantError, errs)
		})
	}
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
  regions            = ["aws-ca-central-1"]
  api_key_auth   = true 
  retention_days = 7
}

resource "temporalcloud_namespace" "test2" {
  name               = "{{ .NamespaceName2 }}"
  regions            = ["aws-ca-central-1"]
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

func TestAccServiceAccountWithProjectAccesses(t *testing.T) {
	type configArgs struct {
		Name            string
		ProjectName     string
		ProjectAccesses string
	}

	name := createRandomName()
	projectName := createRandomName()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {
}

resource "temporalcloud_project" "test" {
  display_name = "{{ .ProjectName }}"
}

resource "temporalcloud_service_account" "terraform" {
  name           = "{{ .Name }}"
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
				Config: config(configArgs{Name: name, ProjectName: projectName, ProjectAccesses: withRole("member")}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_service_account.terraform", "project_accesses.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("temporalcloud_service_account.terraform", "project_accesses.*", map[string]string{
						"role": "member",
					}),
				),
			},
			{
				Config: config(configArgs{Name: name, ProjectName: projectName, ProjectAccesses: withRole("contribute")}),
				Check: resource.TestCheckTypeSetElemNestedAttrs("temporalcloud_service_account.terraform", "project_accesses.*", map[string]string{
					"role": "contribute",
				}),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_service_account.terraform",
			},
			{
				// Configuration is authoritative: dropping the attribute revokes the grant.
				Config: config(configArgs{Name: name, ProjectName: projectName, ProjectAccesses: ""}),
				Check: resource.TestCheckNoResourceAttr(
					"temporalcloud_service_account.terraform", "project_accesses.#"),
			},
		},
	})
}
