package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	"go.temporal.io/cloud-sdk/api/cloudservice/v1"
	connectivityrulev1 "go.temporal.io/cloud-sdk/api/connectivityrule/v1"
)

var (
	_ datasource.DataSource              = &connectivityRuleDataSource{}
	_ datasource.DataSourceWithConfigure = &connectivityRuleDataSource{}
)

func NewConnectivityRuleDataSource() datasource.DataSource {
	return &connectivityRuleDataSource{}
}

type (
	connectivityRuleDataSource struct {
		client *client.Client
	}

	connectivityRuleDataModel struct {
		ID               types.String `tfsdk:"id"`
		ConnectivityType types.String `tfsdk:"connectivity_type"`
		ConnectionID     types.String `tfsdk:"connection_id"`
		Region           types.String `tfsdk:"region"`
		GcpProjectID     types.String `tfsdk:"gcp_project_id"`

		State     types.String `tfsdk:"state"`
		CreatedAt types.String `tfsdk:"created_at"`
	}
)

func connectivityRuleDataSourceSchema(idRequired bool) map[string]schema.Attribute {
	idAttribute := schema.StringAttribute{
		Description: "The unique identifier of the connectivity rule across all Temporal Cloud tenants.",
	}

	switch idRequired {
	case true:
		idAttribute.Required = true
	case false:
		idAttribute.Computed = true
	}

	return map[string]schema.Attribute{
		"id": idAttribute,
		"connectivity_type": schema.StringAttribute{
			Computed:    true,
			Description: "The type of connectivity.",
		},
		"connection_id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of the connection to the connectivity rule.",
		},
		"region": schema.StringAttribute{
			Computed:    true,
			Description: "The region of the connectivity rule.",
		},
		"gcp_project_id": schema.StringAttribute{
			Computed:    true,
			Description: "The GCP project ID of the connectivity rule.",
		},
		"state": schema.StringAttribute{
			Computed:    true,
			Description: "The current state of the connectivity rule.",
		},
		"created_at": schema.StringAttribute{
			Computed:    true,
			Description: "The time the connectivity rule was created.",
		},
	}
}

func (d *connectivityRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = client
}

func (d *connectivityRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connectivity_rule"
}

func (d *connectivityRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about a connectivity rule.",
		Attributes:  connectivityRuleDataSourceSchema(true),
	}
}

func (d *connectivityRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input connectivityRuleDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.ID.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid connectivity rule id", "connectivity rule id is required")
		return
	}

	crResp, err := d.client.CloudService().GetConnectivityRule(ctx, &cloudservice.GetConnectivityRuleRequest{
		ConnectivityRuleId: input.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get connectivity rule", err.Error())
		return
	}

	model, diags := connectivityRuleToConnectivityRuleDataModel(ctx, crResp.GetConnectivityRule())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
}

func connectivityRuleToConnectivityRuleDataModel(ctx context.Context, connectivityRule *connectivityrulev1.ConnectivityRule) (*connectivityRuleDataModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	stateStr, err := enums.FromResourceState(connectivityRule.State)
	if err != nil {
		diags.AddError("Unable to convert connectivity rule state", err.Error())
		return nil, diags
	}

	model := new(connectivityRuleDataModel)
	model.ID = types.StringValue(connectivityRule.GetId())
	model.State = types.StringValue(stateStr)
	model.CreatedAt = types.StringValue(connectivityRule.GetCreatedTime().AsTime().Format(time.RFC3339))

	if connectivityRule.GetSpec().GetPrivateRule() != nil {
		model.ConnectivityType = types.StringValue(connectivityRuleTypePrivate)
		model.ConnectionID = types.StringValue(connectivityRule.GetSpec().GetPrivateRule().GetConnectionId())
		model.Region = types.StringValue(connectivityRule.GetSpec().GetPrivateRule().GetRegion())
		if connectivityRule.GetSpec().GetPrivateRule().GetGcpProjectId() != "" {
			model.GcpProjectID = types.StringValue(connectivityRule.GetSpec().GetPrivateRule().GetGcpProjectId())
		} else {
			model.GcpProjectID = types.StringNull()
		}
	} else if connectivityRule.GetSpec().GetPublicRule() != nil {
		model.ConnectivityType = types.StringValue(connectivityRuleTypePublic)
		model.ConnectionID = types.StringNull()
		model.Region = types.StringNull()
		model.GcpProjectID = types.StringNull()
	} else {
		diags.AddError("Invalid connectivity rule", "connectivity rule must be either public or private")
		return nil, diags
	}
	return model, diags
}
