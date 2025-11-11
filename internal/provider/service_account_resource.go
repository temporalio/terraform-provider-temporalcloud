package provider

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/validation"
)

type (
	serviceAccountResource struct {
		client *client.Client
	}

	serviceAccountResourceModel struct {
		ID                    types.String                             `tfsdk:"id"`
		State                 types.String                             `tfsdk:"state"`
		Name                  types.String                             `tfsdk:"name"`
		Description           types.String                             `tfsdk:"description"`
		AccountAccess         internaltypes.CaseInsensitiveStringValue `tfsdk:"account_access"`
		NamespaceAccesses     types.Set                                `tfsdk:"namespace_accesses"`
		NamespaceScopedAccess types.Object                             `tfsdk:"namespace_scoped_access"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}

	serviceAccountNamespaceAccessModel struct {
		NamespaceID types.String                             `tfsdk:"namespace_id"`
		Permission  internaltypes.CaseInsensitiveStringValue `tfsdk:"permission"`
	}
)

var (
	_ resource.Resource                = (*serviceAccountResource)(nil)
	_ resource.ResourceWithConfigure   = (*serviceAccountResource)(nil)
	_ resource.ResourceWithImportState = (*serviceAccountResource)(nil)

	serviceAccountNamespaceAccessAttrs = map[string]attr.Type{
		"namespace_id": types.StringType,
		"permission":   internaltypes.CaseInsensitiveStringType{},
	}
)

func NewServiceAccountResource() resource.Resource {
	return &serviceAccountResource{}
}

func (r *serviceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud Service Account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Service Account.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description: "The current state of the Service Account.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name associated with the service account.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "The description for the service account.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"account_access": schema.StringAttribute{
				CustomType:  internaltypes.CaseInsensitiveStringType{},
				Description: "The role on the account. Must be one of admin, developer, or read (case-insensitive). Cannot be set if namespace_scoped_access is provided.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(enums.AllowedAccountAccessRoles()...),
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("namespace_scoped_access"),
					}...),
				},
			},
			"namespace_accesses": schema.SetNestedAttribute{
				Description: "The set of namespace accesses. Empty sets are not allowed, omit the attribute instead. Service Accounts with an account_access role of admin cannot be assigned explicit permissions to namespaces. Admins implicitly receive access to all Namespaces. Cannot be set if namespace_scoped_access is provided.",
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
								stringvalidator.OneOfCaseInsensitive(enums.AllowedNamespaceAccessPermissions()...),
							},
						},
					},
				},
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					validation.NewNamespaceAccessValidator("account_access"),
					validation.SetNestedAttributeMustBeUnique("namespace_id"),
					setvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("namespace_scoped_access"),
					}...),
				},
			},
			"namespace_scoped_access": schema.SingleNestedAttribute{
				Description: "Configures this service account as a namespace-scoped service account with access to only a single namespace. The namespace assignment is immutable after creation. Cannot be set if account_access or namespace_accesses are provided.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"namespace_id": schema.StringAttribute{
						Description: "The namespace to scope this service account to. This field is immutable after creation.",
						Required:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"permission": schema.StringAttribute{
						CustomType:  internaltypes.CaseInsensitiveStringType{},
						Description: "The permission to assign. Must be one of admin, write, or read (case-insensitive). This field is mutable.",
						Required:    true,
						Validators: []validator.String{
							stringvalidator.OneOfCaseInsensitive(enums.AllowedNamespaceAccessPermissions()...),
						},
					},
				},
				Validators: []validator.Object{
					objectvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("account_access"),
						path.MatchRoot("namespace_accesses"),
					}...),
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

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountResourceModel
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

	spec, d := buildServiceAccountSpec(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().CreateServiceAccount(ctx, &cloudservicev1.CreateServiceAccountRequest{
		Spec:             spec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Service Account", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create Service Account", err.Error())
		return
	}

	serviceAccount, err := r.client.CloudService().GetServiceAccount(ctx, &cloudservicev1.GetServiceAccountRequest{
		ServiceAccountId: svcResp.ServiceAccountId,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Service Account after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateServiceAccountModelFromSpec(ctx, &plan, serviceAccount.ServiceAccount)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceAccount, err := r.client.CloudService().GetServiceAccount(ctx, &cloudservicev1.GetServiceAccountRequest{
		ServiceAccountId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Service Account Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get Service Account", err.Error())
		return
	}

	resp.Diagnostics.Append(updateServiceAccountModelFromSpec(ctx, &state, serviceAccount.ServiceAccount)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentServiceAccount, err := r.client.CloudService().GetServiceAccount(ctx, &cloudservicev1.GetServiceAccountRequest{
		ServiceAccountId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current Service Account status", err.Error())
		return
	}

	// Prevent conversion between account-scoped and namespace-scoped service accounts
	currentIsNamespaceScoped := currentServiceAccount.ServiceAccount.GetSpec().GetNamespaceScopedAccess() != nil
	planIsNamespaceScoped := !plan.NamespaceScopedAccess.IsNull()

	if currentIsNamespaceScoped != planIsNamespaceScoped {
		if currentIsNamespaceScoped {
			resp.Diagnostics.AddError(
				"Cannot convert namespace-scoped service account to account-scoped",
				"This service account is currently namespace-scoped and cannot be converted to an account-scoped service account. You must delete and recreate the service account to change its scope type.",
			)
		} else {
			resp.Diagnostics.AddError(
				"Cannot convert account-scoped service account to namespace-scoped",
				"This service account is currently account-scoped and cannot be converted to a namespace-scoped service account. You must delete and recreate the service account to change its scope type.",
			)
		}
		return
	}

	spec, d := buildServiceAccountSpec(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateServiceAccount(ctx, &cloudservicev1.UpdateServiceAccountRequest{
		ServiceAccountId: plan.ID.ValueString(),
		Spec:             spec,
		ResourceVersion:  currentServiceAccount.ServiceAccount.GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Service Account", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update Service Account", err.Error())
		return
	}

	serviceAccount, err := r.client.CloudService().GetServiceAccount(ctx, &cloudservicev1.GetServiceAccountRequest{
		ServiceAccountId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Service Account after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateServiceAccountModelFromSpec(ctx, &plan, serviceAccount.ServiceAccount)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentServiceAccount, err := r.client.CloudService().GetServiceAccount(ctx, &cloudservicev1.GetServiceAccountRequest{
		ServiceAccountId: state.ID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Service Account Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get current Service Account status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteServiceAccount(ctx, &cloudservicev1.DeleteServiceAccountRequest{
		ServiceAccountId: state.ID.ValueString(),
		ResourceVersion:  currentServiceAccount.ServiceAccount.GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Service Account Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete Service Account", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete Service Account", err.Error())
	}
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getNamespaceAccessesFromServiceAccountModel(ctx context.Context, model *serviceAccountResourceModel) (map[string]*identityv1.NamespaceAccess, diag.Diagnostics) {
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
		var model serviceAccountNamespaceAccessModel
		diags.Append(access.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		permission, err := enums.ToNamespaceAccessPermission(model.Permission.ValueString())
		if err != nil {
			diags.AddError("Failed to convert namespace access permission", err.Error())
			return nil, diags
		}
		namespaceAccesses[model.NamespaceID.ValueString()] = &identityv1.NamespaceAccess{
			Permission: permission,
		}
	}

	return namespaceAccesses, diags
}

func buildServiceAccountSpec(ctx context.Context, plan *serviceAccountResourceModel) (*identityv1.ServiceAccountSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	description := ""
	if !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}

	spec := &identityv1.ServiceAccountSpec{
		Name:        plan.Name.ValueString(),
		Description: description,
	}

	// Handle namespace-scoped access
	if !plan.NamespaceScopedAccess.IsNull() {
		namespaceScopedAccess, d := getNamespaceScopedAccessFromModel(ctx, plan)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		spec.NamespaceScopedAccess = namespaceScopedAccess
	} else {
		// Handle account-scoped access
		if plan.AccountAccess.IsNull() {
			diags.AddError("Missing access configuration", "Either account_access or namespace_scoped_access must be provided")
			return nil, diags
		}

		namespaceAccesses, d := getNamespaceAccessesFromServiceAccountModel(ctx, plan)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		role, err := enums.ToAccountAccessRole(plan.AccountAccess.ValueString())
		if err != nil {
			diags.AddError("Failed to convert account access role", err.Error())
			return nil, diags
		}

		spec.Access = &identityv1.Access{
			AccountAccess: &identityv1.AccountAccess{
				Role: role,
			},
			NamespaceAccesses: namespaceAccesses,
		}
	}

	return spec, diags
}

func getNamespaceScopedAccessFromModel(ctx context.Context, model *serviceAccountResourceModel) (*identityv1.NamespaceScopedAccess, diag.Diagnostics) {
	var diags diag.Diagnostics
	var namespaceScopedAccessModel serviceAccountNamespaceAccessModel
	diags.Append(model.NamespaceScopedAccess.As(ctx, &namespaceScopedAccessModel, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	permission, err := enums.ToNamespaceAccessPermission(namespaceScopedAccessModel.Permission.ValueString())
	if err != nil {
		diags.AddError("Failed to convert namespace access permission", err.Error())
		return nil, diags
	}

	return &identityv1.NamespaceScopedAccess{
		Namespace: namespaceScopedAccessModel.NamespaceID.ValueString(),
		Access: &identityv1.NamespaceAccess{
			Permission: permission,
		},
	}, diags
}

func updateServiceAccountModelFromSpec(ctx context.Context, state *serviceAccountResourceModel, serviceAccount *identityv1.ServiceAccount) diag.Diagnostics {
	var diags diag.Diagnostics
	stateStr, err := enums.FromResourceState(serviceAccount.GetState())
	if err != nil {
		diags.AddError("Failed to convert resource state", err.Error())
	}

	state.ID = types.StringValue(serviceAccount.GetId())
	state.State = types.StringValue(stateStr)
	state.Name = types.StringValue(serviceAccount.GetSpec().GetName())
	state.Description = types.StringValue(serviceAccount.GetSpec().GetDescription())

	// Check if this is a namespace-scoped service account
	if serviceAccount.GetSpec().GetNamespaceScopedAccess() != nil {
		namespaceScopedAccess := serviceAccount.GetSpec().GetNamespaceScopedAccess()
		permission, err := enums.FromNamespaceAccessPermission(namespaceScopedAccess.GetAccess().GetPermission())
		if err != nil {
			diags.AddError("Failed to convert namespace access permission", err.Error())
			return diags
		}

		model := serviceAccountNamespaceAccessModel{
			NamespaceID: types.StringValue(namespaceScopedAccess.GetNamespace()),
			Permission:  internaltypes.CaseInsensitiveString(permission),
		}

		obj, d := types.ObjectValueFrom(ctx, serviceAccountNamespaceAccessAttrs, model)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}

		state.NamespaceScopedAccess = obj
		state.AccountAccess = internaltypes.CaseInsensitiveStringValue{}
		state.NamespaceAccesses = types.SetNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs})
	} else {
		// Handle account-scoped service account
		role, err := enums.FromAccountAccessRole(serviceAccount.GetSpec().GetAccess().GetAccountAccess().GetRole())
		if err != nil {
			diags.AddError("Failed to convert account access role", err.Error())
		}

		namespaceAccesses := types.SetNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs})
		if len(serviceAccount.GetSpec().GetAccess().GetNamespaceAccesses()) > 0 {
			namespaceAccessObjects := make([]types.Object, 0)
			for ns, namespaceAccess := range serviceAccount.GetSpec().GetAccess().GetNamespaceAccesses() {
				permission, err := enums.FromNamespaceAccessPermission(namespaceAccess.GetPermission())
				if err != nil {
					diags.AddError("Failed to convert namespace access permission", err.Error())
					continue
				}
				model := serviceAccountNamespaceAccessModel{
					NamespaceID: types.StringValue(ns),
					Permission:  internaltypes.CaseInsensitiveString(permission),
				}
				obj, d := types.ObjectValueFrom(ctx, serviceAccountNamespaceAccessAttrs, model)
				diags.Append(d...)
				if d.HasError() {
					continue
				}
				namespaceAccessObjects = append(namespaceAccessObjects, obj)
			}

			accesses, d := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}, namespaceAccessObjects)
			diags.Append(d...)
			if !diags.HasError() {
				namespaceAccesses = accesses
			}
		}

		if diags.HasError() {
			return diags
		}

		state.AccountAccess = internaltypes.CaseInsensitiveString(role)
		state.NamespaceAccesses = namespaceAccesses
		state.NamespaceScopedAccess = types.ObjectNull(serviceAccountNamespaceAccessAttrs)
	}

	return nil
}
