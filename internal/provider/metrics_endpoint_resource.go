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
	"github.com/temporalio/tcld/protogen/api/account/v1"
	"github.com/temporalio/tcld/protogen/api/accountservice/v1"
	"github.com/temporalio/tcld/protogen/api/requestservice/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

type (
	metricsEndpointResource struct {
		accountClient accountservice.AccountServiceClient
		requestClient requestservice.RequestServiceClient
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

	clientStore, ok := req.ProviderData.(*client.ClientStore)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.ClientStore, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.accountClient = clientStore.AccountServiceClient()
	r.requestClient = clientStore.RequestServiceClient()
}

// Metadata returns the resource type name.
func (r *metricsEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_metrics_endpoint"
}

// Schema defines the schema for the resource.
func (r *metricsEndpointResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures a Temporal Cloud account's metrics",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "A unique identifier for the account's metrics configuration",
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
				ctx, timeouts.Opts{
					Create: true,
					Delete: true,
					Update: true,
				},
			),
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
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

	accResp, err := r.accountClient.GetAccount(ctx, &accountservice.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account information", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// create just flips "enabled" to true
	metricsReq := &accountservice.UpdateAccountRequest{
		ResourceVersion: accResp.GetAccount().GetResourceVersion(),
		Spec: &account.AccountSpec{
			Metrics: &account.MetricsSpec{
				Enabled:          true,
				AcceptedClientCa: plan.AcceptedClientCA.ValueString(),
			},
			OutputSinks: accResp.GetAccount().GetSpec().GetOutputSinks(),
		},
	}

	metricsResp, err := r.accountClient.UpdateAccount(ctx, metricsReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create metrics endpoint resource", err.Error())
		return
	}

	if err := client.AwaitRequestStatus(ctx, r.requestClient, metricsResp.GetRequestStatus()); err != nil {
		resp.Diagnostics.AddError("Failed to create metrics endpoint resource", err.Error())
		return
	}

	accResp, err = r.accountClient.GetAccount(ctx, &accountservice.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get resource state", err.Error())
		return
	}

	updateAccountMetricsModelFromSpec(&plan, accResp.GetAccount())
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *metricsEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state metricsEndpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	accResp, err := r.accountClient.GetAccount(ctx, &accountservice.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get metrics endpoint", err.Error())
		return
	}

	updateAccountMetricsModelFromSpec(&state, accResp.GetAccount())
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *metricsEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan metricsEndpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Create(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accResp, err := r.accountClient.GetAccount(ctx, &accountservice.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current metrics endpoint status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	metricsReq := &accountservice.UpdateAccountRequest{
		ResourceVersion: accResp.GetAccount().GetResourceVersion(),
		Spec: &account.AccountSpec{
			Metrics: &account.MetricsSpec{
				Enabled:          accResp.GetAccount().GetSpec().GetMetrics().GetEnabled(),
				AcceptedClientCa: plan.AcceptedClientCA.ValueString(),
			},
			OutputSinks: accResp.GetAccount().GetSpec().GetOutputSinks(),
		},
	}

	metricsResp, err := r.accountClient.UpdateAccount(ctx, metricsReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update metrics endpoint endpoint", err.Error())
		return
	}

	if err := client.AwaitRequestStatus(ctx, r.requestClient, metricsResp.GetRequestStatus()); err != nil {
		resp.Diagnostics.AddError("Failed to update metrics endpoint endpoint", err.Error())
		return
	}

	accResp, err = r.accountClient.GetAccount(ctx, &accountservice.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get metrics endpoint after update", err.Error())
		return
	}

	updateAccountMetricsModelFromSpec(&plan, accResp.GetAccount())
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

	accResp, err := r.accountClient.GetAccount(ctx, &accountservice.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current metrics endpoint status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	// can't actually "delete" account metrics config, setting to disabled is the best equivalent
	metricsReq := &accountservice.UpdateAccountRequest{
		ResourceVersion: accResp.GetAccount().GetResourceVersion(),
		Spec: &account.AccountSpec{
			Metrics: &account.MetricsSpec{
				Enabled: false,
			},
			OutputSinks: accResp.GetAccount().GetSpec().GetOutputSinks(),
		},
	}

	metricsResp, err := r.accountClient.UpdateAccount(ctx, metricsReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete metrics endpoint resource", err.Error())
		return
	}

	if err := client.AwaitRequestStatus(ctx, r.requestClient, metricsResp.GetRequestStatus()); err != nil {
		resp.Diagnostics.AddError("Failed to delete metrics endpoint resource", err.Error())
		return
	}
}

func (r *metricsEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updateAccountMetricsModelFromSpec(state *metricsEndpointResourceModel, spec *account.Account) {
	state.AcceptedClientCA = types.StringValue(spec.GetSpec().GetMetrics().GetAcceptedClientCa())
	state.Uri = types.StringValue(spec.GetMetrics().GetUri())
	state.ID = types.StringValue("account-metrics") // no real ID to key off of here other than account ID, which is hard to get via the API
}
