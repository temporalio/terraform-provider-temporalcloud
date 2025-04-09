package provider

import (
	"context"
	"fmt"

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
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

type (
	userGroupMembersResource struct {
		client *client.Client
	}

	userGroupMembersResourceModel struct {
		ID      types.String `tfsdk:"id"`
		GroupID types.String `tfsdk:"group_id"`
		Users   types.Set    `tfsdk:"users"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*userGroupMembersResource)(nil)
	_ resource.ResourceWithConfigure   = (*userGroupMembersResource)(nil)
	_ resource.ResourceWithImportState = (*userGroupMembersResource)(nil)
)

func NewUserGroupMembersResource() resource.Resource {
	return &userGroupMembersResource{}
}

func (r *userGroupMembersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userGroupMembersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_members"
}

func (r *userGroupMembersResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sets Group Membership for the provided Group ID. Only use one per group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the group.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Description: "The Group ID to set the members for.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"users": schema.SetAttribute{
				Description: "The users to add to the group.",
				Required:    true,
				ElementType: types.StringType,
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}

func (r *userGroupMembersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userGroupMembersResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, defaultCreateTimeout*2)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	existingUsers, err := getAllUserGroupMembers(ctx, r.client, plan.GroupID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current group members", err.Error())
		return
	}

	plannedUsers, d := getUsersFromSet(ctx, plan.Users)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	err = r.setUserGroupMembers(ctx, plan.GroupID.ValueString(), existingUsers, plannedUsers)
	if err != nil {
		resp.Diagnostics.AddError("Failed to set group members", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupMembersModelFromSpec(ctx, &plan, plan.GroupID.ValueString(), plannedUsers)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userGroupMembersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userGroupMembersResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, err := getAllUserGroupMembers(ctx, r.client, state.GroupID.ValueString())
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Group Resource not found, removing from state", map[string]interface{}{
				"id": state.GroupID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get group members", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupMembersModelFromSpec(ctx, &state, state.GroupID.ValueString(), users)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *userGroupMembersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userGroupMembersResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, err := getAllUserGroupMembers(ctx, r.client, plan.GroupID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current group status", err.Error())
		return
	}
	plannedUsers, d := getUsersFromSet(ctx, plan.Users)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	err = r.setUserGroupMembers(ctx, plan.GroupID.ValueString(), users, plannedUsers)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update group", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userGroupMembersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userGroupMembersResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout*2)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, d := getUsersFromSet(ctx, state.Users)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	err := r.setUserGroupMembers(ctx, state.GroupID.ValueString(), existing, []string{})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Group Resource not found, removing from state", map[string]interface{}{
				"id": state.GroupID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete group members", err.Error())
		return
	}
}

func (r *userGroupMembersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *userGroupMembersResource) setUserGroupMembers(ctx context.Context, groupID string, existing, planned []string) error {
	added, removed := internaltypes.ListDiff(existing, planned)
	for _, u := range added {
		resp, err := r.client.CloudService().AddUserGroupMember(ctx, &cloudservicev1.AddUserGroupMemberRequest{
			GroupId: groupID,
			MemberId: &identityv1.UserGroupMemberId{
				MemberType: &identityv1.UserGroupMemberId_UserId{
					UserId: u,
				},
			},
		})
		if err != nil {
			return err
		}

		if err := client.AwaitAsyncOperation(ctx, r.client, resp.GetAsyncOperation()); err != nil {
			return err
		}
	}

	for _, u := range removed {
		resp, err := r.client.CloudService().RemoveUserGroupMember(ctx, &cloudservicev1.RemoveUserGroupMemberRequest{
			GroupId: groupID,
			MemberId: &identityv1.UserGroupMemberId{
				MemberType: &identityv1.UserGroupMemberId_UserId{
					UserId: u,
				},
			},
		})
		if err != nil {
			return err
		}

		if err := client.AwaitAsyncOperation(ctx, r.client, resp.GetAsyncOperation()); err != nil {
			return err
		}
	}

	return nil
}

func updateGroupMembersModelFromSpec(ctx context.Context, state *userGroupMembersResourceModel, groupId string, users []string) diag.Diagnostics {
	var diags diag.Diagnostics
	state.ID = types.StringValue(fmt.Sprintf("group-members-%s", groupId))
	state.GroupID = types.StringValue(groupId)
	userSet := types.SetNull(types.ObjectType{AttrTypes: namespaceAccessAttrs})
	if len(users) > 0 {
		us, d := types.SetValueFrom(ctx, types.StringType, users)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		userSet = us
	}
	state.Users = userSet

	return diags
}

func getAllUserGroupMembers(ctx context.Context, client *client.Client, groupID string) ([]string, error) {
	var users []string
	pageToken := ""
	for {
		resp, err := client.CloudService().GetUserGroupMembers(ctx, &cloudservicev1.GetUserGroupMembersRequest{
			GroupId:   groupID,
			PageToken: pageToken,
			PageSize:  100,
		})
		if err != nil {
			return nil, err
		}

		for _, m := range resp.GetMembers() {
			memberId := m.GetMemberId()
			if memberId != nil && memberId.GetUserId() != "" {
				users = append(users, memberId.GetUserId())
			}
		}

		if resp.GetNextPageToken() == "" {
			break
		}

		pageToken = resp.GetNextPageToken()
	}

	return users, nil
}

func getUsersFromSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	users := make([]string, 0, len(set.Elements()))
	diags.Append(set.ElementsAs(ctx, &users, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return users, diags
}
