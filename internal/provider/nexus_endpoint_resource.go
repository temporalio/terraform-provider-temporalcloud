package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	nexusv1 "go.temporal.io/api/cloud/nexus/v1"
)

type (
	nexusEndpointResource struct {
		client *client.Client
	}

	nexusEndpointResourceModel struct {
		ID                      types.String `tfsdk:"id"`
		Name                    types.String `tfsdk:"name"`
		Description             types.String `tfsdk:"description"`
		WorkerTargetSpec        types.Object `tfsdk:"worker_target_spec"`
		AllowedCallerNamespaces types.Set    `tfsdk:"allowed_caller_namespaces"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}

	nexusEndpointWorkerTargetModel struct {
		NamespaceID types.String `tfsdk:"namespace_id"`
		TaskQueue   types.String `tfsdk:"task_queue"`
	}
)

var (
	_ resource.Resource                = (*nexusEndpointResource)(nil)
	_ resource.ResourceWithConfigure   = (*nexusEndpointResource)(nil)
	_ resource.ResourceWithImportState = (*nexusEndpointResource)(nil)

	workerTargetSpecAttrs = map[string]attr.Type{
		"namespace_id": types.StringType,
		"task_queue":   types.StringType,
	}
)

func NewNexusEndpointResource() resource.Resource {
	return &nexusEndpointResource{}
}

func (r *nexusEndpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *nexusEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nexus_endpoint"
}

func (r *nexusEndpointResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud Nexus endpoint.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Nexus endpoint.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the endpoint. Must be unique within an account and match `^[a-zA-Z][a-zA-Z0-9\\-]*[a-zA-Z0-9]$`",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description for the Nexus endpoint.",
				Optional:    true,
				Sensitive:   true,
			},
			"worker_target_spec": schema.SingleNestedAttribute{
				Description: "A target spec for routing nexus requests to a specific cloud namespace worker.",
				Attributes: map[string]schema.Attribute{
					"namespace_id": schema.StringAttribute{
						Description: "The target cloud namespace to route requests to. Namespace must be in same account as the endpoint.",
						Required:    true,
					},
					"task_queue": schema.StringAttribute{
						Description: "The task queue on the cloud namespace to route requests to.",
						Required:    true,
					},
				},
				Required: true,
			},
			"allowed_caller_namespaces": schema.SetAttribute{
				Description: "Namespace(s) that are allowed to call this Endpoint.",
				ElementType: types.StringType,
				Required:    true,
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

func (r *nexusEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nexusEndpointResourceModel
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

	description := ""
	if !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}

	targetSpec, diags := getTargetSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	policySpecs, diags := getPolicySpecsFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().CreateNexusEndpoint(ctx, &cloudservicev1.CreateNexusEndpointRequest{
		Spec: &nexusv1.EndpointSpec{
			Name:        plan.Name.ValueString(),
			Description: description,
			TargetSpec:  targetSpec,
			PolicySpecs: policySpecs,
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create Nexus endpoint", err.Error())
		return
	}
	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create Nexus endpoint", err.Error())
		return
	}

	nexusEndpoint, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: svcResp.GetEndpointId(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Nexus endpoint after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateNexusEndpointModelFromSpec(ctx, &plan, nexusEndpoint.Endpoint)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *nexusEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nexusEndpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nexusEndpoint, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Nexus endpoint", err.Error())
		return
	}

	resp.Diagnostics.Append(updateNexusEndpointModelFromSpec(ctx, &state, nexusEndpoint.Endpoint)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *nexusEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan nexusEndpointResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nexusEndpoint, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current Nexus endpoint status", err.Error())
		return
	}

	description := ""
	if !plan.Description.IsNull() {
		description = plan.Description.ValueString()
	}

	targetSpec, diags := getTargetSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	policySpecs, diags := getPolicySpecsFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateNexusEndpoint(ctx, &cloudservicev1.UpdateNexusEndpointRequest{
		EndpointId: plan.ID.ValueString(),
		Spec: &nexusv1.EndpointSpec{
			Name:        plan.Name.ValueString(),
			Description: description,
			TargetSpec:  targetSpec,
			PolicySpecs: policySpecs,
		},
		ResourceVersion: nexusEndpoint.GetEndpoint().GetResourceVersion(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Nexus endpoint", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.GetAsyncOperation()); err != nil {
		resp.Diagnostics.AddError("Failed to update Nexus endpoint", err.Error())
		return
	}

	nexusEndpoint, err = r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Nexus endpoint after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateNexusEndpointModelFromSpec(ctx, &plan, nexusEndpoint.Endpoint)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *nexusEndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nexusEndpointResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nexusEndpoint, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current Nexus endpoint status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteNexusEndpoint(ctx, &cloudservicev1.DeleteNexusEndpointRequest{
		EndpointId:      state.ID.ValueString(),
		ResourceVersion: nexusEndpoint.GetEndpoint().GetResourceVersion(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Nexus endpoint", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete Nexus endpoint", err.Error())
	}
}

func (r *nexusEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updateNexusEndpointModelFromSpec(ctx context.Context, model *nexusEndpointResourceModel, nexusEndpoint *nexusv1.Endpoint) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(nexusEndpoint.GetId())

	model.Name = types.StringValue(nexusEndpoint.GetSpec().GetName())

	if nexusEndpoint.GetSpec().GetDescription() != "" {
		model.Description = types.StringValue(nexusEndpoint.GetSpec().GetDescription())
	}

	nexusEndpointTargetSpec := nexusEndpoint.GetSpec().GetTargetSpec()
	if workerSpec := nexusEndpointTargetSpec.GetWorkerTargetSpec(); workerSpec != nil {
		workerTargetSpec := &nexusEndpointWorkerTargetModel{
			NamespaceID: types.StringValue(workerSpec.GetNamespaceId()),
			TaskQueue:   types.StringValue(workerSpec.GetTaskQueue()),
		}
		model.WorkerTargetSpec, diags = types.ObjectValueFrom(ctx, workerTargetSpecAttrs, workerTargetSpec)
		if diags.HasError() {
			return diags
		}
	}

	allowedNamespaces := make([]types.String, 0)
	nexusEndpointPolicySpecs := nexusEndpoint.GetSpec().GetPolicySpecs()
	for _, policySpec := range nexusEndpointPolicySpecs {
		if policySpec.GetAllowedCloudNamespacePolicySpec() != nil {
			allowedNamespaces = append(allowedNamespaces, types.StringValue(policySpec.GetAllowedCloudNamespacePolicySpec().GetNamespaceId()))
		}
	}
	model.AllowedCallerNamespaces, diags = types.SetValueFrom(ctx, types.StringType, allowedNamespaces)
	if diags.HasError() {
		return diags
	}

	return diags
}

func getTargetSpecFromModel(ctx context.Context, model *nexusEndpointResourceModel) (*nexusv1.EndpointTargetSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	var workerTargetSpecModel nexusEndpointWorkerTargetModel
	diags.Append(model.WorkerTargetSpec.As(ctx, &workerTargetSpecModel, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	workerTargetSpec := &nexusv1.WorkerTargetSpec{
		NamespaceId: workerTargetSpecModel.NamespaceID.ValueString(),
		TaskQueue:   workerTargetSpecModel.TaskQueue.ValueString(),
	}

	return &nexusv1.EndpointTargetSpec{
		Variant: &nexusv1.EndpointTargetSpec_WorkerTargetSpec{
			WorkerTargetSpec: workerTargetSpec,
		},
	}, diags
}

func getPolicySpecsFromModel(_ context.Context, model *nexusEndpointResourceModel) ([]*nexusv1.EndpointPolicySpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	policySpecs := make([]*nexusv1.EndpointPolicySpec, 0, len(model.AllowedCallerNamespaces.Elements()))
	for _, namespace := range model.AllowedCallerNamespaces.Elements() {
		ns := namespace.(types.String).ValueString()
		policySpecs = append(policySpecs, &nexusv1.EndpointPolicySpec{
			Variant: &nexusv1.EndpointPolicySpec_AllowedCloudNamespacePolicySpec{
				AllowedCloudNamespacePolicySpec: &nexusv1.AllowedCloudNamespacePolicySpec{
					NamespaceId: ns,
				},
			},
		})
	}

	return policySpecs, diags
}
