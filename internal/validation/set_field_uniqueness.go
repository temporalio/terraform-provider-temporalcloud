package validation

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	_, ok := req.ConfigValue.ElementType(ctx).(basetypes.ObjectTypable)
	if !ok {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Validator for Element Type",
			"While performing schema-based validation, an unexpected error occurred. "+
				"The attribute declares a Object values validator, however its values do not implement the types.MapTypable interface. "+
				"Use the appropriate values validator that matches the element type. "+
				"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
				fmt.Sprintf("Path: %s\n", req.Path.String())+
				fmt.Sprintf("Element Type: %T\n", req.ConfigValue.ElementType(ctx)),
		)

		return
	}

	duplicates := make(map[string]struct{})
	for _, element := range req.ConfigValue.Elements() {
		elementPath := req.Path.AtSetValue(element)
		elementValuable, ok := element.(basetypes.ObjectValuable)

		// The check above should have prevented this, but raise an error
		// instead of a type assertion panic or skipping the element. Any issue
		// here likely indicates something wrong in the framework itself.
		if !ok {
			resp.Diagnostics.AddAttributeError(
				elementPath,
				"Invalid Validator for Element Value",
				"While performing schema-based validation, an unexpected error occurred. "+
					"The attribute declares a Object values validator, however its values do not implement the types.MapTypable interface. "+
					"This is likely an issue with terraform-plugin-framework and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Path: %s\n", req.Path.String())+
					fmt.Sprintf("Element Type: %T\n", req.ConfigValue.ElementType(ctx))+
					fmt.Sprintf("Element Value Type: %T\n", element),
			)

			return
		}

		obj, diags := elementValuable.ToObjectValue(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		field, ok := obj.Attributes()[s.attrFieldName]
		if !ok {
			resp.Diagnostics.AddError("validation: unique attribute field not found", fmt.Sprintf("attribute %s was not found in the configured set object", s.attrFieldName))
			return
		}
		if field.IsNull() || field.IsUnknown() {
			return
		}

		fieldValuable, ok := field.(basetypes.StringValuable)
		if !ok {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid Validator for Attribute Value",
				"While performing schema-based validation, an unexpected error occurred. "+
					"The attribute declares a String values validator, however its values do not implement the types.StringTypeable interface. "+
					"This is likely an issue with terraform-plugin-framework and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Path: %s\n", req.Path.String())+
					fmt.Sprintf("Element Type: %T\n", req.ConfigValue.ElementType(ctx))+
					fmt.Sprintf("Element Value Type: %T\n", element),
			)

			return
		}

		strValue, diags := fieldValuable.ToStringValue(ctx)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if strValue.IsNull() || strValue.IsUnknown() {
			return
		}

		str := strValue.ValueString()
		_, conflict := duplicates[str]
		tflog.Debug(ctx, "checking nested set attribute uniqueness", map[string]interface{}{
			"path":       elementPath.String(),
			"field_name": s.attrFieldName,
			"conflict":   conflict,
		})

		if conflict {
			resp.Diagnostics.AddAttributeError(
				elementPath,
				"validation error: unique set field",
				fmt.Sprintf("%s must be unique accross all set entries but recieved duplicate for namespace: %s", s.attrFieldName, str),
			)

			return
		}

		duplicates[str] = struct{}{}
	}
}
