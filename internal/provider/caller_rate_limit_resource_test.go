package provider

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestCallerRateLimitSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewCallerRateLimitResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestCallerRateLimitRuleTypeChanged(t *testing.T) {
	t.Parallel()

	ruleAttrTypes := map[string]attr.Type{"rps": types.Float64Type}
	rule := func(rps float64) types.Object {
		return types.ObjectValueMust(ruleAttrTypes, map[string]attr.Value{"rps": types.Float64Value(rps)})
	}
	null := types.ObjectNull(ruleAttrTypes)
	unknown := types.ObjectUnknown(ruleAttrTypes)

	cases := []struct {
		name        string
		state, plan types.Object
		wantReplace bool
	}{
		{
			// The other block was removed in the same change, so the rule type switched.
			name:        "block added",
			state:       null,
			plan:        rule(10),
			wantReplace: true,
		},
		{
			name:        "block removed",
			state:       rule(10),
			plan:        null,
			wantReplace: true,
		},
		{
			name:        "rps changed updates in place",
			state:       rule(10),
			plan:        rule(20),
			wantReplace: false,
		},
		{
			// Nested plan modifiers don't run on an unknown object, so the caller can't be checked.
			name:        "unknown plan with existing block",
			state:       rule(10),
			plan:        unknown,
			wantReplace: true,
		},
		{
			name:        "unknown plan with no prior block",
			state:       null,
			plan:        unknown,
			wantReplace: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := planmodifier.ObjectRequest{StateValue: tc.state, PlanValue: tc.plan}
			resp := &objectplanmodifier.RequiresReplaceIfFuncResponse{}

			callerRateLimitRuleTypeChanged(context.Background(), req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if resp.RequiresReplace != tc.wantReplace {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, tc.wantReplace)
			}
		})
	}
}

func TestAccCallerRateLimitResource_ValidationErrors(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCallerRateLimitResourceConfig_BothRules(),
				// Should fail validation before reaching the API
				ExpectError: regexp.MustCompile(`2\s+attributes\s+specified\s+when\s+one\s+\(and\s+only\s+one\)`),
			},
			{
				Config:      testAccCallerRateLimitResourceConfig_NoRule(),
				ExpectError: regexp.MustCompile(`No\s+attribute\s+specified\s+when\s+one\s+\(and\s+only\s+one\)`),
			},
			{
				Config:      testAccCallerRateLimitResourceConfig_NegativeDefaultRps(),
				ExpectError: regexp.MustCompile(`namespace_default\.rps\s+value\s+must\s+be\s+at\s+least\s+0`),
			},
			{
				Config:      testAccCallerRateLimitResourceConfig_NegativeLimitRps(),
				ExpectError: regexp.MustCompile(`namespace_limit\.rps\s+value\s+must\s+be\s+at\s+least\s+0`),
			},
		},
	})
}

func testAccCallerRateLimitResourceConfig_BothRules() string {
	return `
provider "temporalcloud" {
}

resource "temporalcloud_caller_rate_limit" "test" {
  namespace_id = "fake-ns.abc12"
  namespace_default = {
    rps = 10
  }
  namespace_limit = {
    namespace_id = "fake-caller.abc12"
    rps          = 10
  }
}
`
}

func testAccCallerRateLimitResourceConfig_NoRule() string {
	return `
provider "temporalcloud" {
}

resource "temporalcloud_caller_rate_limit" "test" {
  namespace_id = "fake-ns.abc12"
}
`
}

func testAccCallerRateLimitResourceConfig_NegativeDefaultRps() string {
	return `
provider "temporalcloud" {
}

resource "temporalcloud_caller_rate_limit" "test" {
  namespace_id = "fake-ns.abc12"
  namespace_default = {
    rps = -1
  }
}
`
}

func testAccCallerRateLimitResourceConfig_NegativeLimitRps() string {
	return `
provider "temporalcloud" {
}

resource "temporalcloud_caller_rate_limit" "test" {
  namespace_id = "fake-ns.abc12"
  namespace_limit = {
    namespace_id = "fake-caller.abc12"
    rps          = -1
  }
}
`
}
