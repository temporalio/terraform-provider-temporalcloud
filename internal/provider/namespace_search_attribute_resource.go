package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jpillora/maplock"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	cloudservicev1 "github.com/temporalio/terraform-provider-temporalcloud/proto/go/temporal/api/cloud/cloudservice/v1"
)

type (
	namespaceSearchAttributeResource struct {
		client cloudservicev1.CloudServiceClient
	}

	namespaceSearchAttributeModel struct {
		ID          types.String `tfsdk:"id"`
		NamespaceID types.String `tfsdk:"namespace_id"`
		Name        types.String `tfsdk:"name"`
		Type        types.String `tfsdk:"type"`
	}
)

var (
	_ resource.Resource              = (*namespaceSearchAttributeResource)(nil)
	_ resource.ResourceWithConfigure = (*namespaceSearchAttributeResource)(nil)

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

func (r *namespaceSearchAttributeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace_search_attribute"
}

func (r *namespaceSearchAttributeResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of this search attribute",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace_id": schema.StringAttribute{
				Description: "The ID of the namespace to which this search attribute belongs",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the search attribute",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of the search attribute",
				Required:    true,
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
		ns, err := r.client.GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
			Namespace: plan.NamespaceID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to get namespace", err.Error())
			return
		}

		spec := ns.GetNamespace().GetSpec()
		if spec.GetCustomSearchAttributes() == nil {
			spec.CustomSearchAttributes = make(map[string]string)
		}
		if _, present := spec.GetCustomSearchAttributes()[plan.Name.ValueString()]; present {
			resp.Diagnostics.AddError(
				"Search attribute already exists",
				fmt.Sprintf("Search attribute with name `%s` already exists on namespace `%s`", plan.Name.ValueString(), plan.NamespaceID.ValueString()),
			)
			return
		}

		spec.GetCustomSearchAttributes()[plan.Name.ValueString()] = plan.Type.ValueString()
		svcResp, err := r.client.UpdateNamespace(ctx, &cloudservicev1.UpdateNamespaceRequest{
			Namespace:       plan.NamespaceID.ValueString(),
			Spec:            spec,
			ResourceVersion: ns.GetNamespace().GetResourceVersion(),
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

	updatedNs, err := r.client.GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: plan.NamespaceID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after update", err.Error())
		return
	}

	id, err := uuid.GenerateUUID()
	if err != nil {
		resp.Diagnostics.AddError("Failed to generate UUID", err.Error())
		return
	}

	newCSA := updatedNs.GetNamespace().GetSpec().GetCustomSearchAttributes()
	newSearchAttrType, ok := newCSA[plan.Name.ValueString()]
	if !ok {
		resp.Diagnostics.AddError(
			"Failed to find newly-created search attribute",
			fmt.Sprintf("Failed to find search attribute `%s` after update (this is a bug, please report this on GitHub!)", plan.Name.ValueString()),
		)
		return
	}
	plan.ID = types.StringValue(id)
	plan.NamespaceID = types.StringValue(updatedNs.GetNamespace().GetNamespace())
	// plan.Name is already set
	plan.Type = types.StringValue(newSearchAttrType)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *namespaceSearchAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// TODO: NYI
}

func (r *namespaceSearchAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// TODO: NYI
}

func (r *namespaceSearchAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// TODO: NYI
}

// withNamespaceLock locks the given namespace and runs the given function, releasing the lock once the function returns.
func withNamespaceLock(ns string, f func()) {
	namespaceLocks.Lock(ns)
	defer namespaceLocks.Unlock(ns)
	f()
}
