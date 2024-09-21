package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	identityv1 "go.temporal.io/api/cloud/identity/v1"
)

type (
	userResource struct {
		client cloudservicev1.CloudServiceClient
	}

	userResourceModel struct {
		ID                types.String                             `tfsdk:"id"`
		State             types.String                             `tfsdk:"state"`
		Email             types.String                             `tfsdk:"email"`
		AccountAccess     internaltypes.CaseInsensitiveStringValue `tfsdk:"account_access"`
		NamespaceAccesses types.List                               `tfsdk:"namespace_accesses"`

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

	client, ok := req.ProviderData.(cloudservicev1.CloudServiceClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected cloudservicev1.CloudServiceClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
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
				Description: "The role on the account. Must be one of [admin, developer, read] (case-insensitive)",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive("admin", "developer", "read"),
				},
			},
			"namespace_accesses": schema.ListNestedAttribute{
				Description: "The list of namespace accesses.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"namespace_id": schema.StringAttribute{
							Description: "The namespace to assign permissions to.",
							Required:    true,
						},
						"permission": schema.StringAttribute{
							CustomType:  internaltypes.CaseInsensitiveStringType{},
							Description: "The permission to assign. Must be one of [admin, write, read] (case-insensitive)",
							Required:    true,
							Validators: []validator.String{
								stringvalidator.OneOfCaseInsensitive("admin", "write", "read"),
							},
						},
					},
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

	namespaceAccesses := getNamespaceAccessesFromModel(ctx, resp.Diagnostics, &plan)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CreateUser(ctx, &cloudservicev1.CreateUserRequest{
		Spec: &identityv1.UserSpec{
			Email: plan.Email.ValueString(),
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: plan.AccountAccess.ValueString(),
				},
				NamespaceAccesses: namespaceAccesses,
			},
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create user", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create user", err.Error())
		return
	}

	user, err := r.client.GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: svcResp.UserId,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get user after creation", err.Error())
		return
	}

	updateUserModelFromSpec(ctx, resp.Diagnostics, &plan, user.User)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get user", err.Error())
		return
	}

	updateUserModelFromSpec(ctx, resp.Diagnostics, &state, user.User)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespaceAccesses := getNamespaceAccessesFromModel(ctx, resp.Diagnostics, &plan)
	if resp.Diagnostics.HasError() {
		return
	}

	currentUser, err := r.client.GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current user status", err.Error())
		return
	}

	svcResp, err := r.client.UpdateUser(ctx, &cloudservicev1.UpdateUserRequest{
		UserId: plan.ID.ValueString(),
		Spec: &identityv1.UserSpec{
			Email: plan.Email.ValueString(),
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: plan.AccountAccess.ValueString(),
				},
				NamespaceAccesses: namespaceAccesses,
			},
		},
		ResourceVersion: currentUser.GetUser().GetResourceVersion(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update user", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update user", err.Error())
		return
	}

	user, err := r.client.GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get user after update", err.Error())
		return
	}

	updateUserModelFromSpec(ctx, resp.Diagnostics, &plan, user.User)
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

	currentUser, err := r.client.GetUser(ctx, &cloudservicev1.GetUserRequest{
		UserId: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current user status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.DeleteUser(ctx, &cloudservicev1.DeleteUserRequest{
		UserId:          state.ID.ValueString(),
		ResourceVersion: currentUser.GetUser().GetResourceVersion(),
	})
	if err != nil {
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

func getNamespaceAccessesFromModel(ctx context.Context, diags diag.Diagnostics, model *userResourceModel) map[string]*identityv1.NamespaceAccess {
	elements := make([]types.Object, 0, len(model.NamespaceAccesses.Elements()))
	diags.Append(model.NamespaceAccesses.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil
	}

	if len(elements) == 0 {
		return nil
	}

	namespaceAccesses := make(map[string]*identityv1.NamespaceAccess, len(elements))
	for _, access := range elements {
		var model userNamespaceAccessModel
		diags.Append(access.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil
		}
		namespaceAccesses[model.NamespaceID.ValueString()] = &identityv1.NamespaceAccess{
			Permission: model.Permission.ValueString(),
		}
	}

	return namespaceAccesses
}

func updateUserModelFromSpec(ctx context.Context, diags diag.Diagnostics, state *userResourceModel, user *identityv1.User) {
	state.ID = types.StringValue(user.GetId())
	state.State = types.StringValue(user.GetState())
	state.Email = types.StringValue(user.GetSpec().GetEmail())
	state.AccountAccess = internaltypes.CaseInsensitiveString(user.GetSpec().GetAccess().GetAccountAccess().GetRole())

	namespaceAccesses := types.ListNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs})
	if len(user.GetSpec().GetAccess().GetNamespaceAccesses()) > 0 {
		namespaceAccessObjects := make([]types.Object, 0)
		for ns, namespaceAccess := range user.GetSpec().GetAccess().GetNamespaceAccesses() {
			model := userNamespaceAccessModel{
				NamespaceID: types.StringValue(ns),
				Permission:  internaltypes.CaseInsensitiveString(namespaceAccess.GetPermission()),
			}
			obj, d := types.ObjectValueFrom(ctx, userNamespaceAccessAttrs, model)
			diags.Append(d...)
			if diags.HasError() {
				return
			}
			namespaceAccessObjects = append(namespaceAccessObjects, obj)
		}

		accesses, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: namespaceCertificateFilterAttrs}, namespaceAccessObjects)
		diags.Append(d...)
		if diags.HasError() {
			return
		}

		namespaceAccesses = accesses
	}
	state.NamespaceAccesses = namespaceAccesses
}
