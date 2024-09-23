// The MIT License
//
// Copyright (c) 2024 Temporal Technologies Inc.  All rights reserved.
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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/jpillora/maplock"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	identityv1 "go.temporal.io/api/cloud/identity/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

type (
	userNamespaceAccessResource struct {
		client *client.Client
	}

	userNamespaceAccessResourceModel struct {
		ID          types.String `tfsdk:"id"`
		NamespaceID types.String `tfsdk:"namespace_id"`
		UserID      types.String `tfsdk:"user_id"`
		Permission  types.String `tfsdk:"permission"`
	}
)

var (
	_ resource.Resource                = (*userNamespaceAccessResource)(nil)
	_ resource.ResourceWithConfigure   = (*userNamespaceAccessResource)(nil)
	_ resource.ResourceWithImportState = (*userNamespaceAccessResource)(nil)

	// userLocks is a pser-user mutex that protects against concurrent updates to the same user spec,
	// which can happen when a single user is granted access to multiple namespaces in parallel.
	userLocks = maplock.New()
)

func NewUserNamespaceAccessResource() resource.Resource {
	return &userNamespaceAccessResource{}
}

func (r *userNamespaceAccessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userNamespaceAccessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_namespace_access"
}

func (r *userNamespaceAccessResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier for the user namespace access.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"namespace_id": schema.StringAttribute{
				Description: "The ID of the namespace to which this user should be given the requested role",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"user_id": schema.StringAttribute{
				Description: "The ID of the user to which this namespace access should be granted",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permission": schema.StringAttribute{
				Description: "The permission to grant the user in the namespace",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive("admin", "write", "read"),
				},
			},
		},
	}
}

func (r *userNamespaceAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userNamespaceAccessResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	withUserLock(plan.UserID, func() {
		userResp, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
			UserId: plan.UserID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to get user", err.Error())
			return
		}

		user := userResp.GetUser()
		existingAccesses := user.GetSpec().GetAccess().GetNamespaceAccesses()
		if existingAccesses == nil {
			existingAccesses = make(map[string]*identityv1.NamespaceAccess)
		}
		if existingAccesses[plan.NamespaceID.ValueString()] != nil {
			resp.Diagnostics.AddError("User already has access to namespace, cowardly refusing to mutate this access", "")
		}
		existingAccesses[plan.NamespaceID.ValueString()] = &identityv1.NamespaceAccess{
			Permission: plan.Permission.ValueString(),
		}

		user.GetSpec().GetAccess().NamespaceAccesses = existingAccesses
		_, err = r.client.CloudService().UpdateUser(ctx, &cloudservicev1.UpdateUserRequest{
			UserId:          user.GetId(),
			Spec:            user.GetSpec(),
			ResourceVersion: user.GetResourceVersion(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to update user", err.Error())
			return
		}

		plan.ID = types.StringValue(fmt.Sprintf("%s-%s", plan.UserID.ValueString(), plan.NamespaceID.ValueString()))
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	})
}

func (r *userNamespaceAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userNamespaceAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: state.UserID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get user", err.Error())
		return
	}

	user := model.GetUser()
	nsAccess, ok := user.GetSpec().GetAccess().GetNamespaceAccesses()[state.NamespaceID.ValueString()]
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Permission = types.StringValue(nsAccess.GetPermission())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userNamespaceAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (r *userNamespaceAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userNamespaceAccessResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	withUserLock(state.UserID, func() {
		user, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
			UserId: state.UserID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to get user", err.Error())
			return
		}

		delete(user.GetUser().GetSpec().GetAccess().GetNamespaceAccesses(), state.NamespaceID.ValueString())
		svcResp, err := r.client.CloudService().UpdateUser(ctx, &cloudservicev1.UpdateUserRequest{
			UserId:          user.GetUser().GetId(),
			Spec:            user.GetUser().GetSpec(),
			ResourceVersion: user.GetUser().GetResourceVersion(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to update user", err.Error())
			return
		}

		if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
			resp.Diagnostics.AddError("Failed to await user modification", err.Error())
		}
	})
}

func (r *userNamespaceAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	components := strings.Split(req.ID, "/")
	if len(components) != 2 {
		resp.Diagnostics.AddError("Invalid import ID for User Namespace access", "The import ID must be in the format `NamepaceID/SearchAttributeName`, such as `yournamespace.deadbeef/UserID`")
		return
	}

	userID, namespaceID := components[0], components[1]
	var state userNamespaceAccessResourceModel
	state.ID = types.StringValue(fmt.Sprintf("%s-%s", userID, namespaceID))
	state.NamespaceID = types.StringValue(namespaceID)
	state.UserID = types.StringValue(userID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func withUserLock(userID basetypes.StringValue, f func()) {
	userIDStr := userID.ValueString()
	userLocks.Lock(userIDStr)
	defer func() {
		_ = userLocks.Unlock(userIDStr)
	}()
	f()
}
