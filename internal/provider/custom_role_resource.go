package provider

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
)

type (
	customRoleResource struct {
		client *client.Client
	}

	customRoleResourceModel struct {
		ID          types.String   `tfsdk:"id"`
		State       types.String   `tfsdk:"state"`
		Name        types.String   `tfsdk:"name"`
		Description types.String   `tfsdk:"description"`
		Permissions types.List     `tfsdk:"permissions"`
		Timeouts    timeouts.Value `tfsdk:"timeouts"`
	}

	customRolePermissionModel struct {
		Actions   types.Set    `tfsdk:"actions"`
		Resources types.Object `tfsdk:"resources"`
	}

	customRoleResourcesModel struct {
		ResourceType types.String `tfsdk:"resource_type"`
		ResourceIDs  types.Set    `tfsdk:"resource_ids"`
		AllowAll     types.Bool   `tfsdk:"allow_all"`
	}
)

var (
	_ resource.Resource                = (*customRoleResource)(nil)
	_ resource.ResourceWithConfigure   = (*customRoleResource)(nil)
	_ resource.ResourceWithImportState = (*customRoleResource)(nil)

	customRoleResourcesAttrs = map[string]attr.Type{
		"resource_type": types.StringType,
		"resource_ids":  types.SetType{ElemType: types.StringType},
		"allow_all":     types.BoolType,
	}

	customRolePermissionAttrs = map[string]attr.Type{
		"actions":   types.SetType{ElemType: types.StringType},
		"resources": types.ObjectType{AttrTypes: customRoleResourcesAttrs},
	}
)

func NewCustomRoleResource() resource.Resource {
	return &customRoleResource{}
}

func (r *customRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client
}

func (r *customRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_role"
}

func (r *customRoleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud custom role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the custom role.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description: "The current state of the custom role.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the custom role.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description of the custom role.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"permissions": schema.ListNestedAttribute{
				Description: "The permissions assigned to the custom role.",
				Required:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"actions": schema.SetAttribute{
							Description: "The actions allowed by this permission.",
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
							},
						},
						"resources": schema.SingleNestedAttribute{
							Description: "The resources this permission applies to.",
							Required:    true,
							Attributes: map[string]schema.Attribute{
								"resource_type": schema.StringAttribute{
									Description: "The resource type this permission applies to.",
									Required:    true,
								},
								"resource_ids": schema.SetAttribute{
									Description: "The resource IDs this permission applies to. Can be empty when allow_all is true.",
									Required:    true,
									ElementType: types.StringType,
								},
								"allow_all": schema.BoolAttribute{
									Description: "Whether this permission applies to all resources of the given type.",
									Optional:    true,
									Computed:    true,
									Default:     booldefault.StaticBool(false),
								},
							},
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *customRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	spec, d := getCustomRoleSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().CreateCustomRole(ctx, &cloudservicev1.CreateCustomRoleRequest{
		Spec:             spec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create custom role", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to create custom role", err.Error())
		return
	}

	customRole, err := r.client.CloudService().GetCustomRole(ctx, &cloudservicev1.GetCustomRoleRequest{
		RoleId: svcResp.GetRoleId(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get custom role after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateCustomRoleModelFromSpec(ctx, &plan, customRole.GetCustomRole())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *customRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	customRole, err := r.client.CloudService().GetCustomRole(ctx, &cloudservicev1.GetCustomRoleRequest{
		RoleId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Custom Role resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get custom role", err.Error())
		return
	}

	resp.Diagnostics.Append(updateCustomRoleModelFromSpec(ctx, &state, customRole.GetCustomRole())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *customRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	currentCustomRole, err := r.client.CloudService().GetCustomRole(ctx, &cloudservicev1.GetCustomRoleRequest{
		RoleId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current custom role status", err.Error())
		return
	}

	spec, d := getCustomRoleSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateCustomRole(ctx, &cloudservicev1.UpdateCustomRoleRequest{
		RoleId:           plan.ID.ValueString(),
		Spec:             spec,
		ResourceVersion:  currentCustomRole.GetCustomRole().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update custom role", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update custom role", err.Error())
		return
	}

	customRole, err := r.client.CloudService().GetCustomRole(ctx, &cloudservicev1.GetCustomRoleRequest{
		RoleId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get custom role after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateCustomRoleModelFromSpec(ctx, &plan, customRole.GetCustomRole())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *customRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	currentCustomRole, err := r.client.CloudService().GetCustomRole(ctx, &cloudservicev1.GetCustomRoleRequest{
		RoleId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Custom Role resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			return
		}

		resp.Diagnostics.AddError("Failed to get current custom role status", err.Error())
		return
	}

	svcResp, err := r.client.CloudService().DeleteCustomRole(ctx, &cloudservicev1.DeleteCustomRoleRequest{
		RoleId:           state.ID.ValueString(),
		ResourceVersion:  currentCustomRole.GetCustomRole().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete custom role", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to delete custom role", err.Error())
		return
	}
}

func (r *customRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getCustomRoleSpecFromModel(ctx context.Context, model *customRoleResourceModel) (*identityv1.CustomRoleSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	permissions := make([]types.Object, 0, len(model.Permissions.Elements()))
	diags.Append(model.Permissions.ElementsAs(ctx, &permissions, false)...)
	if diags.HasError() {
		return nil, diags
	}

	specPermissions := make([]*identityv1.CustomRoleSpec_Permission, 0, len(permissions))
	for _, permission := range permissions {
		var permissionModel customRolePermissionModel
		diags.Append(permission.As(ctx, &permissionModel, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		var resourcesModel customRoleResourcesModel
		diags.Append(permissionModel.Resources.As(ctx, &resourcesModel, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		actions := make([]string, 0, len(permissionModel.Actions.Elements()))
		diags.Append(permissionModel.Actions.ElementsAs(ctx, &actions, false)...)
		if diags.HasError() {
			return nil, diags
		}

		resourceIDs := make([]string, 0, len(resourcesModel.ResourceIDs.Elements()))
		diags.Append(resourcesModel.ResourceIDs.ElementsAs(ctx, &resourceIDs, false)...)
		if diags.HasError() {
			return nil, diags
		}

		specPermissions = append(specPermissions, &identityv1.CustomRoleSpec_Permission{
			Actions: actions,
			Resources: &identityv1.CustomRoleSpec_Resources{
				ResourceType: resourcesModel.ResourceType.ValueString(),
				ResourceIds:  resourceIDs,
				AllowAll:     resourcesModel.AllowAll.ValueBool(),
			},
		})
	}

	description := ""
	if !model.Description.IsNull() {
		description = model.Description.ValueString()
	}

	return &identityv1.CustomRoleSpec{
		Name:        model.Name.ValueString(),
		Description: description,
		Permissions: specPermissions,
	}, diags
}

func updateCustomRoleModelFromSpec(ctx context.Context, state *customRoleResourceModel, customRole *identityv1.CustomRole) diag.Diagnostics {
	var diags diag.Diagnostics

	state.ID = types.StringValue(customRole.GetId())
	state.Name = types.StringValue(customRole.GetSpec().GetName())
	state.Description = types.StringValue(customRole.GetSpec().GetDescription())

	stateStr, err := enums.FromResourceState(customRole.GetState())
	if err != nil {
		diags.AddError("Failed to convert resource state", err.Error())
		return diags
	}
	state.State = types.StringValue(stateStr)

	permissionObjects := make([]types.Object, 0, len(customRole.GetSpec().GetPermissions()))
	for _, permission := range customRole.GetSpec().GetPermissions() {
		actions, d := types.SetValueFrom(ctx, types.StringType, permission.GetActions())
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		resourceIDs := types.SetValueMust(types.StringType, []attr.Value{})
		if len(permission.GetResources().GetResourceIds()) > 0 {
			resourceIDs, d = types.SetValueFrom(ctx, types.StringType, permission.GetResources().GetResourceIds())
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
		}

		resourcesObject, d := types.ObjectValueFrom(ctx, customRoleResourcesAttrs, customRoleResourcesModel{
			ResourceType: types.StringValue(permission.GetResources().GetResourceType()),
			ResourceIDs:  resourceIDs,
			AllowAll:     types.BoolValue(permission.GetResources().GetAllowAll()),
		})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		permissionObject, d := types.ObjectValueFrom(ctx, customRolePermissionAttrs, customRolePermissionModel{
			Actions:   actions,
			Resources: resourcesObject,
		})
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		permissionObjects = append(permissionObjects, permissionObject)
	}

	permissions, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: customRolePermissionAttrs}, permissionObjects)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	state.Permissions = permissions

	return diags
}
