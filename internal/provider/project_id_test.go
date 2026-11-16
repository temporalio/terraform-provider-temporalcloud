package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// projectIDTestSchema is the smallest schema modifyPlanCreateOnlyProjectID works against; it only
// reads and writes project_id.
var projectIDTestSchema = schema.Schema{
	Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true},
		"project_id": schema.StringAttribute{Optional: true, Computed: true},
	},
}

var projectIDTestType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"id":         tftypes.String,
	"project_id": tftypes.String,
}}

// Sentinels for building project_id values; anything else is a known string.
const (
	projectIDNull    = "<null>"
	projectIDUnknown = "<unknown>"
)

func projectIDTestValue(projectID string) tftypes.Value {
	var v tftypes.Value
	switch projectID {
	case projectIDNull:
		v = tftypes.NewValue(tftypes.String, nil)
	case projectIDUnknown:
		v = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	default:
		v = tftypes.NewValue(tftypes.String, projectID)
	}
	return tftypes.NewValue(projectIDTestType, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, "ns.acct"),
		"project_id": v,
	})
}

func TestModifyPlanCreateOnlyProjectID(t *testing.T) {
	t.Parallel()

	nullObject := tftypes.NewValue(projectIDTestType, nil)

	cases := []struct {
		name string
		// Raw values, so a whole-resource null (create or destroy) can be expressed.
		config, state, plan tftypes.Value
		wantPlanProjectID   string
		wantErr             bool
	}{
		{
			name:   "create is skipped",
			config: projectIDTestValue("proj-a"),
			state:  nullObject,
			plan:   projectIDTestValue("proj-a"),
			// Create planning is left alone even with an unconfigured value: there is no prior
			// project, and the framework already plans it unknown.
			wantPlanProjectID: "proj-a",
		},
		{
			name:              "destroy is skipped",
			config:            nullObject,
			state:             projectIDTestValue("proj-a"),
			plan:              nullObject,
			wantPlanProjectID: projectIDNull,
		},
		{
			// The upgrade case: state written by a provider version without project_id. Stock
			// UseStateForUnknown copied the null in; it must go back to unknown, or the apply
			// that reads the real (default) project ID fails Terraform's consistency check.
			name:              "state predating project_id plans unknown",
			config:            projectIDTestValue(projectIDNull),
			state:             projectIDTestValue(projectIDNull),
			plan:              projectIDTestValue(projectIDNull),
			wantPlanProjectID: projectIDUnknown,
		},
		{
			// Rejecting here could block a config that merely pins the current project; the
			// Update guard decides it against the live resource.
			name:              "state predating project_id with a configured project is deferred to apply",
			config:            projectIDTestValue("proj-b"),
			state:             projectIDTestValue(projectIDNull),
			plan:              projectIDTestValue("proj-b"),
			wantPlanProjectID: "proj-b",
		},
		{
			name:              "unknown prior value with unconfigured project plans unknown",
			config:            projectIDTestValue(projectIDNull),
			state:             projectIDTestValue(projectIDUnknown),
			plan:              projectIDTestValue(projectIDUnknown),
			wantPlanProjectID: projectIDUnknown,
		},
		{
			// The everyday case for existing resources after refresh: Read has written the
			// default project's ID and config never mentions it. No diff, no error.
			name:              "unconfigured project keeps the refreshed value",
			config:            projectIDTestValue(projectIDNull),
			state:             projectIDTestValue("proj-default"),
			plan:              projectIDTestValue("proj-default"),
			wantPlanProjectID: "proj-default",
		},
		{
			name:              "pinning the current project is allowed",
			config:            projectIDTestValue("proj-a"),
			state:             projectIDTestValue("proj-a"),
			plan:              projectIDTestValue("proj-a"),
			wantPlanProjectID: "proj-a",
		},
		{
			name:              "changing the project is rejected",
			config:            projectIDTestValue("proj-b"),
			state:             projectIDTestValue("proj-a"),
			plan:              projectIDTestValue("proj-b"),
			wantPlanProjectID: "proj-b",
			wantErr:           true,
		},
		{
			// Usually a destination project created in the same apply.
			name:              "changing to an unknown project is rejected",
			config:            projectIDTestValue(projectIDUnknown),
			state:             projectIDTestValue("proj-a"),
			plan:              projectIDTestValue(projectIDUnknown),
			wantPlanProjectID: projectIDUnknown,
			wantErr:           true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			req := resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: projectIDTestSchema, Raw: tc.config},
				State:  tfsdk.State{Schema: projectIDTestSchema, Raw: tc.state},
				Plan:   tfsdk.Plan{Schema: projectIDTestSchema, Raw: tc.plan},
			}
			resp := &resource.ModifyPlanResponse{
				Plan: tfsdk.Plan{Schema: projectIDTestSchema, Raw: tc.plan.Copy()},
			}

			modifyPlanCreateOnlyProjectID(ctx, req, resp, "Widget")

			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Fatalf("HasError = %v, want %v; diagnostics: %v", got, tc.wantErr, resp.Diagnostics)
			}
			if tc.wantErr {
				for _, d := range resp.Diagnostics.Errors() {
					if d.Summary() != "Widget cannot be moved between projects" {
						t.Errorf("unexpected error summary %q", d.Summary())
					}
				}
			}

			if resp.Plan.Raw.IsNull() {
				if tc.wantPlanProjectID != projectIDNull {
					t.Fatalf("plan is null, want project_id %q", tc.wantPlanProjectID)
				}
				return
			}
			var got types.String
			resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("project_id"), &got)...)
			switch tc.wantPlanProjectID {
			case projectIDNull:
				if !got.IsNull() {
					t.Errorf("planned project_id = %s, want null", got)
				}
			case projectIDUnknown:
				if !got.IsUnknown() {
					t.Errorf("planned project_id = %s, want unknown", got)
				}
			default:
				if got.IsNull() || got.IsUnknown() || got.ValueString() != tc.wantPlanProjectID {
					t.Errorf("planned project_id = %s, want %q", got, tc.wantPlanProjectID)
				}
			}
		})
	}
}

func TestCheckProjectIDUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		planned types.String
		current string
		wantErr bool
	}{
		{name: "unconfigured", planned: types.StringNull(), current: "proj-a"},
		{name: "unknown", planned: types.StringUnknown(), current: "proj-a"},
		{name: "same project", planned: types.StringValue("proj-a"), current: "proj-a"},
		{name: "different project", planned: types.StringValue("proj-b"), current: "proj-a", wantErr: true},
		{
			// State predating project_id with a configured project: ModifyPlan deferred it, and
			// the live resource is somewhere else.
			name: "configured project differs from live default", planned: types.StringValue("proj-b"),
			current: "proj-default", wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			diags := checkProjectIDUnchanged(tc.planned, tc.current, "Widget")
			if diags.HasError() != tc.wantErr {
				t.Fatalf("HasError = %v, want %v; diagnostics: %v", diags.HasError(), tc.wantErr, diags)
			}
			if tc.wantErr && !strings.Contains(diags.Errors()[0].Detail(), `project_id is "`+tc.current+`"`) {
				t.Errorf("detail %q does not name the current project", diags.Errors()[0].Detail())
			}
		})
	}
}

func TestModifyPlanProjectID(t *testing.T) {
	t.Parallel()

	nullObject := tftypes.NewValue(projectIDTestType, nil)

	cases := []struct {
		name string
		// Raw values, so a whole-resource null (create or destroy) can be expressed.
		config, state, plan tftypes.Value
		wantPlanProjectID   string
	}{
		{
			name:   "create is skipped",
			config: projectIDTestValue("proj-a"),
			state:  nullObject,
			plan:   projectIDTestValue("proj-a"),
			// Create planning is left alone even with an unconfigured value: there is no prior
			// project, and the framework already plans it unknown.
			wantPlanProjectID: "proj-a",
		},
		{
			name:              "destroy is skipped",
			config:            nullObject,
			state:             projectIDTestValue("proj-a"),
			plan:              nullObject,
			wantPlanProjectID: projectIDNull,
		},
		{
			// The upgrade case: state written by a provider version without project_id. Stock
			// UseStateForUnknown copied the null in; it must go back to unknown, or the apply
			// that reads the real (default) project ID fails Terraform's consistency check.
			name:              "state predating project_id plans unknown",
			config:            projectIDTestValue(projectIDNull),
			state:             projectIDTestValue(projectIDNull),
			plan:              projectIDTestValue(projectIDNull),
			wantPlanProjectID: projectIDUnknown,
		},
		{
			// Rejecting here could block a config that merely pins the current project; the
			// Update guard decides it against the live resource.
			name:              "state predating project_id with a configured project is deferred to apply",
			config:            projectIDTestValue("proj-b"),
			state:             projectIDTestValue(projectIDNull),
			plan:              projectIDTestValue("proj-b"),
			wantPlanProjectID: "proj-b",
		},
		{
			name:              "unknown prior value with unconfigured project plans unknown",
			config:            projectIDTestValue(projectIDNull),
			state:             projectIDTestValue(projectIDUnknown),
			plan:              projectIDTestValue(projectIDUnknown),
			wantPlanProjectID: projectIDUnknown,
		},
		{
			// The everyday case for existing resources after refresh: Read has written the
			// default project's ID and config never mentions it. No diff, no error.
			name:              "unconfigured project keeps the refreshed value",
			config:            projectIDTestValue(projectIDNull),
			state:             projectIDTestValue("proj-default"),
			plan:              projectIDTestValue("proj-default"),
			wantPlanProjectID: "proj-default",
		},
		{
			name:              "pinning the current project is allowed",
			config:            projectIDTestValue("proj-a"),
			state:             projectIDTestValue("proj-a"),
			plan:              projectIDTestValue("proj-a"),
			wantPlanProjectID: "proj-a",
		},
		{
			name:              "changing the project plans a move",
			config:            projectIDTestValue("proj-b"),
			state:             projectIDTestValue("proj-a"),
			plan:              projectIDTestValue("proj-b"),
			wantPlanProjectID: "proj-b",
		},
		{
			// Usually a destination project created in the same apply.
			name:              "changing to an unknown project plans a move",
			config:            projectIDTestValue(projectIDUnknown),
			state:             projectIDTestValue("proj-a"),
			plan:              projectIDTestValue(projectIDUnknown),
			wantPlanProjectID: projectIDUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			req := resource.ModifyPlanRequest{
				Config: tfsdk.Config{Schema: projectIDTestSchema, Raw: tc.config},
				State:  tfsdk.State{Schema: projectIDTestSchema, Raw: tc.state},
				Plan:   tfsdk.Plan{Schema: projectIDTestSchema, Raw: tc.plan},
			}
			resp := &resource.ModifyPlanResponse{
				Plan: tfsdk.Plan{Schema: projectIDTestSchema, Raw: tc.plan.Copy()},
			}

			modifyPlanProjectID(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			if resp.Plan.Raw.IsNull() {
				if tc.wantPlanProjectID != projectIDNull {
					t.Fatalf("plan is null, want project_id %q", tc.wantPlanProjectID)
				}
				return
			}
			var got types.String
			resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("project_id"), &got)...)
			switch tc.wantPlanProjectID {
			case projectIDNull:
				if !got.IsNull() {
					t.Errorf("planned project_id = %s, want null", got)
				}
			case projectIDUnknown:
				if !got.IsUnknown() {
					t.Errorf("planned project_id = %s, want unknown", got)
				}
			default:
				if got.IsNull() || got.IsUnknown() || got.ValueString() != tc.wantPlanProjectID {
					t.Errorf("planned project_id = %s, want %q", got, tc.wantPlanProjectID)
				}
			}
		})
	}
}

func TestProjectMoveDestination(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		configured types.String
		current    string
		wantDest   string
		wantMove   bool
	}{
		{
			// The prior value an unconfigured project_id plans can be stale when the resource was
			// moved outside Terraform and the plan skipped refresh. Moving it back would undo a
			// change configuration never asked for.
			name: "unconfigured", configured: types.StringNull(), current: "proj-a",
		},
		{name: "same project", configured: types.StringValue("proj-a"), current: "proj-a"},
		{name: "different project", configured: types.StringValue("proj-b"), current: "proj-a", wantDest: "proj-b", wantMove: true},
		{
			name: "configured project differs from live default", configured: types.StringValue("proj-b"),
			current: "proj-default", wantDest: "proj-b", wantMove: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dest, move := projectMoveDestination(tc.configured, tc.current)
			if dest != tc.wantDest || move != tc.wantMove {
				t.Errorf("projectMoveDestination = (%q, %v), want (%q, %v)", dest, move, tc.wantDest, tc.wantMove)
			}
		})
	}
}

// planChangesTestSchema has a spec attribute and a computed one beside project_id, so a plan can
// differ from state in something other than project_id.
var planChangesTestSchema = schema.Schema{
	Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true},
		"name":       schema.StringAttribute{Required: true},
		"rule_ids":   schema.SetAttribute{Optional: true, ElementType: types.StringType},
		"project_id": schema.StringAttribute{Optional: true, Computed: true},
	},
}

var planChangesTestType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"id":         tftypes.String,
	"name":       tftypes.String,
	"rule_ids":   tftypes.Set{ElementType: tftypes.String},
	"project_id": tftypes.String,
}}

type planChangesTestResource struct {
	id, name, ruleID, projectID string
}

func (r planChangesTestResource) value() tftypes.Value {
	id := tftypes.NewValue(tftypes.String, r.id)
	if r.id == projectIDUnknown {
		id = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}
	return tftypes.NewValue(planChangesTestType, map[string]tftypes.Value{
		"id":   id,
		"name": tftypes.NewValue(tftypes.String, r.name),
		"rule_ids": tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{
			tftypes.NewValue(tftypes.String, r.ruleID),
		}),
		"project_id": tftypes.NewValue(tftypes.String, r.projectID),
	})
}

func TestPlanChangesOnly(t *testing.T) {
	t.Parallel()

	prior := planChangesTestResource{id: "ns.acct", name: "orders", ruleID: "rule-a", projectID: "proj-a"}

	cases := []struct {
		name       string
		plan       planChangesTestResource
		attributes []string
		want       bool
	}{
		{
			name:       "only project_id changes",
			plan:       planChangesTestResource{id: "ns.acct", name: "orders", ruleID: "rule-a", projectID: "proj-b"},
			attributes: []string{"project_id"},
			want:       true,
		},
		{
			name:       "project_id and the spec change",
			plan:       planChangesTestResource{id: "ns.acct", name: "payments", ruleID: "rule-a", projectID: "proj-b"},
			attributes: []string{"project_id"},
		},
		{
			name:       "only the spec changes",
			plan:       planChangesTestResource{id: "ns.acct", name: "payments", ruleID: "rule-a", projectID: "proj-a"},
			attributes: []string{"project_id"},
		},
		{
			name:       "a changed set that is not named is a change",
			plan:       planChangesTestResource{id: "ns.acct", name: "orders", ruleID: "rule-b", projectID: "proj-b"},
			attributes: []string{"project_id"},
		},
		{
			name:       "a changed set that is named is ignored",
			plan:       planChangesTestResource{id: "ns.acct", name: "orders", ruleID: "rule-b", projectID: "proj-b"},
			attributes: []string{"project_id", "rule_ids"},
			want:       true,
		},
		{
			// A computed value the plan leaves unknown is not something configuration asked to
			// change.
			name:       "unknown computed value is not a change",
			plan:       planChangesTestResource{id: projectIDUnknown, name: "orders", ruleID: "rule-a", projectID: "proj-b"},
			attributes: []string{"project_id"},
			want:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := planChangesOnly(
				tfsdk.Plan{Schema: planChangesTestSchema, Raw: tc.plan.value()},
				tfsdk.State{Schema: planChangesTestSchema, Raw: prior.value()},
				tc.attributes...,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("planChangesOnly = %v, want %v", got, tc.want)
			}
		})
	}
}
