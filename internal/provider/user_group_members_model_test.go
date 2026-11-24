package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestGroupMembers_ModelState(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		users []string
	}{
		{name: "nil"},
		{name: "empty", users: []string{}},
		{name: "populated", users: []string{"user-1", "user-2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			var schemaResponse resource.SchemaResponse
			NewUserGroupMembersResource().Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
			model := userGroupMembersResourceModel{
				Users: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("old-user")}),
				Timeouts: timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{
					"create": types.StringType,
					"delete": types.StringType,
				})},
			}
			if diags := updateGroupMembersModelFromSpec(ctx, &model, "group-1", tc.users); diags.HasError() {
				t.Fatalf("Model diagnostics: %v", diags)
			}
			state := tfsdk.State{Schema: schemaResponse.Schema}
			if diags := state.Set(ctx, model); diags.HasError() {
				t.Fatalf("State diagnostics: %v", diags)
			}
			var got userGroupMembersResourceModel
			if diags := state.Get(ctx, &got); diags.HasError() {
				t.Fatalf("State read diagnostics: %v", diags)
			}
			values := make([]attr.Value, 0, len(tc.users))
			for _, user := range tc.users {
				values = append(values, types.StringValue(user))
			}
			want := types.SetValueMust(types.StringType, values)
			if !got.Users.Equal(want) {
				t.Errorf("users = %v, want %v", got.Users, want)
			}
			if got.ID.ValueString() != "group/group-1/members" || got.GroupID.ValueString() != "group-1" {
				t.Errorf("Unexpected group identity: %v / %v", got.ID, got.GroupID)
			}
		})
	}
}
