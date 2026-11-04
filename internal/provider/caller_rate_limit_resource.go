package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
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
)

func NewCallerRateLimitResource() resource.Resource {
	return &callerRateLimitResource{}
}

var (
	_ resource.Resource              = (*callerRateLimitResource)(nil)
	_ resource.ResourceWithConfigure = (*callerRateLimitResource)(nil)
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
	resp.Diagnostics.AddError("Not implemented", "caller rate limit CRUD is not implemented yet")
}

func (r *callerRateLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.AddError("Not implemented", "caller rate limit CRUD is not implemented yet")
}

func (r *callerRateLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Not implemented", "caller rate limit CRUD is not implemented yet")
}

func (r *callerRateLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError("Not implemented", "caller rate limit CRUD is not implemented yet")
}
