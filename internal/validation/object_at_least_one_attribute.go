package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type objectAtLeastOneAttribute struct {
	attrNames []string
}

// ObjectAtLeastOneAttribute validates that at least one of the named attributes of an object is
// set. Unlike objectvalidator.AtLeastOneOf, it only looks at the object's own attributes, so it
// works on the nested object of a set: AtLeastOneOf counts the object itself, which is never null
// there, so it always passes.
func ObjectAtLeastOneAttribute(attrNames ...string) validator.Object {
	return objectAtLeastOneAttribute{attrNames: attrNames}
}

func (v objectAtLeastOneAttribute) Description(_ context.Context) string {
	return fmt.Sprintf("At least one of these attributes must be set: %s", strings.Join(v.attrNames, ", "))
}

func (v objectAtLeastOneAttribute) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v objectAtLeastOneAttribute) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()
	for _, name := range v.attrNames {
		value, ok := attrs[name]
		if !ok {
			continue
		}
		// An unknown value may turn out to be set, so leave it to apply.
		if !value.IsNull() {
			return
		}
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Missing Attribute Configuration",
		v.Description(ctx)+".",
	)
}
