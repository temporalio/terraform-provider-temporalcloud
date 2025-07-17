package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

type (
	usersDataSource struct {
		client *client.Client
	}
)

var (
	_ datasource.DataSource              = (*usersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*usersDataSource)(nil)
)

func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

func (d *usersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *usersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about all Users.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Users data source.",
				Computed:    true,
			},
			"users": schema.ListNestedAttribute{
				Description: "The list of Users.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: userSchema(false),
				},
			},
		},
	}
}

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state usersDataModel

	var users []*identityv1.User
	pageToken := ""
	for {
		r, err := d.client.CloudService().GetUsers(ctx, &cloudservicev1.GetUsersRequest{PageToken: pageToken})
		if err != nil {
			resp.Diagnostics.AddError("Unable to fetch users", err.Error())
			return
		}

		users = append(users, r.GetUsers()...)

		if r.GetNextPageToken() == "" {
			break
		}

		pageToken = r.GetNextPageToken()
	}

	for _, sa := range users {
		userModel, diags := userToUserDataModel(ctx, sa)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		state.Users = append(state.Users, *userModel)
	}

	accResp, err := d.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account information.", err.Error())
		return
	}

	state.ID = types.StringValue(fmt.Sprintf("account-%s-users", accResp.GetAccount().GetId()))
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
