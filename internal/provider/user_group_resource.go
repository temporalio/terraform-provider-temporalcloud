package provider

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
)

type (
	userGroupResource struct {
		client *client.Client
	}

	userGroupResourceModel struct {
		ID    types.String `tfsdk:"id"`
		State types.String `tfsdk:"state"`
		Name  types.String `tfsdk:"name"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*userGroupResource)(nil)
	_ resource.ResourceWithConfigure   = (*userGroupResource)(nil)
	_ resource.ResourceWithImportState = (*userGroupResource)(nil)
)

func NewUserGroupResource() resource.Resource {
	return &userGroupResource{}
}

func (r *userGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *userGroupResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := schema.Schema{
		Description: "Provisions a Temporal Cloud User Group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the group.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description: "The current state of the group.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the group",
				Required:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}

	resp.Schema = s
}

func (r *userGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userGroupResourceModel
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

	svcResp, err := r.client.CloudService().CreateUserGroup(ctx, &cloudservicev1.CreateUserGroupRequest{
		Spec: &identityv1.UserGroupSpec{
			DisplayName: plan.Name.ValueString(),
			GroupType: &identityv1.UserGroupSpec_CloudGroup{
				CloudGroup: &identityv1.CloudGroupSpec{},
			},
		},
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}

	group, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: svcResp.GroupId,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupModelFromSpec(ctx, &plan, group.Group)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Group Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get group", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupModelFromSpec(ctx, &state, group.GetGroup())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *userGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentGroup, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current group status", err.Error())
		return
	}

	svcResp, err := r.client.CloudService().UpdateUserGroup(ctx, &cloudservicev1.UpdateUserGroupRequest{
		GroupId: plan.ID.ValueString(),
		Spec: &identityv1.UserGroupSpec{
			DisplayName: plan.Name.ValueString(),
			Access:      currentGroup.GetGroup().GetSpec().GetAccess(),
			GroupType: &identityv1.UserGroupSpec_CloudGroup{
				CloudGroup: &identityv1.CloudGroupSpec{},
			},
		},
		ResourceVersion:  currentGroup.GetGroup().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update group", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update group", err.Error())
		return
	}

	group, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupModelFromSpec(ctx, &plan, group.Group)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentGroup, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Group Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get current group status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteUserGroup(ctx, &cloudservicev1.DeleteUserGroupRequest{
		GroupId:          state.ID.ValueString(),
		ResourceVersion:  currentGroup.GetGroup().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Group Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete group", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete group", err.Error())
	}
}

func (r *userGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updateGroupModelFromSpec(ctx context.Context, state *userGroupResourceModel, group *identityv1.UserGroup) diag.Diagnostics {
	var diags diag.Diagnostics
	state.ID = types.StringValue(group.GetId())
	stateStr, err := enums.FromResourceState(group.GetState())
	if err != nil {
		diags.AddError("Failed to convert resource state", err.Error())
		return diags
	}
	state.State = types.StringValue(stateStr)
	state.Name = types.StringValue(group.GetSpec().GetDisplayName())

	return diags
}
