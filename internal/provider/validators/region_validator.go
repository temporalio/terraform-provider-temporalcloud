package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var regionFormatPattern = regexp.MustCompile(`^(aws|gcp|azure)-[a-z0-9-]+$`)

type regionFormatValidator struct{}

// RegionFormat returns a validator that checks if a string matches the expected
// Temporal Cloud region format (e.g., aws-us-east-1, gcp-us-central1).
// It does not verify that the region actually exists — that is handled by
// ModifyPlan via the GetRegions API.
func RegionFormat() validator.String {
	return &regionFormatValidator{}
}

func (v *regionFormatValidator) Description(ctx context.Context) string {
	return "must be a valid Temporal Cloud region format (e.g., aws-us-east-1, gcp-us-central1, azure-eastus)"
}

func (v *regionFormatValidator) MarkdownDescription(ctx context.Context) string {
	return "must be a valid Temporal Cloud region format (e.g., `aws-us-east-1`, `gcp-us-central1`, `azure-eastus`). See [available regions](https://docs.temporal.io/cloud/regions)."
}

func (v *regionFormatValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()

	if !regionFormatPattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Region Format",
			fmt.Sprintf("Region %q does not match expected format. Regions must be prefixed with cloud provider (e.g., aws-us-east-1, gcp-us-central1, azure-eastus). See https://docs.temporal.io/cloud/regions", value),
		)
	}
}
