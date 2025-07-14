package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	nexusv1 "go.temporal.io/cloud-sdk/api/nexus/v1"
)

type (
	nexusEndpointsDataSource struct {
		client *client.Client
	}

	nexusEndpointsDataModel struct {
		ID             types.String             `tfsdk:"id"`
		NexusEndpoints []nexusEndpointDataModel `tfsdk:"nexus_endpoints"`
	}
)

var (
	_ datasource.DataSource              = (*nexusEndpointsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*nexusEndpointsDataSource)(nil)
)

func NewNexusEndpointsDataSource() datasource.DataSource {
	return &nexusEndpointsDataSource{}
}

func (d *nexusEndpointsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nexus_endpoints"
}

func (d *nexusEndpointsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *nexusEndpointsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about all Nexus Endpoints.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Nexus Endpoints data source.",
				Computed:    true,
			},
			"nexus_endpoints": schema.ListNestedAttribute{
				Description: "The list of Nexus Endpoints.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: nexusEndpointSchema(false),
				},
			},
		},
	}
}

func (d *nexusEndpointsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state nexusEndpointsDataModel

	var nexusEndpoints []*nexusv1.Endpoint
	pageToken := ""
	for {
		r, err := d.client.CloudService().GetNexusEndpoints(ctx, &cloudservicev1.GetNexusEndpointsRequest{PageToken: pageToken})
		if err != nil {
			resp.Diagnostics.AddError("Unable to fetch nexus endpoints", err.Error())
			return
		}

		nexusEndpoints = append(nexusEndpoints, r.GetEndpoints()...)

		if r.GetNextPageToken() == "" {
			break
		}

		pageToken = r.GetNextPageToken()
	}

	for _, endpoint := range nexusEndpoints {
		nexusEndpointModel, diags := nexusEndpointToNexusEndpointDataModel(ctx, endpoint)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		state.NexusEndpoints = append(state.NexusEndpoints, *nexusEndpointModel)
	}

	accResp, err := d.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account information.", err.Error())
		return
	}

	// Silly, but temporarily necessary:
	// https://developer.hashicorp.com/terraform/plugin/framework/acctests#no-id-found-in-attributes
	state.ID = types.StringValue(fmt.Sprintf("account-%s-nexus-endpoints", accResp.GetAccount().GetId()))
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
