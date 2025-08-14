package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type (
	namespaceTagsResource struct {
		client *client.Client
	}

	namespaceTagsModel struct {
		ID          types.String `tfsdk:"id"`
		NamespaceID types.String `tfsdk:"namespace_id"`
		Tags        types.Map    `tfsdk:"tags"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*namespaceTagsResource)(nil)
	_ resource.ResourceWithConfigure   = (*namespaceTagsResource)(nil)
	_ resource.ResourceWithImportState = (*namespaceTagsResource)(nil)
)

var idFmt = "namespace/%s/tags"

func NewNamespaceTagsResource() resource.Resource {
	return &namespaceTagsResource{}
}

func (r *namespaceTagsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *namespaceTagsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace_tags"
}

func (r *namespaceTagsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the complete set of tags for a Temporal Cloud namespace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of this namespace tags resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace_id": schema.StringAttribute{
				Description: "The ID of the namespace to manage tags for.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tags": schema.MapAttribute{
				Description: "A map of tag keys to tag values.",
				ElementType: types.StringType,
				Required:    true,
				Validators: []validator.Map{
					mapvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func (r *namespaceTagsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceTagsModel
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

	existingTags, err := getNamespaceTags(ctx, r.client, plan.NamespaceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current namespace tags", err.Error())
		return
	}

	plannedTags, d := getTagsFromMap(ctx, plan.Tags)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	err = r.setNamespaceTags(ctx, plan.NamespaceID.ValueString(), existingTags, plannedTags)
	if err != nil {
		resp.Diagnostics.AddError("Failed to set namespace tags", err.Error())
		return
	}

	resp.Diagnostics.Append(updateTagsModelFromNamespace(ctx, &plan, plan.NamespaceID.ValueString(), plannedTags)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *namespaceTagsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceTagsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tags, err := getNamespaceTags(ctx, r.client, state.NamespaceID.ValueString())
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace resource not found, removing from state", map[string]interface{}{
				"namespace_id": state.NamespaceID.ValueString(),
			})

			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get namespace tags", err.Error())
		return
	}

	resp.Diagnostics.Append(updateTagsModelFromNamespace(ctx, &state, state.NamespaceID.ValueString(), tags)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *namespaceTagsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan namespaceTagsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tags, err := getNamespaceTags(ctx, r.client, plan.NamespaceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current namespace tags", err.Error())
		return
	}

	plannedTags, d := getTagsFromMap(ctx, plan.Tags)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	err = r.setNamespaceTags(ctx, plan.NamespaceID.ValueString(), tags, plannedTags)
	if err != nil {
		resp.Diagnostics.AddError("Failed to set namespace tags", err.Error())
		return
	}

	resp.Diagnostics.Append(updateTagsModelFromNamespace(ctx, &plan, plan.NamespaceID.ValueString(), plannedTags)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *namespaceTagsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state namespaceTagsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout*2)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existingTags, d := getTagsFromMap(ctx, state.Tags)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	err := r.setNamespaceTags(ctx, state.NamespaceID.ValueString(), existingTags, map[string]string{})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace resource not found, removing from state", map[string]interface{}{
				"namespace_id": state.NamespaceID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete namespace tags", err.Error())
		return
	}
}

func (r *namespaceTagsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("namespace_id"), req, resp)
}

func (r *namespaceTagsResource) setNamespaceTags(ctx context.Context, namespaceID string, existing, planned map[string]string) error {
	added, removed, modified := internaltypes.MapDiff(existing, planned)

	// combine added and modified (api overwrites values for existing keys)
	tagsToAdd := make(map[string]string)
	for k, v := range added {
		tagsToAdd[k] = v
	}
	for k, v := range modified {
		tagsToAdd[k] = v
	}

	// extract keys from removed map
	tagsToRemove := make([]string, 0, len(removed))
	for k := range removed {
		tagsToRemove = append(tagsToRemove, k)
	}

	resp, err := r.client.CloudService().UpdateNamespaceTags(ctx, &cloudservicev1.UpdateNamespaceTagsRequest{
		Namespace:    namespaceID,
		TagsToAdd:    tagsToAdd,
		TagsToRemove: tagsToRemove,
	})
	if err != nil {
		return err
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, resp.GetAsyncOperation()); err != nil {
		return err
	}

	return nil
}

func getNamespaceTags(ctx context.Context, client *client.Client, namespaceID string) (map[string]string, error) {
	ns, err := client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: namespaceID,
	})
	if err != nil {
		return nil, err
	}

	return ns.GetNamespace().GetTags(), nil
}

func getTagsFromMap(ctx context.Context, m types.Map) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags := make(map[string]string)
	diags.Append(m.ElementsAs(ctx, &tags, false)...)
	if diags.HasError() {
		return nil, diags
	}

	return tags, diags
}

func updateTagsModelFromNamespace(ctx context.Context, state *namespaceTagsModel, namespaceID string, tags map[string]string) diag.Diagnostics {
	var diags diag.Diagnostics
	state.ID = types.StringValue(fmt.Sprintf(idFmt, namespaceID))
	state.NamespaceID = types.StringValue(namespaceID)
	tagsMap := types.MapNull(types.StringType)
	if len(tags) > 0 {
		t, d := types.MapValueFrom(ctx, types.StringType, tags)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		tagsMap = t
	}
	state.Tags = tagsMap

	return diags
}
