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

func TestGroupSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewUserGroupResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccGroup_Basic(t *testing.T) {
	name := createRandomName()
	nameUpdate := createRandomName()
	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_group" "terraform" {
  name = "%s"
}`, name)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name),
			},
			{
				Config: config(nameUpdate),
			},
			{
				Config: config(name),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_group.terraform",
			},
		},
	})
}

func TestAccGroup_WithAccesses(t *testing.T) {
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
  api_key_auth       = true

  retention_days = 7
}

resource "temporalcloud_group" "terraform" {
  name = "{{ .Name }}"
}

resource "temporalcloud_group_access" "terraform_access" {
  group_id = temporalcloud_group.terraform.id
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
					id := state.RootModule().Resources["temporalcloud_group.terraform"].Primary.Attributes["id"]
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
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					nsPerm, ok := spec.GetAccess().GetNamespaceAccesses()[nsID]
					if !ok {
						return fmt.Errorf("expected entry in NamespaceAccesses for namespace %s", nsID)
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
				ResourceName:      "temporalcloud_group.terraform",
			},
		},
	})
}
