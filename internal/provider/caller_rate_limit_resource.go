package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	namespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
)

type (
	callerRateLimitResource struct {
		client *client.Client
	}

	callerRateLimitResourceModel struct {
		ID               types.String   `tfsdk:"id"`
		NamespaceID      types.String   `tfsdk:"namespace_id"`
		NamespaceDefault types.Object   `tfsdk:"namespace_default"`
		NamespaceLimit   types.Object   `tfsdk:"namespace_limit"`
		Timeouts         timeouts.Value `tfsdk:"timeouts"`
	}

	callerRateLimitNamespaceDefaultModel struct {
		Rps types.Float64 `tfsdk:"rps"`
	}

	callerRateLimitNamespaceLimitModel struct {
		NamespaceID types.String  `tfsdk:"namespace_id"`
		Rps         types.Float64 `tfsdk:"rps"`
	}
)

var (
	callerRateLimitNamespaceDefaultAttrTypes = map[string]attr.Type{
		"rps": types.Float64Type,
	}

	callerRateLimitNamespaceLimitAttrTypes = map[string]attr.Type{
		"namespace_id": types.StringType,
		"rps":          types.Float64Type,
	}
)

func NewCallerRateLimitResource() resource.Resource {
	return &callerRateLimitResource{}
}

var (
	_ resource.Resource                = (*callerRateLimitResource)(nil)
	_ resource.ResourceWithConfigure   = (*callerRateLimitResource)(nil)
	_ resource.ResourceWithImportState = (*callerRateLimitResource)(nil)
)

func (r *callerRateLimitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *callerRateLimitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_caller_rate_limit"
}

// callerRateLimitRuleTypeChanged requires replacement when a rule block is added or removed, which
// is how switching between namespace_default and namespace_limit shows up per block. It also requires
// replacement when the planned block is unknown, as the built-in RequiresReplace does, because the
// framework skips nested plan modifiers on an unknown object and the change can't be checked yet.
// An rps change leaves both blocks set and is updated in place. The framework skips this on create
// and destroy.
func callerRateLimitRuleTypeChanged(ctx context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = req.PlanValue.IsUnknown() || req.StateValue.IsNull() != req.PlanValue.IsNull()
}

func callerRateLimitRpsAttribute() schema.Float64Attribute {
	return schema.Float64Attribute{
		Description: "The maximum requests per second allowed from the callers this rule applies to. A value of 0 blocks all traffic from them.",
		Required:    true,
		Validators: []validator.Float64{
			float64validator.AtLeast(0),
		},
	}
}

func (r *callerRateLimitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	ruleTypeChanged := objectplanmodifier.RequiresReplaceIf(
		callerRateLimitRuleTypeChanged,
		"Switching between namespace_default and namespace_limit requires replacement.",
		"Switching between `namespace_default` and `namespace_limit` requires replacement.",
	)

	resp.Schema = schema.Schema{
		Description: "Provisions a caller rate limit, which limits the rate at which other namespaces can call a namespace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the caller rate limit, formatted as `<namespace_id>/<caller_rate_limit_id>`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace_id": schema.StringAttribute{
				Description: "The target namespace being protected by this limit, formatted as `<namespace>.<account_id>`. Changing it replaces the resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"namespace_default": schema.SingleNestedAttribute{
				Description: "The default limit for calling namespaces that don't match a `namespace_limit` rule. The target namespace can have at most one. Exactly one of `namespace_default` or `namespace_limit` must be set, and switching between them replaces the resource.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"rps": callerRateLimitRpsAttribute(),
				},
				PlanModifiers: []planmodifier.Object{ruleTypeChanged},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("namespace_limit"),
					}...),
				},
			},
			"namespace_limit": schema.SingleNestedAttribute{
				Description: "A limit for one specific calling namespace. The target namespace can have at most one per calling namespace. Exactly one of `namespace_default` or `namespace_limit` must be set, and switching between them replaces the resource.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"namespace_id": schema.StringAttribute{
						Description: "The calling namespace this limit applies to, formatted as `<namespace>.<account_id>`. Changing it replaces the resource.",
						Required:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"rps": callerRateLimitRpsAttribute(),
				},
				PlanModifiers: []planmodifier.Object{ruleTypeChanged},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("namespace_default"),
					}...),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *callerRateLimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan callerRateLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	spec, d := getCallerRateLimitSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespaceID := plan.NamespaceID.ValueString()
	svcResp, err := r.client.CloudService().CreateCallerRateLimit(ctx, &cloudservicev1.CreateCallerRateLimitRequest{
		Namespace:        namespaceID,
		Spec:             spec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create caller rate limit", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get caller rate limit creation status", err.Error())
		return
	}

	rl, err := r.client.CloudService().GetCallerRateLimit(ctx, &cloudservicev1.GetCallerRateLimitRequest{
		Namespace:         namespaceID,
		CallerRateLimitId: svcResp.GetCallerRateLimitId(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get caller rate limit", err.Error())
		return
	}

	resp.Diagnostics.Append(updateCallerRateLimitModelFromSpec(ctx, &plan, rl.GetCallerRateLimit(), namespaceID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *callerRateLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state callerRateLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// After import only id is set, so the namespace always comes from the id.
	namespaceID, callerRateLimitID, err := parseCallerRateLimitID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid caller rate limit id", err.Error())
		return
	}

	rl, err := r.client.CloudService().GetCallerRateLimit(ctx, &cloudservicev1.GetCallerRateLimitRequest{
		Namespace:         namespaceID,
		CallerRateLimitId: callerRateLimitID,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Caller Rate Limit Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Failed to get caller rate limit", err.Error())
		return
	}

	resp.Diagnostics.Append(updateCallerRateLimitModelFromSpec(ctx, &state, rl.GetCallerRateLimit(), namespaceID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update only changes rps; every other change replaces the resource.
func (r *callerRateLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state callerRateLimitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	spec, d := getCallerRateLimitSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespaceID, callerRateLimitID, err := parseCallerRateLimitID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid caller rate limit id", err.Error())
		return
	}

	currentRateLimit, err := r.client.CloudService().GetCallerRateLimit(ctx, &cloudservicev1.GetCallerRateLimitRequest{
		Namespace:         namespaceID,
		CallerRateLimitId: callerRateLimitID,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get caller rate limit", err.Error())
		return
	}

	svcResp, err := r.client.CloudService().UpdateCallerRateLimit(ctx, &cloudservicev1.UpdateCallerRateLimitRequest{
		Namespace:         namespaceID,
		CallerRateLimitId: callerRateLimitID,
		Spec:              spec,
		ResourceVersion:   currentRateLimit.GetCallerRateLimit().GetResourceVersion(),
		AsyncOperationId:  uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update caller rate limit", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get caller rate limit update status", err.Error())
		return
	}

	rl, err := r.client.CloudService().GetCallerRateLimit(ctx, &cloudservicev1.GetCallerRateLimitRequest{
		Namespace:         namespaceID,
		CallerRateLimitId: callerRateLimitID,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get caller rate limit", err.Error())
		return
	}

	resp.Diagnostics.Append(updateCallerRateLimitModelFromSpec(ctx, &plan, rl.GetCallerRateLimit(), namespaceID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *callerRateLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state callerRateLimitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespaceID, callerRateLimitID, err := parseCallerRateLimitID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid caller rate limit id", err.Error())
		return
	}

	currentRateLimit, err := r.client.CloudService().GetCallerRateLimit(ctx, &cloudservicev1.GetCallerRateLimitRequest{
		Namespace:         namespaceID,
		CallerRateLimitId: callerRateLimitID,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Caller Rate Limit Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get caller rate limit", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteCallerRateLimit(ctx, &cloudservicev1.DeleteCallerRateLimitRequest{
		Namespace:         namespaceID,
		CallerRateLimitId: callerRateLimitID,
		ResourceVersion:   currentRateLimit.GetCallerRateLimit().GetResourceVersion(),
		AsyncOperationId:  uuid.New().String(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Caller Rate Limit Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete caller rate limit", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get caller rate limit deletion status", err.Error())
		return
	}
}

func (r *callerRateLimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getCallerRateLimitSpecFromModel(ctx context.Context, plan *callerRateLimitResourceModel) (*namespacev1.CallerRateLimitSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The schema validators should prevent these, but guard against building an ambiguous spec.
	if plan.NamespaceDefault.IsNull() && plan.NamespaceLimit.IsNull() {
		diags.AddError("Missing caller rate limit rule", "Exactly one of namespace_default or namespace_limit must be configured")
		return nil, diags
	}
	if !plan.NamespaceDefault.IsNull() && !plan.NamespaceLimit.IsNull() {
		diags.AddError("Multiple caller rate limit rules configured", "Only one of namespace_default or namespace_limit can be configured")
		return nil, diags
	}

	if !plan.NamespaceDefault.IsNull() {
		var namespaceDefault callerRateLimitNamespaceDefaultModel
		diags.Append(plan.NamespaceDefault.As(ctx, &namespaceDefault, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		return &namespacev1.CallerRateLimitSpec{
			Rule: &namespacev1.CallerRateLimitSpec_NamespaceDefault{
				NamespaceDefault: &namespacev1.NamespaceDefaultCallerRateLimit{
					Rps: namespaceDefault.Rps.ValueFloat64(),
				},
			},
		}, diags
	}

	var namespaceLimit callerRateLimitNamespaceLimitModel
	diags.Append(plan.NamespaceLimit.As(ctx, &namespaceLimit, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	return &namespacev1.CallerRateLimitSpec{
		Rule: &namespacev1.CallerRateLimitSpec_NamespaceLimit{
			NamespaceLimit: &namespacev1.NamespaceCallerRateLimit{
				NamespaceId: namespaceLimit.NamespaceID.ValueString(),
				Rps:         namespaceLimit.Rps.ValueFloat64(),
			},
		},
	}, diags
}

// updateCallerRateLimitModelFromSpec writes rl into state. The caller supplies the target namespace
// (the plan in Create, the parsed id in Read and Update) rather than trusting the response. State is
// only written once everything has converted, so an error leaves it as it was.
func updateCallerRateLimitModelFromSpec(ctx context.Context, state *callerRateLimitResourceModel, rl *namespacev1.CallerRateLimit, namespaceID string) diag.Diagnostics {
	var diags diag.Diagnostics

	if rl.GetId() == "" {
		diags.AddError("Invalid caller rate limit", fmt.Sprintf("Caller rate limit in namespace %q has no id. Please report this issue to the provider developers.", namespaceID))
		return diags
	}

	namespaceDefaultObj := types.ObjectNull(callerRateLimitNamespaceDefaultAttrTypes)
	namespaceLimitObj := types.ObjectNull(callerRateLimitNamespaceLimitAttrTypes)

	spec := rl.GetSpec()
	switch {
	case spec.GetNamespaceDefault() != nil:
		var d diag.Diagnostics
		namespaceDefaultObj, d = types.ObjectValueFrom(ctx, callerRateLimitNamespaceDefaultAttrTypes, callerRateLimitNamespaceDefaultModel{
			Rps: types.Float64Value(spec.GetNamespaceDefault().GetRps()),
		})
		diags.Append(d...)
	case spec.GetNamespaceLimit() != nil:
		var d diag.Diagnostics
		namespaceLimitObj, d = types.ObjectValueFrom(ctx, callerRateLimitNamespaceLimitAttrTypes, callerRateLimitNamespaceLimitModel{
			NamespaceID: types.StringValue(spec.GetNamespaceLimit().GetNamespaceId()),
			Rps:         types.Float64Value(spec.GetNamespaceLimit().GetRps()),
		})
		diags.Append(d...)
	default:
		diags.AddError(
			"Unsupported caller rate limit rule",
			fmt.Sprintf("Caller rate limit %q has no rule this provider version recognizes. Please report this issue to the provider developers.", rl.GetId()),
		)
	}
	if diags.HasError() {
		return diags
	}

	state.ID = types.StringValue(callerRateLimitID(namespaceID, rl.GetId()))
	state.NamespaceID = types.StringValue(namespaceID)
	state.NamespaceDefault = namespaceDefaultObj
	state.NamespaceLimit = namespaceLimitObj

	return diags
}

func callerRateLimitID(namespaceID, callerRateLimitID string) string {
	return namespaceID + "/" + callerRateLimitID
}

func parseCallerRateLimitID(id string) (namespaceID, callerRateLimitID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid caller rate limit id %q: expected format `<namespace_id>/<caller_rate_limit_id>`, e.g. `my-ns.abc12/<caller_rate_limit_id>`", id)
	}

	return parts[0], parts[1], nil
}
