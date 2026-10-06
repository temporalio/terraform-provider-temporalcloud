package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// modifyPlanCreateOnlyProjectID enforces that project_id, which is set on create and absent from
// every update request, is never changed on an existing resource. kind names the resource in
// diagnostics, e.g. "Namespace".
//
// Replacement is deliberately not used -- destroying a project-scoped resource should be a
// deliberate act rather than a side effect of editing an attribute, and `apply -auto-approve` is
// normal in CI.
//
// The resource's Update must also call checkProjectIDUnchanged, for the cases this hook cannot
// decide, where the resource's current project is unknown at plan time.
//
// If a move API lands, delete both guards and call it from Update. It is expected to be its own
// RPC, so it bumps resource_version: Update cannot reuse the version it fetched for the spec
// write, and the timeout then has to cover two async operations.
func modifyPlanCreateOnlyProjectID(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, kind string) {
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

	// A null or unknown prior value means the resource's current project is not known here --
	// typically state written before this attribute existed, planned with -refresh=false so Read
	// has not filled it in. There is nothing to compare against, and rejecting would block a
	// config that merely pins the project the resource is already in. Update decides these by
	// comparing against the live resource.
	//
	// Restore unknown first. UseStateForUnknown guards on the whole resource's state being null
	// (i.e. "is this a create"), not the attribute's, so it has already copied that null prior
	// value into the plan. Leaving it there would promise null while the apply writes the real
	// project ID, which Terraform rejects as an inconsistent result after apply. Unknown is the
	// honest plan: the project is not known until the resource is read.
	if prior.IsNull() || prior.IsUnknown() {
		if configured.IsNull() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("project_id"), types.StringUnknown())...)
		}
		return
	}

	// An unknown configured value cannot be compared. Rejected here rather than deferred so the
	// failure lands at plan time: the Update guard would still catch it at apply, but Terraform's
	// own consistency check would not, since it accepts any applied value where the plan was
	// unknown. Usually caused by a target project created in this same apply, which a resource
	// that already exists cannot be in.
	if configured.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("project_id"),
			projectMoveNotSupportedSummary(kind),
			projectMoveNotSupportedDetail(kind, strconv.Quote(prior.ValueString()), "a project created by this configuration"),
		)
		return
	}

	// Config, not Plan: by now Plan has prior state copied into it, so an omitted attribute and one
	// set to the current project are indistinguishable there.
	if configured.IsNull() || configured.Equal(prior) {
		return
	}

	resp.Diagnostics.AddAttributeError(
		path.Root("project_id"),
		projectMoveNotSupportedSummary(kind),
		projectMoveNotSupportedDetail(kind, strconv.Quote(prior.ValueString()), strconv.Quote(configured.ValueString())),
	)
}

// checkProjectIDUnchanged is the apply-time half of modifyPlanCreateOnlyProjectID, comparing the
// planned project_id against the one just fetched from the API. It covers what ModifyPlan
// deliberately skips: a change whose prior project was not known at plan time. Comparing against
// the live resource rather than against state also catches a server-side move since the last
// refresh. Call it before any write, since update requests carry no project_id and would
// otherwise succeed while silently dropping the change.
func checkProjectIDUnchanged(planned types.String, current string, kind string) diag.Diagnostics {
	var diags diag.Diagnostics
	if planned.IsNull() || planned.IsUnknown() || planned.ValueString() == current {
		return diags
	}
	diags.AddAttributeError(
		path.Root("project_id"),
		projectMoveNotSupportedSummary(kind),
		projectMoveNotSupportedDetail(kind, strconv.Quote(current), strconv.Quote(planned.ValueString())),
	)
	return diags
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
