package validation

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type setFieldUnique struct {
	attrFieldName string
}

func SetNestedAttributeMustBeUnique(field string) validator.Set {
	return setFieldUnique{
		attrFieldName: field,
	}
}

func (s setFieldUnique) Description(_ context.Context) string {
	return "Validates that a field in the set nested object is unique across all entries"
}

func (s setFieldUnique) MarkdownDescription(ctx context.Context) string {
	return s.Description(ctx)
}

func (s setFieldUnique) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	elements := make([]types.Object, 0, len(req.ConfigValue.Elements()))
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &elements, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	duplicates := make(map[string]struct{})
	for _, access := range elements {
		field, ok := access.Attributes()[s.attrFieldName]
		if !ok {
			resp.Diagnostics.AddError("validation: unique attribute field not found", fmt.Sprintf("attribute %s was not found in the configured set object", s.attrFieldName))
			return
		}

		// Ensure the field is a string type
		if !field.Type(ctx).Equal(types.StringType) {
			resp.Diagnostics.AddError("validation: unique attribute field is not a string", fmt.Sprintf("attribute %s is not a string", s.attrFieldName))
			return
		}

		value, err := field.ToTerraformValue(ctx)
		if err != nil {
			resp.Diagnostics.AddError("validation: field value is invalid", fmt.Sprintf("field value is invalid: %s", err))
			return
		}

		var str string
		if err := value.As(&str); err != nil {
			resp.Diagnostics.AddError("validation: failed to convert field to string", fmt.Sprintf("field value is invalid: %s", err))
			return
		}

		if _, ok := duplicates[str]; ok {
			resp.Diagnostics.AddError("validation: unique set field", fmt.Sprintf("%s must be unique accross all set entries but recieved duplicate for namespace: %s", s.attrFieldName, str))
			return
		}

		duplicates[str] = struct{}{}
	}
}
