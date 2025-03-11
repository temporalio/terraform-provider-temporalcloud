package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type (
	serviceAccountDataSource struct {
		client *client.Client
	}
)

var (
	_ datasource.DataSource              = (*serviceAccountDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serviceAccountDataSource)(nil)
)

func NewServiceAccountDataSource() datasource.DataSource {
	return &serviceAccountDataSource{}
}

func (d *serviceAccountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (d *serviceAccountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			"Expected *client.Client, got: %T. Please report this issue to the provider developers.",
		)
		return
	}

	d.client = client
}

func (d *serviceAccountDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about a Service Account.",
		Attributes:  serviceAccountSchema(true),
	}
}

func (d *serviceAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input serviceAccountDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.ID.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid service account id", "service account id is required")
		return
	}

	saResp, err := d.client.CloudService().GetServiceAccount(ctx, &cloudservicev1.GetServiceAccountRequest{
		ServiceAccountId: input.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch service account", err.Error())
		return
	}

	saDataModel, diags := serviceAccountToServiceAccountDataModel(ctx, saResp.GetServiceAccount())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &saDataModel)
	resp.Diagnostics.Append(diags...)
}
