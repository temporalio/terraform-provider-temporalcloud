package validation

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

func TestNamespacePermissions(t *testing.T) {
	tests := []struct {
		name              string
		accountRole       string
		namespaceAccesses []string
		expectedErrors    bool
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a schema with the validator
			s := schema.Schema{
				Attributes: map[string]schema.Attribute{
					"account": schema.StringAttribute{
						CustomType: internaltypes.CaseInsensitiveStringType{},
						Required:   true,
					},
					"namespace": schema.SetNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"namespace_id": schema.StringAttribute{
									Required: true,
								},
								"permission": schema.StringAttribute{
									CustomType: internaltypes.CaseInsensitiveStringType{},
									Required:   true,
								},
							},
						},
					},
				},
			}

			// Create the config values
			configVal := map[string]attr.Value{
				"namespace_id": types.StringValue("test-namespace"),
				"permission":   internaltypes.CaseInsensitiveString("read"),
			}

			configObj, diags := types.ObjectValue(map[string]attr.Type{
				"namespace": types.SetType{
					ElemType: types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"namespace_id": internaltypes.CaseInsensitiveStringType{},
							"permission":   internaltypes.CaseInsensitiveStringType{},
						},
					},
				},
				"account": internaltypes.CaseInsensitiveStringType{},
			}, configVal)
			if diags.HasError() {
				t.Fatalf("Failed to create config object: %v", diags)
			}
			// Convert to Terraform values
			tfValue, err := configObj.ToTerraformValue(context.Background())
			if err != nil {
				t.Fatalf("Failed to convert to Terraform value: %v", err)
			}
			// Create the configVal
			config := tfsdk.Config{
				Raw:    tfValue,
				Schema: s,
			}

			// Create a validator request
			req := validator.SetRequest{
				Path:        path.Root("namespace"),
				ConfigValue: configObj,
				Config:      config,
			}
			resp := &validator.SetResponse{}
			// Run the validator
			SetMustBeEmptyWhen(tt.otherPath, tt.values).ValidateSet(context.Background(), req, resp)

			if tt.expectedErrors && !resp.Diagnostics.HasError() {
				t.Error("expected validation error but got none")
			}
			if !tt.expectedErrors && resp.Diagnostics.HasError() {
				t.Errorf("unexpected validation error: %v", resp.Diagnostics)
			}
		})
	}
}
