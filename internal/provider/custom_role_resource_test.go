package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

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
						ResourceType: "account",
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
							"resource_type": types.StringValue("account"),
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
