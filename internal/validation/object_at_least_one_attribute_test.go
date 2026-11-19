package validation

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestObjectAtLeastOneAttribute(t *testing.T) {
	t.Parallel()

	attrTypes := map[string]attr.Type{
		"project_id":   types.StringType,
		"role":         types.StringType,
		"custom_roles": types.SetType{ElemType: types.StringType},
	}
	object := func(role types.String, customRoles types.Set) types.Object {
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"project_id":   types.StringValue("project-a"),
			"role":         role,
			"custom_roles": customRoles,
		})
	}
	customRoles := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("custom-role-a")})
	noCustomRoles := types.SetNull(types.StringType)

	cases := []struct {
		name    string
		value   types.Object
		wantErr bool
	}{
		{name: "role only", value: object(types.StringValue("read"), noCustomRoles)},
		{name: "custom roles only", value: object(types.StringNull(), customRoles)},
		{name: "both", value: object(types.StringValue("read"), customRoles)},
		{name: "neither", value: object(types.StringNull(), noCustomRoles), wantErr: true},
		{name: "unknown role", value: object(types.StringUnknown(), noCustomRoles)},
		{name: "unknown custom roles", value: object(types.StringNull(), types.SetUnknown(types.StringType))},
		{name: "null object", value: types.ObjectNull(attrTypes)},
		{name: "unknown object", value: types.ObjectUnknown(attrTypes)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := validator.ObjectRequest{Path: path.Root("project_accesses"), ConfigValue: tc.value}
			resp := &validator.ObjectResponse{}
			ObjectAtLeastOneAttribute("role", "custom_roles").ValidateObject(context.Background(), req, resp)

			if resp.Diagnostics.HasError() != tc.wantErr {
				t.Fatalf("HasError = %v, want %v; diagnostics: %v", resp.Diagnostics.HasError(), tc.wantErr, resp.Diagnostics)
			}
		})
	}
}
