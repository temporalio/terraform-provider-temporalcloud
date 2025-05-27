// The MIT License
//
// Copyright (c) 2025 Temporal Technologies Inc.  All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package provider

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	"go.temporal.io/cloud-sdk/api/identity/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
)

type groupAccessResource struct {
	client *client.Client
}

type groupAccessResourceModel struct {
	ID types.String `tfsdk:"id"`

	AccountAccess     internaltypes.CaseInsensitiveStringValue `tfsdk:"account_access"`
	NamespaceAccesses types.Set                                `tfsdk:"namespace_accesses"`
}

var (
	_ resource.Resource                = (*groupAccessResource)(nil)
	_ resource.ResourceWithConfigure   = (*groupAccessResource)(nil)
	_ resource.ResourceWithImportState = (*groupAccessResource)(nil)
)

func NewGroupAccessResource() resource.Resource {
	return &groupAccessResource{}
}

func (r *groupAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_access"
}

func (r *groupAccessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := schema.Schema{
		Description: "Provisions Temporal Cloud group access.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the group access across all Temporal Cloud tenants.",
				Computed:    false,
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}

	addAccessSchemaAttrs(s)
	resp.Schema = s
}

func (r *groupAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupAccessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespaceAccesses, d := getNamespaceAccessesFromSet(ctx, plan.NamespaceAccesses)
	resp.Diagnostics.Append(d...)
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

	role, err := enums.ToAccountAccessRole(plan.AccountAccess.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert account access role", err.Error())
		return
	}
	access := &identityv1.Access{
		AccountAccess: &identityv1.AccountAccess{
			Role: role,
		},
		NamespaceAccesses: namespaceAccesses,
	}
	// If the role is unspecified (i.e. none), remove the account access from the spec.
	if role == identityv1.AccountAccess_ROLE_UNSPECIFIED {
		access.AccountAccess = nil
	}

	// Use the current group spec to update the access.
	spec := currentGroup.GetGroup().GetSpec()
	spec.Access = access
	svcResp, err := r.client.CloudService().UpdateUserGroup(ctx, &cloudservicev1.UpdateUserGroupRequest{
		GroupId:          plan.ID.ValueString(),
		Spec:             spec,
		ResourceVersion:  currentGroup.GetGroup().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group access", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to create group access", err.Error())
		return
	}

	group, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group after create", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupAccessModel(ctx, &plan, group.Group)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)

}

func (r *groupAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Group Access Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get group access", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupAccessModel(ctx, &state, model.Group)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *groupAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupAccessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespaceAccesses, d := getNamespaceAccessesFromSet(ctx, plan.NamespaceAccesses)
	resp.Diagnostics.Append(d...)
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

	role, err := enums.ToAccountAccessRole(plan.AccountAccess.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert account access role", err.Error())
		return
	}
	access := &identityv1.Access{
		AccountAccess: &identityv1.AccountAccess{
			Role: role,
		},
		NamespaceAccesses: namespaceAccesses,
	}
	// If the role is unspecified (i.e. none), remove the account access from the spec.
	if role == identityv1.AccountAccess_ROLE_UNSPECIFIED {
		access.AccountAccess = nil
	}

	// Use the current group spec to update the access.
	spec := currentGroup.GetGroup().GetSpec()
	spec.Access = access
	svcResp, err := r.client.CloudService().UpdateUserGroup(ctx, &cloudservicev1.UpdateUserGroupRequest{
		GroupId:          plan.ID.ValueString(),
		Spec:             spec,
		ResourceVersion:  currentGroup.GetGroup().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update group access", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update group access", err.Error())
		return
	}

	group, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get group after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateGroupAccessModel(ctx, &plan, group.Group)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentGroup, err := r.client.CloudService().GetUserGroup(ctx, &cloudservicev1.GetUserGroupRequest{
		GroupId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Group Access Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError("Failed to get current group status", err.Error())
		return
	}

	// Create an empty access spec to remove all access
	access := &identityv1.Access{
		AccountAccess:     nil,
		NamespaceAccesses: map[string]*identityv1.NamespaceAccess{},
	}

	// Use the current group spec to update the access
	spec := currentGroup.GetGroup().GetSpec()
	spec.Access = access

	svcResp, err := r.client.CloudService().UpdateUserGroup(ctx, &cloudservicev1.UpdateUserGroupRequest{
		GroupId:          state.ID.ValueString(),
		Spec:             spec,
		ResourceVersion:  currentGroup.GetGroup().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})

	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Group Access Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError("Failed to remove group access", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to remove group access", err.Error())
		return
	}
}

func (r *groupAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updateGroupAccessModel(ctx context.Context, state *groupAccessResourceModel, group *identity.UserGroup) diag.Diagnostics {
	var diags diag.Diagnostics

	if group == nil {
		return diags
	}

	state.ID = types.StringValue(group.Id)

	role, err := enums.FromAccountAccessRole(group.GetSpec().GetAccess().GetAccountAccess().GetRole())
	if err != nil {
		diags.AddError("Failed to convert account access role", err.Error())
		return diags
	}
	state.AccountAccess = internaltypes.CaseInsensitiveString(role)

	namespaceAccesses, d := getNamespaceSetFromSpec(ctx, group.GetSpec().GetAccess())
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	state.NamespaceAccesses = namespaceAccesses

	return diags
}
