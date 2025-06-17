package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

type setEmptyWhen struct {
	otherPath path.Path
	values    []string
}

// SetMustBeEmptyWhen returns a validator that ensures a set is empty when another attribute has any of the specified values.
func SetMustBeEmptyWhen(otherPath path.Path, values []string) validator.Set {
	return setEmptyWhen{
		otherPath: otherPath,
		values:    values,
	}
}

func (s setEmptyWhen) Description(_ context.Context) string {
	return fmt.Sprintf("Validates that the set is empty when the attribute at %s has any of the values: %s", s.otherPath, strings.Join(s.values, ", "))
}

func (s setEmptyWhen) MarkdownDescription(ctx context.Context) string {
	return s.Description(ctx)
}

func (s setEmptyWhen) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// Get the value of the other attribute
	var otherValue types.CaseInsensitiveStringValue
	diags := req.Config.GetAttribute(ctx, s.otherPath, &otherValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if otherValue.IsNull() || otherValue.IsUnknown() {
		return
	}

	otherValueStr := strings.ToLower(otherValue.ValueString())
	for _, value := range s.values {
		if strings.ToLower(value) == otherValueStr {
			// If the other value matches and the set is not empty, return an error
			if len(req.ConfigValue.Elements()) > 0 {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Set Value",
					fmt.Sprintf("%s must be empty when %s is %s", req.Path, s.otherPath, otherValueStr),
				)
			}
			return
		}
	}
}
