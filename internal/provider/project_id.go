package provider

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// modifyPlanProjectID plans project_id for an existing resource. A configured change is planned
// as an in-place update, which the resource's Update performs with its move RPC.
//
// It only has work to do when the prior value is null or unknown -- typically state written before
// this attribute existed, planned with -refresh=false so Read has not filled it in. UseStateForUnknown
// guards on the whole resource's state being null (i.e. "is this a create"), not the attribute's, so
// it has already copied that null prior value into the plan. Leaving it there would promise null
// while the apply writes the real project ID, which Terraform rejects as an inconsistent result
// after apply. Unknown is the honest plan: the project is not known until the resource is read.
func modifyPlanProjectID(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip on create and destroy; there is no prior project.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var configured, prior types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("project_id"), &configured)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("project_id"), &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if (prior.IsNull() || prior.IsUnknown()) && configured.IsNull() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("project_id"), types.StringUnknown())...)
	}
}

// projectMovePlanned reports whether a plan moves an existing resource to another project. An
// unknown configured value counts: it is usually a project created in the same apply, which the
// resource cannot already be in. An unknown prior project does not, since there is nothing to
// compare against until apply reads the live resource.
func projectMovePlanned(configured, prior types.String) bool {
	if configured.IsNull() || prior.IsNull() || prior.IsUnknown() {
		return false
	}
	return configured.IsUnknown() || !configured.Equal(prior)
}

// projectMoveDestination returns the project Update must move the resource to, comparing the
// configured project_id against the resource's live project.
//
// Config, not Plan: an unconfigured project_id plans the prior state's value, which is stale when
// the resource was moved outside Terraform and the plan skipped refresh. Moving it back would undo
// a change configuration never asked for.
func projectMoveDestination(configured types.String, current string) (string, bool) {
	if configured.IsNull() || configured.IsUnknown() || configured.ValueString() == current {
		return "", false
	}
	return configured.ValueString(), true
}

// planChangesOnly reports whether the plan differs from prior state in the named top-level
// attributes alone. After a move has applied them, the spec update RPCs would reject a request
// that changes nothing, so the update is skipped. Computed values the plan leaves unknown are not
// changes configuration asked for, so they compare as their prior values.
func planChangesOnly(plan tfsdk.Plan, state tfsdk.State, attributes ...string) (bool, error) {
	ignored := make([]*tftypes.AttributePath, len(attributes))
	for i, name := range attributes {
		ignored[i] = tftypes.NewAttributePath().WithAttributeName(name)
	}
	isIgnored := func(p *tftypes.AttributePath) bool {
		return slices.ContainsFunc(ignored, p.Equal)
	}

	compared, err := tftypes.Transform(plan.Raw, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if v.IsKnown() && !isIgnored(p) {
			return v, nil
		}
		if prior, ok := valueAt(state.Raw, p); ok {
			return prior, nil
		}
		return v, nil
	})
	if err != nil {
		return false, err
	}
	return compared.Equal(state.Raw), nil
}

// valueAt returns the value at p, or false when v has nothing there.
func valueAt(v tftypes.Value, p *tftypes.AttributePath) (tftypes.Value, bool) {
	found, _, err := tftypes.WalkAttributePath(v, p)
	if err != nil {
		return tftypes.Value{}, false
	}
	value, ok := found.(tftypes.Value)
	return value, ok
}

// modifyPlanImmutableOptionalProjectID rejects any change to project_id on an existing resource
// that has no move API. The project_id is Optional but not Computed, so null is a real value
// meaning "not in a project" rather than "not known yet". Null to a project and a project to null
// are rejected too. kind names the resource in diagnostics, e.g. "Custom role".
//
// The resource's Update must also call checkOptionalProjectIDUnchanged, which compares against the
// live resource and catches state that is stale because the plan was made with -refresh=false.
func modifyPlanImmutableOptionalProjectID(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, kind string) {
	// Skip on create and destroy; there is no prior project to move away from.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var configured, prior types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("project_id"), &configured)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("project_id"), &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An unknown configured value cannot be compared, so it's rejected here to fail at plan time.
	// Usually caused by a target project created in this same apply, which a resource that already
	// exists cannot be in.
	if configured.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("project_id"),
			projectMoveNotSupportedSummary(kind),
			projectMoveNotSupportedDetail(kind, describeOptionalProjectID(prior), "a project created by this configuration"),
		)
		return
	}

	// Config, not Plan: with no Computed flag the two are the same here, but Config is what the
	// user asked for.
	if configured.Equal(prior) {
		return
	}

	resp.Diagnostics.AddAttributeError(
		path.Root("project_id"),
		projectMoveNotSupportedSummary(kind),
		projectMoveNotSupportedDetail(kind, describeOptionalProjectID(prior), describeOptionalProjectID(configured)),
	)
}

// checkOptionalProjectIDUnchanged is the apply-time half of modifyPlanImmutableOptionalProjectID,
// comparing the planned project_id against the one just fetched from the API, which catches state
// made stale by -refresh=false. A null plan is compared too: it means the resource is not in a
// project.
func checkOptionalProjectIDUnchanged(planned types.String, current string, kind string) diag.Diagnostics {
	var diags diag.Diagnostics
	if planned.IsUnknown() || planned.ValueString() == current {
		return diags
	}
	currentValue := types.StringNull()
	if current != "" {
		currentValue = types.StringValue(current)
	}
	diags.AddAttributeError(
		path.Root("project_id"),
		projectMoveNotSupportedSummary(kind),
		projectMoveNotSupportedDetail(kind, describeOptionalProjectID(currentValue), describeOptionalProjectID(planned)),
	)
	return diags
}

// describeOptionalProjectID renders an Optional project_id for diagnostics, where null means the
// resource is not in a project.
func describeOptionalProjectID(v types.String) string {
	if v.IsNull() || v.ValueString() == "" {
		return "unset (account-scoped)"
	}
	return strconv.Quote(v.ValueString())
}

func projectMoveNotSupportedSummary(kind string) string {
	return fmt.Sprintf("%s cannot be moved between projects", kind)
}

func projectMoveNotSupportedDetail(kind, from, to string) string {
	return fmt.Sprintf(
		"project_id is %s and cannot be changed to %s. A %s cannot be moved between projects.",
		from, to, kind,
	)
}
