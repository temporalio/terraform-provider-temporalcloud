package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type (
	nexusEndpointDataSource struct {
		client *client.Client
	}
)

var (
	_ datasource.DataSource              = (*nexusEndpointDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*nexusEndpointDataSource)(nil)
)

func NewNexusEndpointDataSource() datasource.DataSource {
	return &nexusEndpointDataSource{}
}

func (d *nexusEndpointDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nexus_endpoint"
}

func (d *nexusEndpointDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cloudClient, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			"Expected *client.Client, got: %T. Please report this issue to the provider developers.",
		)
		return
	}

	d.client = cloudClient
}

func (d *nexusEndpointDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about a Nexus Endpoint.",
		Attributes:  nexusEndpointSchema(true),
	}
}

func (d *nexusEndpointDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input nexusEndpointDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.ID.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid nexus endpoint id", "nexus endpoint id is required")
		return
	}

	getResp, err := d.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: input.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch nexus endpoint", err.Error())
		return
	}

	endpointDataModel, diags := nexusEndpointToNexusEndpointDataModel(ctx, getResp.GetEndpoint())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &endpointDataModel)
	resp.Diagnostics.Append(diags...)
}
