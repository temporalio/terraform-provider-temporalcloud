package provider

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/validation"
)

type (
	userResource struct {
		client *client.Client
	}

	userResourceModel struct {
		ID                types.String                             `tfsdk:"id"`
		State             types.String                             `tfsdk:"state"`
		Email             types.String                             `tfsdk:"email"`
		AccountAccess     internaltypes.CaseInsensitiveStringValue `tfsdk:"account_access"`
		NamespaceAccesses types.Set                                `tfsdk:"namespace_accesses"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}

	userNamespaceAccessModel struct {
		NamespaceID types.String                             `tfsdk:"namespace_id"`
		Permission  internaltypes.CaseInsensitiveStringValue `tfsdk:"permission"`
	}
)

var (
	_ resource.Resource                = (*userResource)(nil)
	_ resource.ResourceWithConfigure   = (*userResource)(nil)
	_ resource.ResourceWithImportState = (*userResource)(nil)

	userNamespaceAccessAttrs = map[string]attr.Type{
		"namespace_id": types.StringType,
		"permission":   internaltypes.CaseInsensitiveStringType{},
	}
)

func NewUserResource() resource.Resource {
	return &userResource{}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the user.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description: "The current state of the user.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Description: "The email address for the user.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"account_access": schema.StringAttribute{
				CustomType:  internaltypes.CaseInsensitiveStringType{},
				Description: "The role on the account. Must be one of owner, admin, developer, none, or read (case-insensitive). owner is only valid for import and cannot be created, updated or deleted without Temporal support. none is only valid for users managed via SCIM that derive their roles from group memberships.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive("owner", "admin", "developer", "read", "none"),
				},
			},
			"namespace_accesses": schema.SetNestedAttribute{
				Description: "The set of namespace accesses. Empty sets are not allowed, omit the attribute instead. Users with account_access roles of owner or admin cannot be assigned explicit permissions to namespaces. They implicitly receive access to all Namespaces.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"namespace_id": schema.StringAttribute{
							Description: "The namespace to assign permissions to.",
							Required:    true,
						},
						"permission": schema.StringAttribute{
							CustomType:  internaltypes.CaseInsensitiveStringType{},
							Description: "The permission to assign. Must be one of admin, write, or read (case-insensitive)",
							Required:    true,
							Validators: []validator.String{
								stringvalidator.OneOfCaseInsensitive("admin", "write", "read"),
							},
						},
					},
				},
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					validation.SetNestedAttributeMustBeUnique("namespace_id"),
				},
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

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userResourceModel
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

	namespaceAccesses, d := getNamespaceAccessesFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := enums.ToAccountAccessRole(plan.AccountAccess.ValueString())
	if err != nil {
		diags.AddError("Failed to convert account access role", err.Error())
		return
	}

	spec := &identityv1.UserSpec{
		Email: plan.Email.ValueString(),
		Access: &identityv1.Access{
			AccountAccess: &identityv1.AccountAccess{
				Role: role,
			},
			NamespaceAccesses: namespaceAccesses,
		},
	}
	if err := enums.ValidateAccess(spec.Access); err != nil {
		resp.Diagnostics.AddError("Invalid Access Configuration", err.Error())
		return
	}
	svcResp, err := r.client.CloudService().CreateUser(ctx, &cloudservicev1.CreateUserRequest{
		Spec:             spec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create user", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create user", err.Error())
		return
	}

	user, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: svcResp.UserId,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get user after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateUserModelFromSpec(ctx, &plan, user.User)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get user", err.Error())
		return
	}

	resp.Diagnostics.Append(updateUserModelFromSpec(ctx, &state, user.User)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespaceAccesses, d := getNamespaceAccessesFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentUser, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current user status", err.Error())
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

	if err := enums.ValidateAccess(access); err != nil {
		resp.Diagnostics.AddError("Invalid Access Configuration", err.Error())
		return
	}

	svcResp, err := r.client.CloudService().UpdateUser(ctx, &cloudservicev1.UpdateUserRequest{
		UserId: plan.ID.ValueString(),
		Spec: &identityv1.UserSpec{
			Email:  plan.Email.ValueString(),
			Access: access,
		},
		ResourceVersion:  currentUser.GetUser().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update user", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update user", err.Error())
		return
	}

	user, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get user after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateUserModelFromSpec(ctx, &plan, user.User)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentUser, err := r.client.CloudService().GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get current user status", err.Error())
		return
	}

	// API will not allow deletion of account owners. Remove from state, add a warning, but don't attempt the API call.
	if currentUser.GetUser() != nil && currentUser.GetUser().GetSpec().GetAccess().GetAccountAccess().GetRole() == identityv1.AccountAccess_ROLE_OWNER {
		resp.Diagnostics.AddWarning(
			"Delete Ignored",
			"The Temporal Cloud API does not support deleting an account owner. Terraform will silently drop this resource but will not delete the Temporal Cloud User.",
		)

		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteUser(ctx, &cloudservicev1.DeleteUserRequest{
		UserId:           state.ID.ValueString(),
		ResourceVersion:  currentUser.GetUser().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "User Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete user", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete user", err.Error())
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getNamespaceAccessesFromModel(ctx context.Context, model *userResourceModel) (map[string]*identityv1.NamespaceAccess, diag.Diagnostics) {
	var diags diag.Diagnostics

	elements := make([]types.Object, 0, len(model.NamespaceAccesses.Elements()))
	diags.Append(model.NamespaceAccesses.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(elements) == 0 {
		return nil, diags
	}

	namespaceAccesses := make(map[string]*identityv1.NamespaceAccess, len(elements))
	for _, access := range elements {
		var model userNamespaceAccessModel
		diags.Append(access.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		permission, err := enums.ToNamespaceAccessPermission(model.Permission.ValueString())
		if err != nil {
			diags.AddError("Failed to convert namespace permission", err.Error())
			return nil, diags
		}
		namespaceAccesses[model.NamespaceID.ValueString()] = &identityv1.NamespaceAccess{
			Permission: permission,
		}
	}

	return namespaceAccesses, diags
}

func updateUserModelFromSpec(ctx context.Context, state *userResourceModel, user *identityv1.User) diag.Diagnostics {
	var diags diag.Diagnostics
	state.ID = types.StringValue(user.GetId())
	stateStr, err := enums.FromResourceState(user.GetState())
	if err != nil {
		diags.AddError("Failed to convert resource state", err.Error())
		return diags
	}
	state.State = types.StringValue(stateStr)
	state.Email = types.StringValue(user.GetSpec().GetEmail())
	role, err := enums.FromAccountAccessRole(user.GetSpec().GetAccess().GetAccountAccess().GetRole())
	if err != nil {
		diags.AddError("Failed to convert account access role", err.Error())
		return diags
	}
	state.AccountAccess = internaltypes.CaseInsensitiveString(role)

	namespaceAccesses := types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs})
	if len(user.GetSpec().GetAccess().GetNamespaceAccesses()) > 0 {
		namespaceAccessObjects := make([]types.Object, 0)
		for ns, namespaceAccess := range user.GetSpec().GetAccess().GetNamespaceAccesses() {
			permission, err := enums.FromNamespaceAccessPermission(namespaceAccess.GetPermission())
			if err != nil {
				diags.AddError("Failed to convert namespace access permission", err.Error())
				return diags
			}
			model := userNamespaceAccessModel{
				NamespaceID: types.StringValue(ns),
				Permission:  internaltypes.CaseInsensitiveString(permission),
			}
			obj, d := types.ObjectValueFrom(ctx, userNamespaceAccessAttrs, model)
			diags.Append(d...)
			if diags.HasError() {
				return diags
			}
			namespaceAccessObjects = append(namespaceAccessObjects, obj)
		}

		accesses, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: userNamespaceAccessAttrs}, namespaceAccessObjects)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		namespaceAccesses = accesses
	}
	state.NamespaceAccesses = namespaceAccesses

	return diags
}
