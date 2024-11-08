package modifiers

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewCaseInsensitivePlanModifier(description, markdownDescription string) planmodifier.String {
	return &caseInsensitivePlanModifier{
		description:         description,
		markdownDescription: markdownDescription,
	}
}

type caseInsensitivePlanModifier struct {
	description         string
	markdownDescription string
}

// Description returns a human-readable description of the plan modifier.
func (m caseInsensitivePlanModifier) Description(_ context.Context) string {
	return "If the value of this attribute changes (type case-insensitive), update the resource accordingly."
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m caseInsensitivePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyString implements the plan modification logic.
func (m caseInsensitivePlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() {
		return
	}

	if req.Plan.Raw.IsNull() {
		return
	}

	if strings.EqualFold(req.PlanValue.ValueString(), req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.StringValue(req.PlanValue.ValueString())
}
