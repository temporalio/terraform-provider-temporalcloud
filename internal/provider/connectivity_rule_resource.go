package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	connectivityrulev1 "go.temporal.io/cloud-sdk/api/connectivityrule/v1"
)

const (
	connectivityRuleTypePublic  = "public"
	connectivityRuleTypePrivate = "private"
)

type (
	connectivityRuleResource struct {
		client *client.Client
	}

	connectivityRuleResourceModel struct {
		ID               types.String   `tfsdk:"id"`
		ConnectivityType types.String   `tfsdk:"connectivity_type"`
		ConnectionID     types.String   `tfsdk:"connection_id"`
		Region           types.String   `tfsdk:"region"`
		GcpProjectID     types.String   `tfsdk:"gcp_project_id"`
		Timeouts         timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*connectivityRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*connectivityRuleResource)(nil)
	_ resource.ResourceWithImportState = (*connectivityRuleResource)(nil)
)

func NewConnectivityRuleResource() resource.Resource {
	return &connectivityRuleResource{}
}

func (r *connectivityRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *connectivityRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connectivity_rule"
}

func (r *connectivityRuleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud Connectivity Rule.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Connectivity Rule.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connectivity_type": schema.StringAttribute{
				Description: "The type of connectivity. Must be one of 'public' or 'private'.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(connectivityRuleTypePublic, connectivityRuleTypePrivate),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"connection_id": schema.StringAttribute{
				Description: "The connection ID of the private connection.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"gcp_project_id": schema.StringAttribute{
				Description: "The GCP project ID. Required when cloud_provider is 'gcp'.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"region": schema.StringAttribute{
				Description: "The region of the connection. Example: 'aws-us-west-2'.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}

func (r *connectivityRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectivityRuleResourceModel
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

	spec, d := getConnectivityRuleSpecFromModel(&plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().CreateConnectivityRule(ctx, &cloudservicev1.CreateConnectivityRuleRequest{
		Spec:             spec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Connectivity Rule", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create Connectivity Rule", err.Error())
		return
	}

	connectivityRule, err := r.client.CloudService().GetConnectivityRule(ctx, &cloudservicev1.GetConnectivityRuleRequest{
		ConnectivityRuleId: svcResp.ConnectivityRuleId,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Connectivity Rule after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateConnectivityRuleModelFromSpec(&plan, connectivityRule.ConnectivityRule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)

}

func (r *connectivityRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Connectivity rules cannot be updated. To modify a connectivity rule, it must be destroyed and recreated.",
	)
}

func (r *connectivityRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectivityRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	connectivityRule, err := r.client.CloudService().GetConnectivityRule(ctx, &cloudservicev1.GetConnectivityRuleRequest{
		ConnectivityRuleId: state.ID.ValueString(),
	})

	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Connectivity Rule Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get Connectivity Rule", err.Error())
		return
	}

	resp.Diagnostics.Append(updateConnectivityRuleModelFromSpec(&state, connectivityRule.ConnectivityRule)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *connectivityRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectivityRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	currentConnectivityRule, err := r.client.CloudService().GetConnectivityRule(ctx, &cloudservicev1.GetConnectivityRuleRequest{
		ConnectivityRuleId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Connectivity Rule Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to get Connectivity Rule", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteConnectivityRule(ctx, &cloudservicev1.DeleteConnectivityRuleRequest{
		ConnectivityRuleId: state.ID.ValueString(),
		ResourceVersion:    currentConnectivityRule.ConnectivityRule.GetResourceVersion(),
		AsyncOperationId:   uuid.New().String(),
	})

	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Connectivity Rule Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to delete Connectivity Rule", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete Connectivity Rule", err.Error())
		return
	}
}

func (r *connectivityRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getConnectivityRuleSpecFromModel(model *connectivityRuleResourceModel) (*connectivityrulev1.ConnectivityRuleSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch model.ConnectivityType.ValueString() {
	case connectivityRuleTypePublic:
		return &connectivityrulev1.ConnectivityRuleSpec{
			ConnectionType: &connectivityrulev1.ConnectivityRuleSpec_PublicRule{
				PublicRule: &connectivityrulev1.PublicConnectivityRule{},
			},
		}, diags

	case connectivityRuleTypePrivate:
		if model.ConnectionID.IsNull() {
			diags.AddError("Connection ID is required", "connection_id must be specified when connectivity_type is 'private'")
			return nil, diags
		}

		if model.Region.IsNull() {
			diags.AddError("Region is required", "region must be specified when connectivity_type is 'private'")
			return nil, diags
		}

		if strings.HasPrefix(model.Region.ValueString(), "gcp") && model.GcpProjectID.IsNull() {
			diags.AddError("GCP Project ID is required", "gcp_project_id must be specified when region is gcp")
			return nil, diags
		}

		privateRule := &connectivityrulev1.PrivateConnectivityRule{
			ConnectionId: model.ConnectionID.ValueString(),
			Region:       model.Region.ValueString(),
			GcpProjectId: model.GcpProjectID.ValueString(),
		}
		return &connectivityrulev1.ConnectivityRuleSpec{
			ConnectionType: &connectivityrulev1.ConnectivityRuleSpec_PrivateRule{
				PrivateRule: privateRule,
			},
		}, diags

	default:
		diags.AddError("Invalid connectivity type", fmt.Sprintf("connectivity_type must be 'public' or 'private', got: %s", model.ConnectivityType.ValueString()))
		return nil, diags
	}
}

func updateConnectivityRuleModelFromSpec(model *connectivityRuleResourceModel, connectivityRule *connectivityrulev1.ConnectivityRule) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(connectivityRule.GetId())

	if connectivityRule.Spec.GetPrivateRule() != nil {
		model.ConnectivityType = types.StringValue(connectivityRuleTypePrivate)
		model.ConnectionID = types.StringValue(connectivityRule.GetSpec().GetPrivateRule().GetConnectionId())
		model.Region = types.StringValue(connectivityRule.Spec.GetPrivateRule().GetRegion())
		model.GcpProjectID = types.StringValue(connectivityRule.Spec.GetPrivateRule().GetGcpProjectId())
	} else if connectivityRule.Spec.GetPublicRule() != nil {
		model.ConnectivityType = types.StringValue(connectivityRuleTypePublic)
		model.ConnectionID = types.StringValue("")
		model.Region = types.StringValue("")
		model.GcpProjectID = types.StringValue("")
	} else {
		diags.AddError("Invalid connectivity rule", "connectivity rule must be either public or private")
		return diags
	}

	return diags
}
