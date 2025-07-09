package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

type (
	usersDataSource struct {
		client *client.Client
	}

	usersDataModel struct {
		ID    types.String    `tfsdk:"id"`
		Users []userDataModel `tfsdk:"users"`
	}

	userDataModel struct {
		ID                types.String                             `tfsdk:"id"`
		Email             types.String                             `tfsdk:"email"`
		State             types.String                             `tfsdk:"state"`
		AccountAccess     internaltypes.CaseInsensitiveStringValue `tfsdk:"account_access"`
		NamespaceAccesses types.Set                                `tfsdk:"namespace_accesses"`
		CreatedAt         types.String                             `tfsdk:"created_at"`
		UpdatedAt         types.String                             `tfsdk:"updated_at"`
	}

	userNSAccessModel struct {
		NamespaceID types.String `tfsdk:"namespace_id"`
		Permission  types.String `tfsdk:"permission"`
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

func userSchema(idRequired bool) map[string]schema.Attribute {
	idAttribute := schema.StringAttribute{
		Description: "The unique identifier of the User.",
	}

	switch idRequired {
	case true:
		idAttribute.Required = true
	case false:
		idAttribute.Computed = true
	}

	return map[string]schema.Attribute{
		"id": idAttribute,
		"email": schema.StringAttribute{
			Description: "The email of the User.",
			Computed:    true,
		},
		"state": schema.StringAttribute{
			Description: "The current state of the User.",
			Computed:    true,
		},
		"account_access": schema.StringAttribute{
			CustomType:  internaltypes.CaseInsensitiveStringType{},
			Description: "The role on the account. Must be one of admin, developer, or read (case-insensitive).",
			Computed:    true,
		},
		"namespace_accesses": schema.SetNestedAttribute{
			Description: "The set of namespace permissions for this user, including each namespace and its role.",
			Optional:    true,
			Computed:    true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"namespace_id": schema.StringAttribute{
						Description: "The namespace to assign permissions to.",
						Computed:    true,
					},
					"permission": schema.StringAttribute{
						CustomType:  types.StringType,
						Description: "The permission to assign. Must be one of admin, write, or read (case-insensitive)",
						Computed:    true,
					},
				},
			},
		},
		"created_at": schema.StringAttribute{
			Description: "The creation time of the User.",
			Computed:    true,
		},
		"updated_at": schema.StringAttribute{
			Description: "The last update time of the User.",
			Computed:    true,
		},
	}
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

func userToUserDataModel(ctx context.Context, sa *identityv1.User) (*userDataModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	stateStr, err := enums.FromResourceState(sa.State)
	if err != nil {
		diags.AddError("Unable to convert user state", err.Error())
		return nil, diags
	}

	userModel := &userDataModel{
		ID:        types.StringValue(sa.Id),
		Email:     types.StringValue(sa.GetSpec().GetEmail()),
		State:     types.StringValue(stateStr),
		CreatedAt: types.StringValue(sa.GetCreatedTime().AsTime().GoString()),
		UpdatedAt: types.StringValue(sa.GetLastModifiedTime().AsTime().GoString()),
	}

	role, err := enums.FromAccountAccessRole(sa.GetSpec().GetAccess().GetAccountAccess().GetRole())
	if err != nil {
		diags.AddError("Failed to convert account access role", err.Error())
		return nil, diags
	}

	userModel.AccountAccess = internaltypes.CaseInsensitiveString(role)

	namespaceAccesses := types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs})

	if len(sa.GetSpec().GetAccess().GetNamespaceAccesses()) > 0 {
		namespaceAccessObjects := make([]types.Object, 0)
		for ns, namespaceAccess := range sa.GetSpec().GetAccess().GetNamespaceAccesses() {
			permission, err := enums.FromNamespaceAccessPermission(namespaceAccess.GetPermission())
			if err != nil {
				diags.AddError("Failed to convert namespace access permission", err.Error())
				return nil, diags
			}

			model := userNSAccessModel{
				NamespaceID: types.StringValue(ns),
				Permission:  types.StringValue(permission),
			}
			obj, d := types.ObjectValueFrom(ctx, userNamespaceAccessAttrs, model)
			diags.Append(d...)
			if diags.HasError() {
				return nil, diags
			}

			namespaceAccessObjects = append(namespaceAccessObjects, obj)
		}

		accesses, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: userNamespaceAccessAttrs}, namespaceAccessObjects)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		namespaceAccesses = accesses
	}
	userModel.NamespaceAccesses = namespaceAccesses

	return userModel, diags
}
