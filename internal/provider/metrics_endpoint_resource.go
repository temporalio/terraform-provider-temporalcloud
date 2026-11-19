package provider

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	accountv1 "go.temporal.io/cloud-sdk/api/account/v1"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

type (
	metricsEndpointResource struct {
		client *client.Client
	}

	metricsEndpointResourceModel struct {
		ID               types.String                 `tfsdk:"id"`
		AcceptedClientCA internaltypes.EncodedCAValue `tfsdk:"accepted_client_ca"`
		Uri              types.String                 `tfsdk:"uri"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*metricsEndpointResource)(nil)
	_ resource.ResourceWithConfigure   = (*metricsEndpointResource)(nil)
	_ resource.ResourceWithImportState = (*metricsEndpointResource)(nil)
)

const metricsEndpointDeprecationMessage = "The PromQL metrics endpoint was disabled on October 5, 2026, and the " +
	"temporalcloud_metrics_endpoint resource is deprecated. " +
	"Remove this resource from your configuration and run `terraform apply` to clear the existing endpoint configuration. " +
	"Use the OpenMetrics endpoint instead: https://docs.temporal.io/cloud/metrics/openmetrics"

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
}

// Metadata returns the resource type name.
func (r *metricsEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_metrics_endpoint"
}

// Schema defines the schema for the resource.
func (r *metricsEndpointResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures a Temporal Cloud account's metrics",
		MarkdownDescription: "Configures a Temporal Cloud account's metrics endpoint for the PromQL query API.\n\n" +
			"!> **Deprecated.** The PromQL metrics endpoint was deprecated on April 2, 2026 and disabled on October 5, 2026. " +
			"This resource can no longer be created or updated. " +
			"Remove this resource from your configuration and run `terraform apply` to clear the existing endpoint configuration. " +
			"Use the [OpenMetrics endpoint](https://docs.temporal.io/cloud/metrics/openmetrics) instead. " +
			"See the [`temporalcloud_service_account`](https://registry.terraform.io/providers/temporalio/temporalcloud/latest/docs/resources/service_account) " +
			"resource for an example of setting up OpenMetrics with Terraform.",
		DeprecationMessage: metricsEndpointDeprecationMessage,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "A unique identifier for the account's metrics configuration. Always `account-ACCOUNT_ID-metrics`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"accepted_client_ca": schema.StringAttribute{
				CustomType:  internaltypes.EncodedCAType{},
				Description: "The Base64-encoded CA cert in PEM format used to authenticate clients connecting to the metrics endpoint.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
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

func (r *metricsEndpointResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"The PromQL metrics endpoint is no longer supported.",
		metricsEndpointDeprecationMessage,
	)
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

	if accResp.GetAccount().GetMetrics().GetUri() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	updateMetricsEndpointModelFromSpec(&state, accResp.GetAccount())
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *metricsEndpointResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"The PromQL metrics endpoint is no longer supported.",
		metricsEndpointDeprecationMessage,
	)
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

	// can't actually "delete" account metrics config, removing the CA cert is the best equivalent
	metricsReq := &cloudservicev1.UpdateAccountRequest{
		ResourceVersion: accResp.GetAccount().GetResourceVersion(),
		Spec: &accountv1.AccountSpec{
			Metrics: nil,
		},
		AsyncOperationId: uuid.New().String(),
	}

	deleteCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	metricsResp, err := r.client.CloudService().UpdateAccount(deleteCtx, metricsReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete metrics endpoint resource", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(deleteCtx, r.client, metricsResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to delete metrics endpoint resource", err.Error())
		return
	}
}

func (r *metricsEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updateMetricsEndpointModelFromSpec(state *metricsEndpointResourceModel, spec *accountv1.Account) {
	state.AcceptedClientCA = internaltypes.EncodedCA(base64.StdEncoding.EncodeToString(spec.GetSpec().GetMetrics().GetAcceptedClientCa()))
	state.Uri = types.StringValue(spec.GetMetrics().GetUri())
	state.ID = types.StringValue(fmt.Sprintf("account-%s-metrics", spec.GetId()))
}
