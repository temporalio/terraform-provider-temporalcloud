package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	accountv1 "go.temporal.io/api/cloud/account/v1"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

type (
	metricsEndpointResource struct {
		client *client.Client
	}

	metricsEndpointResourceModel struct {
		ID               types.String `tfsdk:"id"`
		AcceptedClientCA types.String `tfsdk:"accepted_client_ca"`
		Uri              types.String `tfsdk:"uri"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*metricsEndpointResource)(nil)
	_ resource.ResourceWithConfigure   = (*metricsEndpointResource)(nil)
	_ resource.ResourceWithImportState = (*metricsEndpointResource)(nil)
)

func NewMetricsEndpointResource() resource.Resource {
	return &metricsEndpointResource{}
}

func (r *metricsEndpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	cli, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = cli

	return
}

// Metadata returns the resource type name.
func (r *metricsEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_metrics_endpoint"
}

// Schema defines the schema for the resource
func (r *metricsEndpointResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures a Temporal Cloud account's metrics",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "A unique identifier for the account's metrics configuration. Always `account-ACCOUNT_ID-metrics`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"accepted_client_ca": schema.StringAttribute{
				Description: "The Base64-encoded CA cert in PEM format used to authenticate clients connecting to the metrics endpoint.",
				Required:    true,
			},
			"uri": schema.StringAttribute{
				Description: "The Prometheus metrics endpoint URI",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(
				ctx,
				timeouts.Opts{
					Create: true,
					Delete: true,
					Update: true,
				},
			),
		},
	}
}

func (r *metricsEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan metricsEndpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accResp, err := r.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account information.", err.Error())
		return
	}

	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// create just enables the metrics endpoint by providing a CA certificate
	metricsReq := &cloudservicev1.UpdateAccountRequest{
		ResourceVersion: accResp.GetAccount().GetResourceVersion(),
		Spec: &accountv1.AccountSpec{
			Metrics: &accountv1.MetricsSpec{
				AcceptedClientCa: []byte(plan.AcceptedClientCA.ValueString()),
			},
		},
	}

	metricsResp, err := r.client.CloudService().UpdateAccount(createCtx, metricsReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create metrics endpoint resource.", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(createCtx, r.client.CloudOperationsClient, metricsResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to create metrics endpoint resource.", err.Error())
		return
	}

	accResp, err = r.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get resource state after create.", err.Error())
		return
	}

	updateMetricsEndpointModelFromSpec(&plan, accResp.GetAccount())
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *metricsEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state metricsEndpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	accResp, err := r.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get metrics endpoint", err.Error())
		return
	}

	updateMetricsEndpointModelFromSpec(&state, accResp.GetAccount())
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *metricsEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan metricsEndpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accResp, err := r.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account information.", err.Error())
		return
	}

	updateCtx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	metricsReq := &cloudservicev1.UpdateAccountRequest{
		ResourceVersion: accResp.GetAccount().GetResourceVersion(),
		Spec: &accountv1.AccountSpec{
			Metrics: &accountv1.MetricsSpec{
				AcceptedClientCa: []byte(plan.AcceptedClientCA.ValueString()),
			},
		},
	}

	metricsResp, err := r.client.CloudService().UpdateAccount(updateCtx, metricsReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update metrics endpoint resource.", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(updateCtx, r.client.CloudOperationsClient, metricsResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update metrics endpoint resource.", err.Error())
		return
	}

	accResp, err = r.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get resource state after update.", err.Error())
		return
	}

	updateMetricsEndpointModelFromSpec(&plan, accResp.GetAccount())
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *metricsEndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state metricsEndpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accResp, err := r.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current metrics endpoint status", err.Error())
		return
	}

	deleteCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// can't actually "delete" account metrics config, removing the CA cert is the best equivalent
	metricsReq := &cloudservicev1.UpdateAccountRequest{
		ResourceVersion: accResp.GetAccount().GetResourceVersion(),
		Spec: &accountv1.AccountSpec{
			Metrics: &accountv1.MetricsSpec{},
		},
	}

	metricsResp, err := r.client.CloudService().UpdateAccount(deleteCtx, metricsReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete metrics endpoint resource", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(deleteCtx, r.client.CloudOperationsClient, metricsResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to delete metrics endpoint resource", err.Error())
		return
	}
}

func (r *metricsEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updateMetricsEndpointModelFromSpec(state *metricsEndpointResourceModel, spec *accountv1.Account) {
	state.AcceptedClientCA = types.StringValue(string(spec.GetSpec().GetMetrics().GetAcceptedClientCa()))
	state.Uri = types.StringValue(spec.GetMetrics().GetUri())
	state.ID = types.StringValue(fmt.Sprintf("account-%s-metrics", spec.GetId()))
}
