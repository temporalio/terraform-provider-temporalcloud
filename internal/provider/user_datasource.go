package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

type (
	userDataSource struct {
		client *client.Client
	}
)

var (
	_ datasource.DataSource              = (*userDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userDataSource)(nil)
)

func NewUserDataSource() datasource.DataSource {
	return &userDataSource{}
}

func (d *userDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *userDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about a User.",
		Attributes:  userSchema(true),
	}
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input userDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.ID.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid user id", "user id is required")
		return
	}

	saResp, err := d.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: input.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch user", err.Error())
		return
	}

	saDataModel, diags := userToUserDataModel(ctx, saResp.GetUser())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &saDataModel)
	resp.Diagnostics.Append(diags...)
}
