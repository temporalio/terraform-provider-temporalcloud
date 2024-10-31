package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
)

var (
	_ datasource.DataSource              = &regionsDataSource{}
	_ datasource.DataSourceWithConfigure = &regionsDataSource{}
)

// NewCoffeesDataSource is a helper function to simplify the provider implementation.
func NewRegionsDataSource() datasource.DataSource {
	return &regionsDataSource{}
}

type (
	regionsDataSource struct {
		client *client.Client
	}

	regionsDataModel struct {
		// https://developer.hashicorp.com/terraform/plugin/framework/acctests#no-id-found-in-attributes
		ID      types.String      `tfsdk:"id"`
		Regions []regionDataModel `tfsdk:"regions"`
	}

	regionDataModel struct {
		ID                  types.String `tfsdk:"id"`
		CloudProvider       types.String `tfsdk:"cloud_provider"`
		CloudProviderRegion types.String `tfsdk:"cloud_provider_region"`
		Location            types.String `tfsdk:"location"`
	}
)

// Configure adds the provider configured client to the data source.
func (d *regionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Metadata returns the data source type name.
func (d *regionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_regions"
}

// Schema defines the schema for the data source.
func (d *regionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"regions": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "The unique identifier for the region, e.g. `aws-us-east-1`.",
							Computed:    true,
						},
						"cloud_provider": schema.StringAttribute{
							Description: "The name of the Cloud provider for this region, e.g. `aws`.",
							Computed:    true,
						},
						"cloud_provider_region": schema.StringAttribute{
							Description: "The name of the region within the Cloud provider, e.g. `us-east-1`.",
							Computed:    true,
						},
						"location": schema.StringAttribute{
							Description: "The physical location of the region, e.g. \"US East (N. Virginia)\".",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *regionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state regionsDataModel

	regions, err := d.client.CloudService().GetRegions(ctx, &cloudservicev1.GetRegionsRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch regions", err.Error())
		return
	}

	for _, region := range regions.GetRegions() {
		provider, err := enums.FromRegionCloudProvider(region.GetCloudProvider())
		if err != nil {
			resp.Diagnostics.AddError("Unable to convert region cloud provider", err.Error())
			return
		}
		regionModel := regionDataModel{
			ID:                  types.StringValue(region.GetId()),
			CloudProvider:       types.StringValue(provider),
			CloudProviderRegion: types.StringValue(region.GetCloudProviderRegion()),
			Location:            types.StringValue(region.GetLocation()),
		}

		state.Regions = append(state.Regions, regionModel)
	}

	// Silly, but temporarily necessary:
	// https://developer.hashicorp.com/terraform/plugin/framework/acctests#no-id-found-in-attributes
	state.ID = types.StringValue("terraform")
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
