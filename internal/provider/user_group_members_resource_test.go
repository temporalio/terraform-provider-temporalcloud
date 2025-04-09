package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestGroupMembersSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewUserGroupMembersResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccBasicGroupMembers(t *testing.T) {
	name := createRandomName()
	emailAddr := createRandomEmail()
	emailAddr2 := createRandomEmail()
	config := func(email, email2, name, users string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_user" "user1" {
  email = "%s"
  account_access = "read"
}

resource "temporalcloud_user" "user2" {
  email = "%s"
  account_access = "read"
}

resource "temporalcloud_group" "terraform" {
  name = "%s"
  account_access = "developer"
}

resource "temporalcloud_group_members" "terraform" {
  group_id = temporalcloud_group.terraform.id
  user_ids = %s
}
`, email, email2, name, users)
	}

	user1TFID := "temporalcloud_user.user1.id"
	user2TFID := "temporalcloud_user.user2.id"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config(emailAddr, emailAddr2, name, "[]"),
				ExpectError: regexp.MustCompile(""),
			},
			{
				Config: config(emailAddr, emailAddr2, name, fmt.Sprintf("[%s]", user1TFID)),
			},
			{
				Config: config(emailAddr, emailAddr2, name, fmt.Sprintf("[%s, %s]", user1TFID, user2TFID)),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_group_members.terraform",
			},
			{
				Config: config(emailAddr, emailAddr2, name, fmt.Sprintf("[%s]", user1TFID)),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_group_members.terraform",
			},
		},
	})
}
