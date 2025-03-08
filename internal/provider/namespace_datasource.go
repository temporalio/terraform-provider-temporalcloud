package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
)

var (
	_ datasource.DataSource              = &namespaceDataSource{}
	_ datasource.DataSourceWithConfigure = &namespaceDataSource{}
)

func NewNamespaceDataSource() datasource.DataSource {
	return &namespaceDataSource{}
}

type namespaceDataSource struct {
	client *client.Client
}

func (d *namespaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *namespaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (d *namespaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about a Namespace.",
		Attributes:  namespaceDataSourceSchema(true),
	}
}

func (d *namespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input namespaceDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.ID.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid namespace id", "namespace id is required")
		return
	}

	namespaceResp, err := d.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: input.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch namespaces", err.Error())
		return
	}

	namespaceModel, diags := namespaceToNamespaceDataModel(ctx, namespaceResp.GetNamespace())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, namespaceModel)
	resp.Diagnostics.Append(diags...)
}
