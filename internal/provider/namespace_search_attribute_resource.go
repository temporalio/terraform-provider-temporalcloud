package provider

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jpillora/maplock"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	namespacev1 "go.temporal.io/api/cloud/namespace/v1"
)

type (
	namespaceSearchAttributeResource struct {
		client *client.Client
	}

	namespaceSearchAttributeModel struct {
		ID          types.String                             `tfsdk:"id"`
		NamespaceID types.String                             `tfsdk:"namespace_id"`
		Name        types.String                             `tfsdk:"name"`
		Type        internaltypes.CaseInsensitiveStringValue `tfsdk:"type"`
	}
)

var (
	_ resource.Resource                = (*namespaceSearchAttributeResource)(nil)
	_ resource.ResourceWithConfigure   = (*namespaceSearchAttributeResource)(nil)
	_ resource.ResourceWithImportState = (*namespaceSearchAttributeResource)(nil)

	// namespaceLocks is a per-namespace mutex that protects against concurrent updates to the same namespace spec,
	// which can happen when we are modifying multiple search attributes in parallel.
	namespaceLocks = maplock.New()
)

func NewNamespaceSearchAttributeResource() resource.Resource {
	return &namespaceSearchAttributeResource{}
}

func (r *namespaceSearchAttributeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *namespaceSearchAttributeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace_search_attribute"
}

func (r *namespaceSearchAttributeResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Create a [search attribute](https://docs.temporal.io/visibility#search-attribute) in a Temporal Cloud [namespace](https://registry.terraform.io/providers/temporalio/temporalcloud/latest/docs/resources/namespace). Note the limits on [quantity](https://docs.temporal.io/cloud/limits#number-of-custom-search-attributes) and [naming](https://docs.temporal.io/cloud/limits#custom-search-attribute-names).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of this search attribute.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace_id": schema.StringAttribute{
				Description: "The ID of the namespace to which this search attribute belongs.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the search attribute.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				CustomType:    internaltypes.CaseInsensitiveStringType{},
				Description:   "The type of the search attribute. Must be one of `bool`, `datetime`, `double`, `int`, `keyword`, `keyword_list` or `text`. (case-insensitive)",
				Required:      true,
				PlanModifiers: []planmodifier.String{newSearchAttrTypePlanModifier()},
			},
		},
	}
}

func (r *namespaceSearchAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceSearchAttributeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	withNamespaceLock(plan.NamespaceID.ValueString(), func() {
		ns, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
			Namespace: plan.NamespaceID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to get namespace", err.Error())
			return
		}

		spec := ns.GetNamespace().GetSpec()
		if spec.GetSearchAttributes() == nil {
			spec.SearchAttributes = make(map[string]namespacev1.NamespaceSpec_SearchAttributeType)
		}
		if _, present := spec.GetSearchAttributes()[plan.Name.ValueString()]; present {
			resp.Diagnostics.AddError(
				"Search attribute already exists",
				fmt.Sprintf("Search attribute with name `%s` already exists on namespace `%s`", plan.Name.ValueString(), plan.NamespaceID.ValueString()),
			)
			return
		}

		saType, err := enums.ToNamespaceSearchAttribute(plan.Type.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid search attribute type", err.Error())
			return
		}

		spec.GetSearchAttributes()[plan.Name.ValueString()] = saType
		svcResp, err := r.client.CloudService().UpdateNamespace(ctx, &cloudservicev1.UpdateNamespaceRequest{
			Namespace:        plan.NamespaceID.ValueString(),
			Spec:             spec,
			ResourceVersion:  ns.GetNamespace().GetResourceVersion(),
			AsyncOperationId: uuid.New().String(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to update namespace", err.Error())
			return
		}

		if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
			resp.Diagnostics.AddError("Failed to update namespace", err.Error())
			return
		}
	})
	if resp.Diagnostics.HasError() {
		return
	}

	updatedNs, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: plan.NamespaceID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after update", err.Error())
		return
	}

	plan.ID = types.StringValue(updatedNs.GetNamespace().GetNamespace() + "/" + plan.Name.ValueString())
	plan.NamespaceID = types.StringValue(updatedNs.GetNamespace().GetNamespace())
	resp.Diagnostics.Append(plan.updateFromSpec(updatedNs.GetNamespace().GetSpec())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *namespaceSearchAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceSearchAttributeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: state.NamespaceID.ValueString(),
	})
	if err != nil {
		switch client.StatusCode(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace Search Attribute Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	resp.Diagnostics.Append(state.updateFromSpec(model.GetNamespace().GetSpec())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *namespaceSearchAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state namespaceSearchAttributeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	withNamespaceLock(plan.NamespaceID.ValueString(), func() {
		ns, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
			Namespace: plan.NamespaceID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to get namespace after update", err.Error())
			return
		}

		if !plan.Name.Equal(state.Name) {
			svcResp, err := r.client.CloudService().RenameCustomSearchAttribute(ctx, &cloudservicev1.RenameCustomSearchAttributeRequest{
				Namespace:                         plan.NamespaceID.ValueString(),
				ExistingCustomSearchAttributeName: state.Name.ValueString(),
				NewCustomSearchAttributeName:      plan.Name.ValueString(),
				ResourceVersion:                   ns.GetNamespace().GetResourceVersion(),
			})
			if err != nil {
				resp.Diagnostics.AddError("Failed to rename search attribute", err.Error())
				return
			}

			if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
				resp.Diagnostics.AddError("Failed to rename search attribute", err.Error())
				return
			}
		}

		spec := ns.GetNamespace().GetSpec()
		saType, err := enums.ToNamespaceSearchAttribute(plan.Type.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Invalid search attribute type", err.Error())
			return
		}
		// Assumption: a search attribute named plan.Name already exists
		spec.GetSearchAttributes()[plan.Name.ValueString()] = saType
		svcResp, err := r.client.CloudService().UpdateNamespace(ctx, &cloudservicev1.UpdateNamespaceRequest{
			Namespace:        plan.NamespaceID.ValueString(),
			Spec:             spec,
			ResourceVersion:  ns.GetNamespace().GetResourceVersion(),
			AsyncOperationId: uuid.New().String(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to update namespace", err.Error())
			return
		}

		if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
			resp.Diagnostics.AddError("Failed to update namespace", err.Error())
			return
		}

		updatedNs, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
			Namespace: plan.NamespaceID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to get namespace after update", err.Error())
			return
		}

		resp.Diagnostics.Append(plan.updateFromSpec(updatedNs.GetNamespace().GetSpec())...)
		if resp.Diagnostics.HasError() {
			return
		}
		// plan.ID does not change
		// plan.NamespaceID does not change
		// plan.Name is already set
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	})
}

func (r *namespaceSearchAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Delete Ignored",
		"The Temporal Cloud API does not support deleting a search attribute. Terraform will silently drop this resource but will not delete it from the Temporal Cloud namespace.",
	)
}

func (r *namespaceSearchAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	components := strings.Split(req.ID, "/")
	if len(components) != 2 {
		resp.Diagnostics.AddError("Invalid import ID for Namespace search attribute", "The import ID must be in the format `NamepaceID/SearchAttributeName`, such as `yournamespace.deadbeef/CustomSearchAttribute`")
		return
	}

	nsID, saName := components[0], components[1]
	var state namespaceSearchAttributeModel
	state.ID = types.StringValue(req.ID)
	state.NamespaceID = types.StringValue(nsID)
	state.Name = types.StringValue(saName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (m *namespaceSearchAttributeModel) updateFromSpec(spec *namespacev1.NamespaceSpec) diag.Diagnostics {
	var diags diag.Diagnostics
	newCSA := spec.GetSearchAttributes()
	searchAttrType, ok := newCSA[m.Name.ValueString()]
	if !ok {
		diags.AddError(
			"Failed to find search attribute",
			fmt.Sprintf("Failed to find search attribute `%s` after update (this is a bug, please report this on GitHub!)", m.Name.ValueString()),
		)
		return diags
	}

	// plan.ID is already set
	// plan.NamespaceID is already set
	// plan.Name is already set
	saTypeStr, err := enums.FromNamespaceSearchAttribute(searchAttrType)
	if err != nil {
		diags.AddError("Failed to convert search attribute type", err.Error())
		return diags
	}
	m.Type = internaltypes.CaseInsensitiveString(saTypeStr)
	return diags
}

// withNamespaceLock locks the given namespace and runs the given function, releasing the lock once the function returns.
func withNamespaceLock(ns string, f func()) {
	namespaceLocks.Lock(ns)
	defer func() {
		_ = namespaceLocks.Unlock(ns)
	}()
	f()
}

func newSearchAttrTypePlanModifier() planmodifier.String {
	return &searchAttrTypePlanModifier{}
}

type searchAttrTypePlanModifier struct {
}

// Description returns a human-readable description of the plan modifier.
func (m searchAttrTypePlanModifier) Description(_ context.Context) string {
	return "If the value of the search attribute changes (case-insensitive), update the resource accordingly."
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m searchAttrTypePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

// PlanModifyString implements the plan modification logic.
func (m searchAttrTypePlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() {
		// Its a create operation, no need to update the plan.
		return
	}
	if req.Plan.Raw.IsNull() {
		// Its a delete operation, no need to update the plan.
		return
	}

	saTypePlan, err := enums.ToNamespaceSearchAttribute(req.PlanValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse search attribute type in plan", err.Error())
		return
	}
	saTypeState, err := enums.ToNamespaceSearchAttribute(req.StateValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse search attribute type in state", err.Error())
		return
	}

	if saTypePlan == saTypeState {
		// The state and the plan values are equal.
		// No need to update the resource, update the response to the same as the one in the state to avoid an update.
		resp.PlanValue = req.StateValue
		return
	}
	// Its a change in the value, we don't allow changing the search attribute type, error out
	resp.Diagnostics.AddError("Search attribute type change not allowed", "Changing the search attribute type is not allowed.")
}
