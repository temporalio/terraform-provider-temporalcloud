package provider

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"google.golang.org/protobuf/proto"

	namespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
)

func testCallerRateLimitNamespaceDefault(rps float64) types.Object {
	return types.ObjectValueMust(callerRateLimitNamespaceDefaultAttrTypes, map[string]attr.Value{
		"rps": types.Float64Value(rps),
	})
}

func testCallerRateLimitNamespaceLimit(namespaceID string, rps float64) types.Object {
	return types.ObjectValueMust(callerRateLimitNamespaceLimitAttrTypes, map[string]attr.Value{
		"namespace_id": types.StringValue(namespaceID),
		"rps":          types.Float64Value(rps),
	})
}

// testCallerRateLimitModel builds a model with the given rule blocks; pass a null object to leave
// a block unset.
func testCallerRateLimitModel(namespaceDefault, namespaceLimit types.Object) callerRateLimitResourceModel {
	return callerRateLimitResourceModel{
		ID:               types.StringValue("target-ns.abc12/rl-1"),
		NamespaceID:      types.StringValue("target-ns.abc12"),
		NamespaceDefault: namespaceDefault,
		NamespaceLimit:   namespaceLimit,
	}
}

var (
	testCallerRateLimitNoDefault = types.ObjectNull(callerRateLimitNamespaceDefaultAttrTypes)
	testCallerRateLimitNoLimit   = types.ObjectNull(callerRateLimitNamespaceLimitAttrTypes)
)

func testCallerRateLimitDefaultSpec(rps float64) *namespacev1.CallerRateLimitSpec {
	return &namespacev1.CallerRateLimitSpec{
		Rule: &namespacev1.CallerRateLimitSpec_NamespaceDefault{
			NamespaceDefault: &namespacev1.NamespaceDefaultCallerRateLimit{Rps: rps},
		},
	}
}

func testCallerRateLimitLimitSpec(namespaceID string, rps float64) *namespacev1.CallerRateLimitSpec {
	return &namespacev1.CallerRateLimitSpec{
		Rule: &namespacev1.CallerRateLimitSpec_NamespaceLimit{
			NamespaceLimit: &namespacev1.NamespaceCallerRateLimit{NamespaceId: namespaceID, Rps: rps},
		},
	}
}

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

	rule := testCallerRateLimitNamespaceDefault
	null := types.ObjectNull(callerRateLimitNamespaceDefaultAttrTypes)
	unknown := types.ObjectUnknown(callerRateLimitNamespaceDefaultAttrTypes)

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

// TestAccCallerRateLimitResource uses the target namespace as its own caller, which the server
// allows, so the test only needs one namespace.
func TestAccCallerRateLimitResource(t *testing.T) {
	targetName := fmt.Sprintf("tf-test-crl-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccCallerRateLimitResourceConfig(targetName, "10", "5", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("temporalcloud_caller_rate_limit.default", "id"),
					resource.TestCheckResourceAttrPair("temporalcloud_caller_rate_limit.default", "namespace_id", "temporalcloud_namespace.target", "id"),
					resource.TestCheckResourceAttr("temporalcloud_caller_rate_limit.default", "namespace_default.rps", "10"),
					resource.TestCheckNoResourceAttr("temporalcloud_caller_rate_limit.default", "namespace_limit"),
					resource.TestCheckResourceAttrSet("temporalcloud_caller_rate_limit.limit", "id"),
					resource.TestCheckResourceAttrPair("temporalcloud_caller_rate_limit.limit", "namespace_id", "temporalcloud_namespace.target", "id"),
					resource.TestCheckResourceAttrPair("temporalcloud_caller_rate_limit.limit", "namespace_limit.namespace_id", "temporalcloud_namespace.target", "id"),
					resource.TestCheckResourceAttr("temporalcloud_caller_rate_limit.limit", "namespace_limit.rps", "5"),
					resource.TestCheckNoResourceAttr("temporalcloud_caller_rate_limit.limit", "namespace_default"),
				),
			},
			// Update testing: rps changes in place, including down to 0, which blocks the caller.
			{
				Config: testAccCallerRateLimitResourceConfig(targetName, "2.5", "0", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("temporalcloud_caller_rate_limit.default", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("temporalcloud_caller_rate_limit.limit", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_caller_rate_limit.default", "namespace_default.rps", "2.5"),
					resource.TestCheckResourceAttr("temporalcloud_caller_rate_limit.limit", "namespace_limit.rps", "0"),
				),
			},
			// A second default rule for the same target is rejected by the server.
			{
				Config: testAccCallerRateLimitResourceConfig(targetName, "2.5", "0", `
resource "temporalcloud_caller_rate_limit" "duplicate" {
  namespace_id = temporalcloud_namespace.target.id
  namespace_default = {
    rps = 1
  }
}
`),
				ExpectError: regexp.MustCompile(`caller rate limit already exists`),
			},
			// ImportState testing
			{
				Config:                  testAccCallerRateLimitResourceConfig(targetName, "2.5", "0", ""),
				ResourceName:            "temporalcloud_caller_rate_limit.default",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				Config:                  testAccCallerRateLimitResourceConfig(targetName, "2.5", "0", ""),
				ResourceName:            "temporalcloud_caller_rate_limit.limit",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccCallerRateLimitResource_Replace(t *testing.T) {
	targetName := fmt.Sprintf("tf-test-crl-target-%s", randomString(8))
	callerName := fmt.Sprintf("tf-test-crl-caller-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCallerRateLimitResourceReplaceConfig(targetName, callerName, `
  namespace_default = {
    rps = 10
  }
`),
				Check: resource.TestCheckResourceAttr("temporalcloud_caller_rate_limit.test", "namespace_default.rps", "10"),
			},
			// Switching the rule type replaces the resource.
			{
				Config: testAccCallerRateLimitResourceReplaceConfig(targetName, callerName, `
  namespace_limit = {
    namespace_id = temporalcloud_namespace.target.id
    rps          = 10
  }
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("temporalcloud_caller_rate_limit.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("temporalcloud_caller_rate_limit.test", "namespace_default"),
					resource.TestCheckResourceAttrPair("temporalcloud_caller_rate_limit.test", "namespace_limit.namespace_id", "temporalcloud_namespace.target", "id"),
				),
			},
			// Changing the caller replaces the resource.
			{
				Config: testAccCallerRateLimitResourceReplaceConfig(targetName, callerName, `
  namespace_limit = {
    namespace_id = temporalcloud_namespace.caller.id
    rps          = 10
  }
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("temporalcloud_caller_rate_limit.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttrPair("temporalcloud_caller_rate_limit.test", "namespace_limit.namespace_id", "temporalcloud_namespace.caller", "id"),
			},
			// Back to a default rule. If an earlier Delete were a no-op, the leftover default rule would fail this with "already exists".
			{
				Config: testAccCallerRateLimitResourceReplaceConfig(targetName, callerName, `
  namespace_default = {
    rps = 20
  }
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("temporalcloud_caller_rate_limit.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_caller_rate_limit.test", "namespace_default.rps", "20"),
					resource.TestCheckNoResourceAttr("temporalcloud_caller_rate_limit.test", "namespace_limit"),
				),
			},
		},
	})
}

// testAccCallerRateLimitResourceConfig has a default rule and a rule for the target calling itself.
// extra is appended as-is.
func testAccCallerRateLimitResourceConfig(targetName, defaultRps, limitRps, extra string) string {
	return fmt.Sprintf(`
provider "temporalcloud" {
}

%[1]s

resource "temporalcloud_caller_rate_limit" "default" {
  namespace_id = temporalcloud_namespace.target.id
  namespace_default = {
    rps = %[2]s
  }
}

resource "temporalcloud_caller_rate_limit" "limit" {
  namespace_id = temporalcloud_namespace.target.id
  namespace_limit = {
    namespace_id = temporalcloud_namespace.target.id
    rps          = %[3]s
  }
}
%[4]s
`, testAccNamespaceResourceConfig("target", targetName), defaultRps, limitRps, extra)
}

// testAccCallerRateLimitResourceReplaceConfig keeps both namespaces in every step and puts rule
// inside the caller rate limit.
func testAccCallerRateLimitResourceReplaceConfig(targetName, callerName, rule string) string {
	return fmt.Sprintf(`
provider "temporalcloud" {
}

%[1]s
%[2]s

resource "temporalcloud_caller_rate_limit" "test" {
  namespace_id = temporalcloud_namespace.target.id
%[3]s
}
`, testAccNamespaceResourceConfig("target", targetName), testAccNamespaceResourceConfig("caller", callerName), rule)
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

func TestGetCallerRateLimitSpecFromModel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		model    callerRateLimitResourceModel
		wantSpec *namespacev1.CallerRateLimitSpec
		wantErr  string
	}{
		{
			name:     "namespace default",
			model:    testCallerRateLimitModel(testCallerRateLimitNamespaceDefault(10), testCallerRateLimitNoLimit),
			wantSpec: testCallerRateLimitDefaultSpec(10),
		},
		{
			name:     "namespace limit",
			model:    testCallerRateLimitModel(testCallerRateLimitNoDefault, testCallerRateLimitNamespaceLimit("caller-ns.abc12", 5)),
			wantSpec: testCallerRateLimitLimitSpec("caller-ns.abc12", 5),
		},
		{
			name:     "fractional rps",
			model:    testCallerRateLimitModel(testCallerRateLimitNoDefault, testCallerRateLimitNamespaceLimit("caller-ns.abc12", 0.1)),
			wantSpec: testCallerRateLimitLimitSpec("caller-ns.abc12", 0.1),
		},
		{
			name:    "no rule",
			model:   testCallerRateLimitModel(testCallerRateLimitNoDefault, testCallerRateLimitNoLimit),
			wantErr: "Missing caller rate limit rule",
		},
		{
			name:    "both rules",
			model:   testCallerRateLimitModel(testCallerRateLimitNamespaceDefault(10), testCallerRateLimitNamespaceLimit("caller-ns.abc12", 5)),
			wantErr: "Multiple caller rate limit rules configured",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, diags := getCallerRateLimitSpecFromModel(context.Background(), &tc.model)

			if tc.wantErr != "" {
				if !diags.HasError() || diags.Errors()[0].Summary() != tc.wantErr {
					t.Fatalf("diagnostics = %v, want error %q", diags, tc.wantErr)
				}
				if spec != nil {
					t.Errorf("spec = %v, want nil", spec)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			// proto.Equal compares float64 fields exactly, so 0.1 must come through unchanged.
			if !proto.Equal(spec, tc.wantSpec) {
				t.Errorf("spec = %v, want %v", spec, tc.wantSpec)
			}
		})
	}
}

func TestUpdateCallerRateLimitModelFromSpec(t *testing.T) {
	t.Parallel()

	// Prior state with stale values and the opposite rule type, so anything left unwritten shows up.
	staleWithLimit := testCallerRateLimitModel(testCallerRateLimitNoDefault, testCallerRateLimitNamespaceLimit("stale-ns.abc12", 1))
	staleWithLimit.ID = types.StringValue("stale")
	staleWithLimit.NamespaceID = types.StringValue("stale")
	staleWithDefault := staleWithLimit
	staleWithDefault.NamespaceDefault = testCallerRateLimitNamespaceDefault(1)
	staleWithDefault.NamespaceLimit = testCallerRateLimitNoLimit

	cases := []struct {
		name      string
		state     callerRateLimitResourceModel
		rl        *namespacev1.CallerRateLimit
		wantModel callerRateLimitResourceModel
		wantErr   string
	}{
		{
			// The response namespace differs from the parameter, so the test shows the parameter wins.
			name:      "namespace default",
			state:     staleWithLimit,
			rl:        &namespacev1.CallerRateLimit{Id: "rl-1", Namespace: "other-ns.abc12", Spec: testCallerRateLimitDefaultSpec(10)},
			wantModel: testCallerRateLimitModel(testCallerRateLimitNamespaceDefault(10), testCallerRateLimitNoLimit),
		},
		{
			name:      "namespace limit with no response namespace",
			state:     staleWithDefault,
			rl:        &namespacev1.CallerRateLimit{Id: "rl-1", Spec: testCallerRateLimitLimitSpec("caller-ns.abc12", 0.1)},
			wantModel: testCallerRateLimitModel(testCallerRateLimitNoDefault, testCallerRateLimitNamespaceLimit("caller-ns.abc12", 0.1)),
		},
		{
			name:    "nil spec",
			state:   staleWithLimit,
			rl:      &namespacev1.CallerRateLimit{Id: "rl-1"},
			wantErr: `"rl-1"`,
		},
		{
			name:    "nil rule",
			state:   staleWithLimit,
			rl:      &namespacev1.CallerRateLimit{Id: "rl-1", Spec: &namespacev1.CallerRateLimitSpec{}},
			wantErr: `"rl-1"`,
		},
		{
			name:    "empty id",
			state:   staleWithLimit,
			rl:      &namespacev1.CallerRateLimit{Spec: testCallerRateLimitDefaultSpec(10)},
			wantErr: `"target-ns.abc12"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := tc.state

			diags := updateCallerRateLimitModelFromSpec(context.Background(), &state, tc.rl, "target-ns.abc12")

			if tc.wantErr != "" {
				if !diags.HasError() {
					t.Fatal("expected an error")
				}
				if !strings.Contains(diags.Errors()[0].Detail(), tc.wantErr) {
					t.Errorf("error detail %q does not contain %s", diags.Errors()[0].Detail(), tc.wantErr)
				}
				if !reflect.DeepEqual(state, tc.state) {
					t.Errorf("state changed on error: got %+v, want %+v", state, tc.state)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if !reflect.DeepEqual(state, tc.wantModel) {
				t.Errorf("state = %+v, want %+v", state, tc.wantModel)
			}
		})
	}
}

func TestParseCallerRateLimitID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                  string
		id                    string
		wantNamespaceID       string
		wantCallerRateLimitID string
		wantErr               bool
	}{
		// Built with callerRateLimitID, so this is also the build/parse round trip.
		{name: "valid", id: callerRateLimitID("my-ns.abc12", "rl-1"), wantNamespaceID: "my-ns.abc12", wantCallerRateLimitID: "rl-1"},
		{name: "no separator", id: "my-ns.abc12", wantErr: true},
		{name: "extra separator", id: "my-ns.abc12/rl-1/extra", wantErr: true},
		{name: "empty namespace", id: "/rl-1", wantErr: true},
		{name: "empty caller rate limit id", id: "my-ns.abc12/", wantErr: true},
		{name: "empty string", id: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			namespaceID, callerRateLimitID, err := parseCallerRateLimitID(tc.id)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got (%q, %q)", namespaceID, callerRateLimitID)
				}
				if !strings.Contains(err.Error(), "<namespace_id>/<caller_rate_limit_id>") {
					t.Errorf("error %q does not show the expected format", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if namespaceID != tc.wantNamespaceID || callerRateLimitID != tc.wantCallerRateLimitID {
				t.Errorf("got (%q, %q), want (%q, %q)", namespaceID, callerRateLimitID, tc.wantNamespaceID, tc.wantCallerRateLimitID)
			}
		})
	}
}

func TestCallerRateLimitCreateErrorDetail(t *testing.T) {
	t.Parallel()

	err := errors.New("operation failed")

	cases := []struct {
		name        string
		rateLimitID string
		want        string
	}{
		{
			name:        "id present",
			rateLimitID: "rl-1",
			want:        "operation failed. The caller rate limit may still exist; import it with: terraform import <resource address> my-ns.abc12/rl-1",
		},
		{
			// Nothing to import without an id, so the hint would only show a broken id.
			name:        "id empty",
			rateLimitID: "",
			want:        "operation failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := callerRateLimitCreateErrorDetail(err, "my-ns.abc12", tc.rateLimitID); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
