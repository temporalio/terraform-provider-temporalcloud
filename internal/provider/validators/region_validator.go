package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Known valid Temporal Cloud regions as of 2026.
// See https://docs.temporal.io/cloud/regions
// This list is sourced from the Temporal Cloud API via data.temporalcloud_regions.
var validRegions = map[string]bool{
	// AWS regions (14)
	"aws-ap-northeast-1": true, // Tokyo
	"aws-ap-northeast-2": true, // Seoul
	"aws-ap-south-1":     true, // Mumbai
	"aws-ap-south-2":     true, // Hyderabad
	"aws-ap-southeast-1": true, // Singapore
	"aws-ap-southeast-2": true, // Sydney
	"aws-ca-central-1":   true, // Canada
	"aws-eu-central-1":   true, // Frankfurt
	"aws-eu-west-1":      true, // Ireland
	"aws-eu-west-2":      true, // London
	"aws-sa-east-1":      true, // Sao Paulo
	"aws-us-east-1":      true, // N. Virginia
	"aws-us-east-2":      true, // Ohio
	"aws-us-west-2":      true, // Oregon
	// GCP regions (5)
	"gcp-asia-south1":  true, // Mumbai
	"gcp-europe-west3": true, // Frankfurt
	"gcp-us-central1":  true, // Iowa
	"gcp-us-east4":     true, // N. Virginia
	"gcp-us-west1":     true, // Oregon
}

// regionFormatPattern validates that a region follows the expected format.
var regionFormatPattern = regexp.MustCompile(`^(aws|gcp)-[a-z0-9-]+$`)

type regionValidator struct{}

// Region returns a validator that checks if a string is a valid Temporal Cloud region.
func Region() validator.String {
	return &regionValidator{}
}

func (v *regionValidator) Description(ctx context.Context) string {
	return "must be a valid Temporal Cloud region (e.g., aws-us-east-1, gcp-us-central1)"
}

func (v *regionValidator) MarkdownDescription(ctx context.Context) string {
	return "must be a valid Temporal Cloud region (e.g., `aws-us-east-1`, `gcp-us-central1`). See [available regions](https://docs.temporal.io/cloud/regions)."
}

func (v *regionValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()

	// First check format
	if !regionFormatPattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Region Format",
			fmt.Sprintf("Region %q does not match expected format. Regions must be prefixed with cloud provider (e.g., aws-us-east-1, gcp-us-central1). See https://docs.temporal.io/cloud/regions", value),
		)
		return
	}

	// Then check against known valid regions
	if !validRegions[value] {
		resp.Diagnostics.AddAttributeWarning(
			req.Path,
			"Unknown Region",
			fmt.Sprintf("Region %q is not in the list of known Temporal Cloud regions. This may be a new region or a typo. See https://docs.temporal.io/cloud/regions for the current list.", value),
		)
	}
}
