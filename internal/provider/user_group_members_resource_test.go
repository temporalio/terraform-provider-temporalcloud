package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

func TestGroupMembers_Schema(t *testing.T) {
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

func TestAccGroupMembers_Basic(t *testing.T) {
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
}

resource "temporalcloud_group_members" "terraform" {
  group_id = temporalcloud_group.terraform.id
  users = %s
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
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_group_members.terraform"].Primary.Attributes["group_id"]
					conn := newConnection(t)
					members, err := conn.GetUserGroupMembers(context.Background(), &cloudservicev1.GetUserGroupMembersRequest{
						GroupId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get group: %v", err)
					}

					if len(members.GetMembers()) != 1 {
						return errors.New("expected 1 member")
					}

					userId := state.RootModule().Resources["temporalcloud_user.user1"].Primary.Attributes["id"]
					if members.GetMembers()[0].GetMemberId().GetUserId() != userId {
						return errors.New("expected user1 to be a member")
					}

					return nil
				},
			},
			{
				Config: config(emailAddr, emailAddr2, name, fmt.Sprintf("[%s, %s]", user1TFID, user2TFID)),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_group_members.terraform"].Primary.Attributes["group_id"]
					conn := newConnection(t)
					members, err := conn.GetUserGroupMembers(context.Background(), &cloudservicev1.GetUserGroupMembersRequest{
						GroupId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get group: %v", err)
					}

					if len(members.GetMembers()) != 2 {
						return errors.New("expected 2 members")
					}

					return nil
				},
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_group_members.terraform",
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					return state.RootModule().Resources["temporalcloud_group_members.terraform"].Primary.Attributes["group_id"], nil
				},
			},
			{
				Config: config(emailAddr, emailAddr2, name, fmt.Sprintf("[%s]", user1TFID)),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_group_members.terraform",
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					return state.RootModule().Resources["temporalcloud_group_members.terraform"].Primary.Attributes["group_id"], nil
				},
			},
		},
	})
}
