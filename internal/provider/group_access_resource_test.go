package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"text/template"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
)

func TestGroupAccessSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewGroupAccessResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccGroupAccess_Basic(t *testing.T) {
	config := func(role string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {
}

data "temporalcloud_scim_group" "terraform" {
  idp_id = "tf-basic-scim-group"
}

resource "temporalcloud_group_access" "terraform" {
  id = data.temporalcloud_scim_group.terraform.id
  account_access = "%s"

}`, role)
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("read"),
			},
			{
				Config: config("developer"),
			},
			{
				Config: config("admin"),
			},
		},
	})
}

func TestAccGroupAccess_WithNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		NamespaceName string
		AccountPerm   string
		NamespacePerm string
	}

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {
}

resource "temporalcloud_namespace" "test" {
  name = "{{ .NamespaceName }}"
  regions = ["aws-us-east-1"]
  api_key_auth = true
  retention_days = 7
}

data "temporalcloud_scim_group" "terraform" {
  idp_id = "tf-basic-scim-group"
}

resource "temporalcloud_group_access" "terraform" {
  id = data.temporalcloud_scim_group.terraform.id
  account_access = "{{ .AccountPerm }}"
  namespace_accesses = [
    {
      namespace_id = temporalcloud_namespace.test.id
      permission = "{{ .NamespacePerm }}"
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

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					NamespaceName: randomString(10),
					AccountPerm:   "read",
					NamespacePerm: "write",
				}),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_group_access.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					group, err := conn.GetUserGroup(context.Background(), &cloudservicev1.GetUserGroupRequest{
						GroupId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get group: %v", err)
					}

					spec := group.Group.GetSpec()
					if spec.GetAccess().GetAccountAccess().GetRole() != identityv1.AccountAccess_ROLE_READ {
						return errors.New("expected account role to be read")
					}
					if len(spec.GetAccess().GetNamespaceAccesses()) != 1 {
						return errors.New("expected 1 namespace access")
					}
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					nsPerm, ok := spec.GetAccess().GetNamespaceAccesses()[nsID]
					if !ok {
						return fmt.Errorf("expected entry in NamespaceAccesses for namespace %s", nsID)
					}
					if nsPerm.GetPermission() != identityv1.NamespaceAccess_PERMISSION_WRITE {
						return errors.New("expected namespace permission to be write")
					}
					return nil
				},
			},
		},
	})
}

func TestAccGroupAccess_WithEmptyNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		AccountPerm string
	}

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {
}

data "temporalcloud_scim_group" "terraform" {
  idp_id = "tf-basic-scim-group"
}

resource "temporalcloud_group_access" "terraform" {
  id = data.temporalcloud_scim_group.terraform.id
  account_access = "{{ .AccountPerm }}"
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

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					AccountPerm: "read",
				}),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_group_access.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					group, err := conn.GetUserGroup(context.Background(), &cloudservicev1.GetUserGroupRequest{
						GroupId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get group: %v", err)
					}

					spec := group.Group.GetSpec()
					if spec.GetAccess().GetAccountAccess().GetRole() != identityv1.AccountAccess_ROLE_READ {
						return errors.New("expected account role to be read")
					}
					if len(spec.GetAccess().GetNamespaceAccesses()) != 0 {
						return errors.New("expected 0 namespace access")
					}
					return nil
				},
			},
		},
	})
}
