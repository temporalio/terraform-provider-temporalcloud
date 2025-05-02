package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"go.temporal.io/cloud-sdk/api/identity/v1"
)

var (
	_ datasource.DataSource              = &scimGroupDataSource{}
	_ datasource.DataSourceWithConfigure = &scimGroupDataSource{}
)

func NewSCIMGroupDataSource() datasource.DataSource {
	return &scimGroupDataSource{}
}

type scimGroupDataModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	IdpId types.String `tfsdk:"idp_id"`
}

type scimGroupDataSource struct {
	client *client.Client
}

func (d *scimGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *scimGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim_group"
}

func (d *scimGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about a SCIM group.",
		Attributes:  scimGroupDataSourceSchema(),
	}
}

func (d *scimGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input scimGroupDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Need to get group by idp ID
	// Get user groups, filter by idp ID
	groupResp, err := d.client.CloudService().GetUserGroups(ctx, &cloudservicev1.GetUserGroupsRequest{
		ScimGroup: &cloudservicev1.GetUserGroupsRequest_SCIMGroupFilter{
			IdpId: input.IdpId.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read SCIM group",
			fmt.Sprintf("Unable to read SCIM group %s: %s", input.IdpId.ValueString(), err),
		)
		return
	}

	if len(groupResp.Groups) == 0 {
		resp.Diagnostics.AddError(
			"SCIM group not found",
			fmt.Sprintf("SCIM group %s not found", input.IdpId.ValueString()),
		)
		return
	}
	if len(groupResp.Groups) > 1 {
		resp.Diagnostics.AddError(
			"Multiple SCIM groups found",
			fmt.Sprintf("Multiple SCIM groups found for %s", input.IdpId.ValueString()),
		)
		return
	}
	group := groupResp.Groups[0]

	scimGroupDataModel, diags := scimGroupToDataModel(ctx, group)
	resp.Diagnostics.Append(diags...)

	resp.State.Set(ctx, scimGroupDataModel)
}

func scimGroupDataSourceSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description: "The ID of the SCIM group.",
			Computed:    true,
		},
		"name": schema.StringAttribute{
			Description: "The name of the SCIM group.",
			Computed:    true,
		},
		"idp_id": schema.StringAttribute{
			Description: "The IDP ID of the SCIM group.",
			Required:    true,
		},
	}
}

func scimGroupToDataModel(_ context.Context, group *identity.UserGroup) (scimGroupDataModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var groupDataModel scimGroupDataModel

	if group == nil {
		return groupDataModel, diags
	}

	groupDataModel.ID = types.StringValue(group.Id)
	groupDataModel.Name = types.StringValue(group.Spec.DisplayName)
	if group.Spec.GetScimGroup() == nil {
		diags.AddError("Invalid SCIM group", fmt.Sprintf("group %s is not a SCIM group", group.Id))
		return groupDataModel, diags
	}
	groupDataModel.IdpId = types.StringValue(group.Spec.GetScimGroup().IdpId)

	return groupDataModel, diags
}
