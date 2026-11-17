package provider

import (
	"context"
	"slices"

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
