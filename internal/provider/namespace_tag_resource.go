package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

type (
	namespaceTagResource struct {
		client *client.Client
	}

	namespaceTagModel struct {
		ID          types.String `tfsdk:"id"`
		NamespaceID types.String `tfsdk:"namespace_id"`
		Key         types.String `tfsdk:"key"`
		Value       types.String `tfsdk:"value"`
	}
)

var (
	_ resource.Resource                = (*namespaceTagResource)(nil)
	_ resource.ResourceWithConfigure   = (*namespaceTagResource)(nil)
	_ resource.ResourceWithImportState = (*namespaceTagResource)(nil)
)

func NewNamespaceTagResource() resource.Resource {
	return &namespaceTagResource{}
}

func (r *namespaceTagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *namespaceTagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace_tag"
}

func (r *namespaceTagResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single tag on a Temporal Cloud [namespace](https://registry.terraform.io/providers/temporalio/temporalcloud/latest/docs/resources/namespace). Use multiple instances of this resource to manage multiple tags on the same namespace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of this namespace tag resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace_id": schema.StringAttribute{
				Description: "The ID of the namespace to tag.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key": schema.StringAttribute{
				Description: "The tag key.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				Description: "The tag value.",
				Required:    true,
			},
		},
	}
}

func (r *namespaceTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceTagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateNamespaceTags(ctx, &cloudservicev1.UpdateNamespaceTagsRequest{
		Namespace: plan.NamespaceID.ValueString(),
		TagsToAdd: map[string]string{
			plan.Key.ValueString(): plan.Value.ValueString(),
		},
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create namespace tag", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to create namespace tag", err.Error())
		return
	}

	updatedNs, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: plan.NamespaceID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after tag creation", err.Error())
		return
	}

	plan.ID = types.StringValue(updatedNs.GetNamespace().GetNamespace() + "/" + plan.Key.ValueString())
	plan.NamespaceID = types.StringValue(updatedNs.GetNamespace().GetNamespace())

	tags := updatedNs.GetNamespace().GetTags()
	tagValue, exists := tags[plan.Key.ValueString()]
	if !exists {
		resp.Diagnostics.AddError(
			"Tag not found after creation",
			"This should not happen - please report as a bug",
		)
		return
	}
	plan.Value = types.StringValue(tagValue)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *namespaceTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceTagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: state.NamespaceID.ValueString(),
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace not found, removing tag from state", map[string]interface{}{
				"namespace_id": state.NamespaceID.ValueString(),
				"tag_key":      state.Key.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	tags := ns.GetNamespace().GetTags()
	if tagValue, exists := tags[state.Key.ValueString()]; !exists {
		tflog.Warn(ctx, "Tag not found on namespace, removing from state", map[string]interface{}{
			"namespace_id": state.NamespaceID.ValueString(),
			"tag_key":      state.Key.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	} else {
		// Update the value in case it was changed outside Terraform
		state.Value = types.StringValue(tagValue)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *namespaceTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state namespaceTagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateNamespaceTags(ctx, &cloudservicev1.UpdateNamespaceTagsRequest{
		Namespace: plan.NamespaceID.ValueString(),
		TagsToAdd: map[string]string{
			plan.Key.ValueString(): plan.Value.ValueString(),
		},
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update namespace tag", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update namespace tag", err.Error())
		return
	}

	updatedNs, err := r.client.CloudService().GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: plan.NamespaceID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to verify namespace tag update", err.Error())
		return
	}

	if tagValue, exists := updatedNs.GetNamespace().GetTags()[plan.Key.ValueString()]; !exists {
		resp.Diagnostics.AddError(
			"Tag not found after update",
			fmt.Sprintf("Tag with key '%s' was not found after update", plan.Key.ValueString()),
		)
		return
	} else {
		plan.Value = types.StringValue(tagValue)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *namespaceTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state namespaceTagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateNamespaceTags(ctx, &cloudservicev1.UpdateNamespaceTagsRequest{
		Namespace:        state.NamespaceID.ValueString(),
		TagsToRemove:     []string{state.Key.ValueString()},
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			tflog.Warn(ctx, "Namespace not found during tag deletion", map[string]interface{}{
				"namespace_id": state.NamespaceID.ValueString(),
				"tag_key":      state.Key.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError("Failed to delete namespace tag", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace tag", err.Error())
		return
	}
}

func (r *namespaceTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	components := strings.Split(req.ID, "/")
	if len(components) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Import ID must be in the format 'NamespaceID/TagKey', for example: 'yournamespace.deadbeef/CustomTagKey'",
		)
		return
	}

	nsID, tagKey := components[0], components[1]
	var state namespaceTagModel
	state.ID = types.StringValue(req.ID)
	state.NamespaceID = types.StringValue(nsID)
	state.Key = types.StringValue(tagKey)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
