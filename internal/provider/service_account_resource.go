package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"

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
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	identityv1 "go.temporal.io/api/cloud/identity/v1"
)

type (
	serviceAccountResource struct {
		client *client.Client
	}

	serviceAccountResourceModel struct {
		ID                types.String                             `tfsdk:"id"`
		State             types.String                             `tfsdk:"state"`
		Name              types.String                             `tfsdk:"name"`
		AccountAccess     internaltypes.CaseInsensitiveStringValue `tfsdk:"account_access"`
		NamespaceAccesses types.List                               `tfsdk:"namespace_accesses"`

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
		Description: "Provisions a Temporal Cloud Service Account. To prevent overwriting, include each Service Account's Temporal configuration in one and only one Terraform file.",
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
			"account_access": schema.StringAttribute{
				CustomType:  internaltypes.CaseInsensitiveStringType{},
				Description: "The role on the account. Must be one of [admin, developer, read] (case-insensitive)",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive("admin", "developer", "read"),
				},
			},
			"namespace_accesses": schema.ListNestedAttribute{
				Description: "The list of namespace accesses. Empty lists are not allowed, omit the attribute instead.",
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
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
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

	namespaceAccesses := getNamespaceAccessesFromServiceAccountModel(ctx, resp.Diagnostics, &plan)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := enums.ToAccountAccessRole(plan.AccountAccess.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert account access role", err.Error())
		return
	}
	svcResp, err := r.client.CloudService().CreateServiceAccount(ctx, &cloudservicev1.CreateServiceAccountRequest{
		Spec: &identityv1.ServiceAccountSpec{
			Name: plan.Name.ValueString(),
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: role,
				},
				NamespaceAccesses: namespaceAccesses,
			},
		},
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

	namespaceAccesses := getNamespaceAccessesFromServiceAccountModel(ctx, resp.Diagnostics, &plan)
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

	role, err := enums.ToAccountAccessRole(plan.AccountAccess.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to convert account access role", err.Error())
		return
	}
	svcResp, err := r.client.CloudService().UpdateServiceAccount(ctx, &cloudservicev1.UpdateServiceAccountRequest{
		ServiceAccountId: plan.ID.ValueString(),
		Spec: &identityv1.ServiceAccountSpec{
			Name: plan.Name.ValueString(),
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: role,
				},
				NamespaceAccesses: namespaceAccesses,
			},
		},
		ResourceVersion: currentServiceAccount.ServiceAccount.GetResourceVersion(),
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
		resp.Diagnostics.AddError("Failed to get current Service Account status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteServiceAccount(ctx, &cloudservicev1.DeleteServiceAccountRequest{
		ServiceAccountId: state.ID.ValueString(),
		ResourceVersion:  currentServiceAccount.ServiceAccount.GetResourceVersion(),
	})
	if err != nil {
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

func getNamespaceAccessesFromServiceAccountModel(ctx context.Context, diags diag.Diagnostics, model *serviceAccountResourceModel) map[string]*identityv1.NamespaceAccess {
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
		var model serviceAccountNamespaceAccessModel
		diags.Append(access.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil
		}
		persmission, err := enums.ToNamespaceAccessPermission(model.Permission.ValueString())
		if err != nil {
			diags.AddError("Failed to convert namespace access permission", err.Error())
			return nil
		}
		namespaceAccesses[model.NamespaceID.ValueString()] = &identityv1.NamespaceAccess{
			Permission: persmission,
		}
	}

	return namespaceAccesses
}

func updateServiceAccountModelFromSpec(ctx context.Context, state *serviceAccountResourceModel, serviceAccount *identityv1.ServiceAccount) diag.Diagnostics {
	var diags diag.Diagnostics
	stateStr, err := enums.FromResourceState(serviceAccount.GetState())
	if err != nil {
		diags.AddError("Failed to convert resource state", err.Error())
	}
	role, err := enums.FromAccountAccessRole(serviceAccount.GetSpec().GetAccess().GetAccountAccess().GetRole())
	if err != nil {
		diags.AddError("Failed to convert account access role", err.Error())
	}

	namespaceAccesses := types.ListNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs})
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

		accesses, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}, namespaceAccessObjects)
		diags.Append(d...)
		if !diags.HasError() {
			namespaceAccesses = accesses
		}
	}

	if diags.HasError() {
		return diags
	}

	state.ID = types.StringValue(serviceAccount.GetId())
	state.State = types.StringValue(stateStr)
	state.Name = types.StringValue(serviceAccount.GetSpec().GetName())
	state.AccountAccess = internaltypes.CaseInsensitiveString(role)
	state.NamespaceAccesses = namespaceAccesses

	return nil
}
