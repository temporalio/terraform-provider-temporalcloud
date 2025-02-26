package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
)

type (
	serviceAccountsDataSource struct {
		client *client.Client
	}

	serviceAccountsDataModel struct {
		ID              types.String              `tfsdk:"id"`
		ServiceAccounts []serviceAccountDataModel `tfsdk:"service_accounts"`
	}

	serviceAccountDataModel struct {
		ID                types.String                             `tfsdk:"id"`
		Name              types.String                             `tfsdk:"name"`
		Description       types.String                             `tfsdk:"description"`
		State             types.String                             `tfsdk:"state"`
		AccountAccess     internaltypes.CaseInsensitiveStringValue `tfsdk:"account_access"`
		NamespaceAccesses types.Set                                `tfsdk:"namespace_accesses"`
		CreatedAt         types.String                             `tfsdk:"created_at"`
		UpdatedAt         types.String                             `tfsdk:"updated_at"`
	}

	serviceAccountNSAccessModel struct {
		NamespaceID types.String `tfsdk:"namespace_id"`
		Permission  types.String `tfsdk:"permission"`
	}
)

var (
	_ datasource.DataSource              = (*serviceAccountsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serviceAccountsDataSource)(nil)
)

func NewServiceAccountsDataSource() datasource.DataSource {
	return &serviceAccountsDataSource{}
}

func (d *serviceAccountsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_accounts"
}

func (d *serviceAccountsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serviceAccountsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about all Service Accounts.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Service Accounts data source.",
				Computed:    true,
			},
			"service_accounts": schema.ListNestedAttribute{
				Description: "The list of Service Accounts.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "The unique identifier of the Service Account.",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "The description of the Service Account.",
							Computed:    true,
						},
						"state": schema.StringAttribute{
							Description: "The current state of the Service Account.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "The name associated with the service account.",
							Computed:    true,
						},
						"account_access": schema.StringAttribute{
							CustomType:  internaltypes.CaseInsensitiveStringType{},
							Description: "The role on the account. Must be one of admin, developer, or read (case-insensitive).",
							Computed:    true,
						},
						"namespace_accesses": schema.SetNestedAttribute{
							Description: "The set of namespace permissions for this service account, including each namespace and its role.",
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
							Description: "The creation time of the Service Account.",
							Computed:    true,
						},
						"updated_at": schema.StringAttribute{
							Description: "The last update time of the Service Account.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *serviceAccountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state serviceAccountsDataModel

	var serviceAccounts []*identityv1.ServiceAccount
	pageToken := ""
	for {
		r, err := d.client.CloudService().GetServiceAccounts(ctx, &cloudservicev1.GetServiceAccountsRequest{PageToken: pageToken})
		if err != nil {
			resp.Diagnostics.AddError("Unable to fetch service accounts", err.Error())
			return
		}

		serviceAccounts = append(serviceAccounts, r.GetServiceAccount()...)

		if r.GetNextPageToken() == "" {
			break
		}

		pageToken = r.GetNextPageToken()
	}

	for _, sa := range serviceAccounts {
		stateStr, err := enums.FromResourceState(sa.State)
		if err != nil {
			resp.Diagnostics.AddError("Unable to convert service account state", err.Error())
			return
		}

		serviceAccountModel := serviceAccountDataModel{
			ID:          types.StringValue(sa.Id),
			Name:        types.StringValue(sa.GetSpec().GetName()),
			Description: types.StringValue(sa.GetSpec().GetDescription()),
			State:       types.StringValue(stateStr),
			CreatedAt:   types.StringValue(sa.GetCreatedTime().AsTime().GoString()),
			UpdatedAt:   types.StringValue(sa.GetLastModifiedTime().AsTime().GoString()),
		}

		role, err := enums.FromAccountAccessRole(sa.GetSpec().GetAccess().GetAccountAccess().GetRole())
		if err != nil {
			resp.Diagnostics.AddError("Failed to convert account access role", err.Error())
			return
		}

		serviceAccountModel.AccountAccess = internaltypes.CaseInsensitiveString(role)

		namespaceAccesses := types.SetNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs})

		if len(sa.GetSpec().GetAccess().GetNamespaceAccesses()) > 0 {
			namespaceAccessObjects := make([]types.Object, 0)
			for ns, namespaceAccess := range sa.GetSpec().GetAccess().GetNamespaceAccesses() {
				permission, err := enums.FromNamespaceAccessPermission(namespaceAccess.GetPermission())
				if err != nil {
					resp.Diagnostics.AddError("Failed to convert namespace access permission", err.Error())
					continue
				}
				model := serviceAccountNSAccessModel{
					NamespaceID: types.StringValue(ns),
					Permission:  types.StringValue(permission),
				}
				obj, d := types.ObjectValueFrom(ctx, serviceAccountNamespaceAccessAttrs, model)
				resp.Diagnostics.Append(d...)
				if d.HasError() {
					continue
				}
				namespaceAccessObjects = append(namespaceAccessObjects, obj)
			}

			accesses, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}, namespaceAccessObjects)
			resp.Diagnostics.Append(d...)
			if resp.Diagnostics.HasError() {
				return
			}
			namespaceAccesses = accesses
		}
		serviceAccountModel.NamespaceAccesses = namespaceAccesses

		state.ServiceAccounts = append(state.ServiceAccounts, serviceAccountModel)

	}

	accResp, err := d.client.CloudService().GetAccount(ctx, &cloudservicev1.GetAccountRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account information.", err.Error())
		return
	}

	state.ID = types.StringValue(fmt.Sprintf("account-%s-service-accounts", accResp.GetAccount().GetId()))
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
