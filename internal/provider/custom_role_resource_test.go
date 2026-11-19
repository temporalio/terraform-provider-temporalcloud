package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/proto"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	resourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"
)

func TestCustomRoleSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewCustomRoleResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)
	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestUpdateCustomRoleModelFromSpecPreservesEmptyResourceIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := &customRoleResourceModel{}

	diags := updateCustomRoleModelFromSpec(ctx, state, &identityv1.CustomRole{
		Id:    "role-id",
		State: resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		Spec: &identityv1.CustomRoleSpec{
			Name:        "role-name",
			Description: "role-description",
			Permissions: []*identityv1.CustomRoleSpec_Permission{
				{
					Actions: []string{"cloud.account.get"},
					Resources: &identityv1.CustomRoleSpec_Resources{
						ResourceType: "accounts",
						ResourceIds:  nil,
						AllowAll:     true,
					},
				},
			},
		},
	})
	if diags.HasError() {
		t.Fatalf("updateCustomRoleModelFromSpec diagnostics: %+v", diags)
	}

	var permissions []types.Object
	diags = state.Permissions.ElementsAs(ctx, &permissions, false)
	if diags.HasError() {
		t.Fatalf("Permissions diagnostics: %+v", diags)
	}
	if len(permissions) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(permissions))
	}

	var permissionModel customRolePermissionModel
	diags = permissions[0].As(ctx, &permissionModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("Permission diagnostics: %+v", diags)
	}

	var resourcesModel customRoleResourcesModel
	diags = permissionModel.Resources.As(ctx, &resourcesModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("Resources diagnostics: %+v", diags)
	}

	if resourcesModel.ResourceIDs.IsNull() {
		t.Fatal("expected resource_ids to be an empty set, got null")
	}
	if resourcesModel.ResourceIDs.IsUnknown() {
		t.Fatal("expected resource_ids to be known")
	}
	if len(resourcesModel.ResourceIDs.Elements()) != 0 {
		t.Fatalf("expected empty resource_ids set, got %d elements", len(resourcesModel.ResourceIDs.Elements()))
	}
}

func TestValidateCustomRolePermissions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := []struct {
		name               string
		resourceIDs        types.Set
		allowAll           types.Bool
		wantErrorSubstring string
	}{
		{
			name:        "non-empty resource ids with omitted allow_all",
			resourceIDs: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("acc-1")}),
			allowAll:    types.BoolNull(),
		},
		{
			name:        "non-empty resource ids with allow_all false",
			resourceIDs: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("acc-1")}),
			allowAll:    types.BoolValue(false),
		},
		{
			name:        "allow_all true with empty resource ids",
			resourceIDs: types.SetValueMust(types.StringType, []attr.Value{}),
			allowAll:    types.BoolValue(true),
		},
		{
			name:               "allow_all true with non-empty resource ids",
			resourceIDs:        types.SetValueMust(types.StringType, []attr.Value{types.StringValue("acc-1")}),
			allowAll:           types.BoolValue(true),
			wantErrorSubstring: "resource_ids must be empty when allow_all is true",
		},
		{
			name:               "empty resource ids with allow_all false",
			resourceIDs:        types.SetValueMust(types.StringType, []attr.Value{}),
			allowAll:           types.BoolValue(false),
			wantErrorSubstring: "allow_all must be true when resource_ids is empty",
		},
		{
			name:               "empty resource ids with omitted allow_all",
			resourceIDs:        types.SetValueMust(types.StringType, []attr.Value{}),
			allowAll:           types.BoolNull(),
			wantErrorSubstring: "allow_all must be true when resource_ids is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			permissions := types.ListValueMust(
				types.ObjectType{AttrTypes: customRolePermissionAttrs},
				[]attr.Value{
					types.ObjectValueMust(customRolePermissionAttrs, map[string]attr.Value{
						"actions": types.SetValueMust(types.StringType, []attr.Value{types.StringValue("cloud.account.get")}),
						"resources": types.ObjectValueMust(customRoleResourcesAttrs, map[string]attr.Value{
							"resource_type": types.StringValue("accounts"),
							"resource_ids":  tc.resourceIDs,
							"allow_all":     tc.allowAll,
						}),
					}),
				},
			)

			diags := validateCustomRolePermissions(ctx, permissions)
			if tc.wantErrorSubstring == "" {
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %+v", diags)
				}
				return
			}

			if !diags.HasError() {
				t.Fatal("expected diagnostics, got none")
			}

			found := false
			for _, diag := range diags.Errors() {
				if strings.Contains(diag.Detail(), tc.wantErrorSubstring) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected diagnostic containing %q, got %+v", tc.wantErrorSubstring, diags)
			}
		})
	}
}

func TestCustomRoleProjectIDMapping(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	permissions := types.ListValueMust(types.ObjectType{AttrTypes: customRolePermissionAttrs}, []attr.Value{
		types.ObjectValueMust(customRolePermissionAttrs, map[string]attr.Value{
			"actions": types.SetValueMust(types.StringType, []attr.Value{types.StringValue("cloud.namespace.get")}),
			"resources": types.ObjectValueMust(customRoleResourcesAttrs, map[string]attr.Value{
				"resource_type": types.StringValue("namespaces"),
				"resource_ids":  types.SetValueMust(types.StringType, []attr.Value{}),
				"allow_all":     types.BoolValue(true),
			}),
		}),
	})

	cases := []struct {
		name          string
		modelValue    types.String
		specProjectID string
	}{
		{name: "account-scoped", modelValue: types.StringNull(), specProjectID: ""},
		{name: "project-scoped", modelValue: types.StringValue("proj-a"), specProjectID: "proj-a"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, diags := getCustomRoleSpecFromModel(ctx, &customRoleResourceModel{
				Name:        types.StringValue("role"),
				ProjectID:   tc.modelValue,
				Permissions: permissions,
			})
			if diags.HasError() {
				t.Fatalf("getCustomRoleSpecFromModel diagnostics: %+v", diags)
			}
			if spec.GetProjectId() != tc.specProjectID {
				t.Errorf("spec project_id = %q, want %q", spec.GetProjectId(), tc.specProjectID)
			}

			// Start from a different value so the test shows Read overwrites it.
			state := &customRoleResourceModel{ProjectID: types.StringValue("stale")}
			diags = updateCustomRoleModelFromSpec(ctx, state, &identityv1.CustomRole{
				Id:    "role-id",
				State: resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
				Spec:  spec,
			})
			if diags.HasError() {
				t.Fatalf("updateCustomRoleModelFromSpec diagnostics: %+v", diags)
			}
			if !state.ProjectID.Equal(tc.modelValue) {
				t.Errorf("state project_id = %s, want %s", state.ProjectID, tc.modelValue)
			}
		})
	}
}

// projectCustomRoleConfig declares temporalcloud_project.test and a custom role scoped to it,
// temporalcloud_custom_role.test, with the given actions on every namespace in the project.
func projectCustomRoleConfig(projectName, roleName string, actions ...string) string {
	quoted := make([]string, 0, len(actions))
	for _, action := range actions {
		quoted = append(quoted, fmt.Sprintf("%q", action))
	}
	return fmt.Sprintf(`
resource "temporalcloud_project" "test" {
  display_name = %q
}

resource "temporalcloud_custom_role" "test" {
  name       = %q
  project_id = temporalcloud_project.test.id

  permissions = [
    {
      actions = [%s]
      resources = {
        resource_type = "namespaces"
        resource_ids  = []
        allow_all     = true
      }
    }
  ]
}
`, projectName, roleName, strings.Join(quoted, ", "))
}

// projectAccessesWithCustomRole renders a project_accesses attribute granting
// temporalcloud_custom_role.test in temporalcloud_project.test, plus role if it isn't empty.
func projectAccessesWithCustomRole(role string) string {
	roleLine := ""
	if role != "" {
		roleLine = fmt.Sprintf("role         = %q\n      ", role)
	}
	return fmt.Sprintf(`project_accesses = [
    {
      project_id   = temporalcloud_project.test.id
      %scustom_roles = [temporalcloud_custom_role.test.id]
    }
  ]`, roleLine)
}

// newTestClient is newConnection for tests that also need to await async operations.
func newTestClient(t *testing.T) *client.Client {
	endpoint := os.Getenv("TEMPORAL_CLOUD_ENDPOINT")
	if endpoint == "" {
		endpoint = "saas-api.tmprl.cloud:443"
	}
	allowInsecure := os.Getenv("TEMPORAL_CLOUD_ALLOW_INSECURE") == "true"
	c, err := client.NewConnectionWithAPIKey(endpoint, allowInsecure, os.Getenv("TEMPORAL_CLOUD_API_KEY"), "test")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	return c
}

func TestAccProjectCustomRole(t *testing.T) {
	projectName := createRandomName()
	otherProjectName := createRandomName()
	roleName := createRandomName()

	config := func(projectRef string, actions ...string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {
}

resource "temporalcloud_project" "other" {
  display_name = %q
}
`, otherProjectName) + strings.Replace(
			projectCustomRoleConfig(projectName, roleName, actions...),
			"project_id = temporalcloud_project.test.id",
			"project_id = "+projectRef,
			1,
		)
	}
	projectMoveError := regexp.MustCompile("Custom role cannot be moved between projects")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("temporalcloud_project.test.id", "cloud.namespace.get"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("temporalcloud_custom_role.test", "project_id", "temporalcloud_project.test", "id"),
					resource.TestCheckResourceAttr("temporalcloud_custom_role.test", "permissions.0.actions.#", "1"),
				),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_custom_role.test",
			},
			{
				// Permissions are mutable in place, which also exercises Update sending the role's
				// project_id back unchanged.
				Config: config("temporalcloud_project.test.id", "cloud.namespace.get", "cloud.namespace.exportsink.get"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("temporalcloud_custom_role.test", "project_id", "temporalcloud_project.test", "id"),
					resource.TestCheckResourceAttr("temporalcloud_custom_role.test", "permissions.0.actions.#", "2"),
				),
			},
			{
				Config:      config("temporalcloud_project.other.id", "cloud.namespace.get", "cloud.namespace.exportsink.get"),
				ExpectError: projectMoveError,
			},
			{
				// Removing project_id would make the role account-scoped, which is a move too.
				Config:      config("null", "cloud.namespace.get", "cloud.namespace.exportsink.get"),
				ExpectError: projectMoveError,
			},
		},
	})
}

// TestAccUserProjectCustomRoleGrantedOutsideTerraform covers project access granted outside
// Terraform through custom roles only. Read must show it, with a null role, rather than failing
// on the UNSPECIFIED role, and the next plan must show its revocation rather than dropping it
// silently.
func TestAccUserProjectCustomRoleGrantedOutsideTerraform(t *testing.T) {
	emailAddr := createRandomEmail()
	projectName := createRandomName()
	roleName := createRandomName()

	config := func(projectAccesses string) string {
		return `
provider "temporalcloud" {
}
` + projectCustomRoleConfig(projectName, roleName, "cloud.namespace.get") + fmt.Sprintf(`
resource "temporalcloud_user" "terraform" {
  email          = %q
  account_access = "read"
  %s
}
`, emailAddr, projectAccesses)
	}

	var userID, projectID, customRoleID string

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(""),
				Check: func(s *terraform.State) error {
					userID = s.RootModule().Resources["temporalcloud_user.terraform"].Primary.ID
					projectID = s.RootModule().Resources["temporalcloud_project.test"].Primary.ID
					customRoleID = s.RootModule().Resources["temporalcloud_custom_role.test"].Primary.ID
					return nil
				},
			},
			{
				PreConfig: func() {
					grantProjectCustomRoleToUser(t, userID, projectID, customRoleID)
				},
				RefreshState: true,
				// Configuration doesn't name the grant, so the plan after refresh revokes it.
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_user.terraform", "project_accesses.#", "1"),
					resource.TestCheckNoResourceAttr("temporalcloud_user.terraform", "project_accesses.0.role"),
					resource.TestCheckResourceAttr("temporalcloud_user.terraform", "project_accesses.0.custom_roles.#", "1"),
					resource.TestCheckTypeSetElemAttrPair(
						"temporalcloud_user.terraform", "project_accesses.0.custom_roles.*",
						"temporalcloud_custom_role.test", "id",
					),
				),
			},
			{
				// The revocation shows up in a plan, not silently.
				Config:             config(""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Naming it in configuration keeps it, with no diff after apply.
				Config: config(projectAccessesWithCustomRole("")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_user.terraform", "project_accesses.#", "1"),
					resource.TestCheckResourceAttr("temporalcloud_user.terraform", "project_accesses.0.custom_roles.#", "1"),
				),
			},
		},
	})
}

// grantProjectCustomRoleToUser gives a user access to a project through a custom role only, the
// way the UI or another API client would.
func grantProjectCustomRoleToUser(t *testing.T, userID, projectID, customRoleID string) {
	ctx := context.Background()
	c := newTestClient(t)

	user, err := c.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{UserId: userID})
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	spec, ok := proto.Clone(user.GetUser().GetSpec()).(*identityv1.UserSpec)
	if !ok {
		t.Fatalf("failed to clone user spec")
	}
	if spec.Access == nil {
		spec.Access = &identityv1.Access{}
	}
	if spec.Access.ProjectAccesses == nil {
		spec.Access.ProjectAccesses = map[string]*identityv1.ProjectAccess{}
	}
	spec.Access.ProjectAccesses[projectID] = &identityv1.ProjectAccess{CustomRoles: []string{customRoleID}}

	resp, err := c.CloudService().UpdateUser(ctx, &cloudservicev1.UpdateUserRequest{
		UserId:           userID,
		Spec:             spec,
		ResourceVersion:  user.GetUser().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}
	if err := client.AwaitAsyncOperation(ctx, c, resp.GetAsyncOperation()); err != nil {
		t.Fatalf("failed to await user update: %v", err)
	}
}
