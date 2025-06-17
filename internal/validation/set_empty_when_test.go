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

func TestSetMustBeEmptyWhen(t *testing.T) {
	tests := []struct {
		name           string
		otherPath      path.Path
		values         []string
		setValue       types.Set
		otherValue     internaltypes.CaseInsensitiveStringValue
		expectedErrors bool
	}{
		{
			name:           "empty set when other value matches",
			otherPath:      path.Root("other"),
			values:         []string{"test"},
			setValue:       types.SetValueMust(types.StringType, []attr.Value{}),
			otherValue:     internaltypes.CaseInsensitiveString("test"),
			expectedErrors: false,
		},
		{
			name:      "non-empty set when other value matches",
			otherPath: path.Root("other"),
			values:    []string{"test"},
			setValue: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("value1"),
			}),
			otherValue:     internaltypes.CaseInsensitiveString("test"),
			expectedErrors: true,
		},
		{
			name:      "non-empty set when other value doesn't match",
			otherPath: path.Root("other"),
			values:    []string{"test"},
			setValue: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("value1"),
			}),
			otherValue:     internaltypes.CaseInsensitiveString("other"),
			expectedErrors: false,
		},
		{
			name:      "case insensitive matching",
			otherPath: path.Root("other"),
			values:    []string{"TEST"},
			setValue: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("value1"),
			}),
			otherValue:     internaltypes.CaseInsensitiveString("test"),
			expectedErrors: true,
		},
		{
			name:           "null set value",
			otherPath:      path.Root("other"),
			values:         []string{"test"},
			setValue:       types.SetNull(types.StringType),
			otherValue:     internaltypes.CaseInsensitiveString("test"),
			expectedErrors: false,
		},
		{
			name:           "unknown set value",
			otherPath:      path.Root("other"),
			values:         []string{"test"},
			setValue:       types.SetUnknown(types.StringType),
			otherValue:     internaltypes.CaseInsensitiveString("test"),
			expectedErrors: false,
		},
		{
			name:      "null other value",
			otherPath: path.Root("other"),
			values:    []string{"test"},
			setValue: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("value1"),
			}),
			otherValue: internaltypes.CaseInsensitiveStringValue{
				StringValue: types.StringNull(),
			},
			expectedErrors: false,
		},
		{
			name:      "unknown other value",
			otherPath: path.Root("other"),
			values:    []string{"test"},
			setValue: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("value1"),
			}),
			otherValue: internaltypes.CaseInsensitiveStringValue{
				StringValue: types.StringUnknown(),
			},
			expectedErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a schema with the validator
			s := schema.Schema{
				Attributes: map[string]schema.Attribute{
					"test": schema.SetAttribute{
						ElementType: types.StringType,
						Required:    true,
						Validators: []validator.Set{
							SetMustBeEmptyWhen(tt.otherPath, tt.values),
						},
					},
					"other": schema.StringAttribute{
						Required:   true,
						CustomType: internaltypes.CaseInsensitiveStringType{},
					},
				},
			}

			// Create the config values
			configVal := map[string]attr.Value{
				"test":  tt.setValue,
				"other": tt.otherValue,
			}

			// Create the config object
			configObj, diags := types.ObjectValue(map[string]attr.Type{
				"test":  types.SetType{ElemType: types.StringType},
				"other": internaltypes.CaseInsensitiveStringType{},
			}, configVal)
			if diags.HasError() {
				t.Fatalf("Failed to create config object: %v", diags)
			}

			// Convert to Terraform value
			tfValue, err := configObj.ToTerraformValue(context.Background())
			if err != nil {
				t.Fatalf("Failed to convert to Terraform value: %v", err)
			}

			// Create the config
			config := tfsdk.Config{
				Raw:    tfValue,
				Schema: s,
			}

			// Create a validator request
			req := validator.SetRequest{
				Path:        path.Root("test"),
				ConfigValue: tt.setValue,
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
