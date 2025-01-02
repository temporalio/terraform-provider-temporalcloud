// The MIT License
//
// Copyright (c) 2023 Temporal Technologies Inc.  All rights reserved.
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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	nexusv1 "go.temporal.io/api/cloud/nexus/v1"
)

type (
	nexusResource struct {
		client *client.Client
	}

	nexusResourceModel struct {
		ID             types.String              `tfsdk:"id"`
		Name           types.String              `tfsdk:"name"`
		Description    types.String              `tfsdk:"description"`
		TargetEndPoint *nexusTargetEndPointModel `tfsdk:"target_endpoint"`
		EndPointPolicy types.Set                 `tfsdk:"endpoint_policy"`

		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}

	// struct for the target object
	// additional options may be available in the future for this endpoint which is why it's encapsulated into it's own struct
	nexusTargetEndPointModel struct {
		WorkerTargetEndPoint workerTargetEndPointModel `tfsdk:"worker_target_endpoint"`
	}

	// struct for the target object
	workerTargetEndPointModel struct {
		NamespaceID types.String `tfsdk:"target_namespace_id"`
		TaskQueue   types.String `tfsdk:"target_task_queue"`
	}

	// struct for the policy object
	// additional options may be available in the future for this endpoint which is why it's encapsulated into it's own struct
	nexusEndPointPolicyModel struct {
		AllowedCloudNamespacePolicy allowedCloudNamespacePolicyModel `tfsdk:"allowed_cloud_namespace_policy"`
	}

	allowedCloudNamespacePolicyModel struct {
		NamespaceID types.String `tfsdk:"namespace_id"`
	}
)

var (
	_ resource.Resource                = (*nexusResource)(nil)
	_ resource.ResourceWithConfigure   = (*nexusResource)(nil)
	_ resource.ResourceWithImportState = (*nexusResource)(nil)
)

func NewNexusResource() resource.Resource {
	return &nexusResource{}
}

func (r *nexusResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Metadata returns the resource type name.
func (r *nexusResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nexus"
}

// Schema defines the schema for the resource.
func (r *nexusResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a Temporal Cloud Nexus Endpoint.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Nexus Endpoint.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the Nexus endpoint.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "The description for the Nexus endpoint.",
				Optional:    true,
			},
			"target_endpoint": schema.SingleNestedAttribute{
				Description: "A target endpoint that receives and routes Nexus requests.",
				Attributes: map[string]schema.Attribute{
					"worker_target_endpoint": schema.SingleNestedAttribute{
						Description: "The worker target specification.",
						Attributes: map[string]schema.Attribute{
							"target_namespace_id": schema.StringAttribute{
								Description: "The ID of the target cloud namespace to route requests to.",
								Required:    true,
							},
							"target_task_queue": schema.StringAttribute{
								Description: "The task queue on the cloud namespace to route requests to.",
								Required:    true,
							},
						},
						Required: true,
					},
				},
				Required: true,
			},
			"endpoint_policy": schema.SetNestedAttribute{
				Description: "The set of policies for the endpoint.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"allowed_cloud_namespace_policy": schema.SingleNestedAttribute{
							Description: "The allowed cloud namespace policy specification.",
							Attributes: map[string]schema.Attribute{
								"namespace_id": schema.StringAttribute{
									Description: "The namespace ID allowed to call into this endpoint.",
									Required:    true,
								},
							},
							Required: true,
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

// Create creates the resource and sets the initial Terraform state.
func (r *nexusResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var state nexusResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := state.Timeouts.Create(ctx, defaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	policySpecs, d := getEndPointPoliciesFromModel(ctx, &state)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &cloudservicev1.CreateNexusEndpointRequest{
		Spec: &nexusv1.EndpointSpec{
			Name: state.Name.ValueString(),
			TargetSpec: &nexusv1.EndpointTargetSpec{
				Variant: &nexusv1.EndpointTargetSpec_WorkerTargetSpec{
					WorkerTargetSpec: &nexusv1.WorkerTargetSpec{
						NamespaceId: state.TargetEndPoint.WorkerTargetEndPoint.NamespaceID.ValueString(),
						TaskQueue:   state.TargetEndPoint.WorkerTargetEndPoint.TaskQueue.ValueString(),
					},
				},
			},
			PolicySpecs: policySpecs,
			Description: state.Description.ValueString(),
		},
	}

	svcResp, err := r.client.CloudService().CreateNexusEndpoint(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Nexus Enpoint", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create Nexus Endpoint", err.Error())
		return
	}

	endpoint, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: svcResp.EndpointId,
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to get Nexus Endpoint after creation", err.Error())
		return
	}

	resp.Diagnostics.Append(updateNexusModelFromSpec(ctx, &state, endpoint.Endpoint)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *nexusResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nexusResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readReq := &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: state.ID.ValueString(),
	}

	endpoint, err := r.client.CloudService().GetNexusEndpoint(ctx, readReq)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Nexus resource", fmt.Sprintf("Could not read Nexus resource: %s", err))
		return
	}

	resp.Diagnostics.Append(updateNexusModelFromSpec(ctx, &state, endpoint.Endpoint)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *nexusResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state nexusResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentNE, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current namespace status", err.Error())
		return
	}

	policySpecs, d := getEndPointPoliciesFromModel(ctx, &state)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &cloudservicev1.UpdateNexusEndpointRequest{
		EndpointId: state.ID.ValueString(),
		Spec: &nexusv1.EndpointSpec{
			Name: state.Name.ValueString(),
			TargetSpec: &nexusv1.EndpointTargetSpec{
				Variant: &nexusv1.EndpointTargetSpec_WorkerTargetSpec{
					WorkerTargetSpec: &nexusv1.WorkerTargetSpec{
						NamespaceId: state.TargetEndPoint.WorkerTargetEndPoint.NamespaceID.ValueString(),
						TaskQueue:   state.TargetEndPoint.WorkerTargetEndPoint.TaskQueue.ValueString(),
					},
				},
			},
			PolicySpecs: policySpecs,
			Description: state.Description.ValueString(),
		},
		ResourceVersion: currentNE.GetEndpoint().GetResourceVersion(),
	}

	svcResp, err := r.client.CloudService().UpdateNexusEndpoint(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Nexus Endpoint", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to update Nexus Endpoint", err.Error())
		return
	}

	endpoint, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: state.ID.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to get Nexus EndPoint after update", err.Error())
		return
	}

	resp.Diagnostics.Append(updateNexusModelFromSpec(ctx, &state, endpoint.Endpoint)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *nexusResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nexusResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentNE, err := r.client.CloudService().GetNexusEndpoint(ctx, &cloudservicev1.GetNexusEndpointRequest{
		EndpointId: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get current namespace status", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	svcResp, err := r.client.CloudService().DeleteNexusEndpoint(ctx, &cloudservicev1.DeleteNexusEndpointRequest{
		EndpointId:      state.ID.ValueString(),
		ResourceVersion: currentNE.GetEndpoint().GetResourceVersion(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Nexus Endpoint", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete Nexus Endpoint", err.Error())
	}
}

func (r *nexusResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updateNexusModelFromSpec(ctx context.Context, state *nexusResourceModel, endpoint *nexusv1.Endpoint) diag.Diagnostics {
	var diags diag.Diagnostics

	state.ID = types.StringValue(endpoint.Id)
	state.Name = types.StringValue(endpoint.Spec.Name)
	state.Description = types.StringValue(endpoint.Spec.Description)

	if endpoint.Spec.TargetSpec.GetWorkerTargetSpec() != nil {
		state.TargetEndPoint = &nexusTargetEndPointModel{
			WorkerTargetEndPoint: workerTargetEndPointModel{
				NamespaceID: types.StringValue(endpoint.Spec.TargetSpec.GetWorkerTargetSpec().NamespaceId),
				TaskQueue:   types.StringValue(endpoint.Spec.TargetSpec.GetWorkerTargetSpec().TaskQueue),
			},
		}
	} else {
		state.TargetEndPoint = nil
	}

	policySpecs := make([]attr.Value, len(endpoint.Spec.PolicySpecs))
	for i, policySpec := range endpoint.Spec.PolicySpecs {
		// Create a nexusEndPointPolicyModel object for each policy specification
		policy := nexusEndPointPolicyModel{
			AllowedCloudNamespacePolicy: allowedCloudNamespacePolicyModel{
				NamespaceID: types.StringValue(policySpec.GetAllowedCloudNamespacePolicySpec().NamespaceId),
			},
		}
		// Convert the nexusEndPointPolicyModel object to an attr.Value
		policySpecValue, diags := types.ObjectValueFrom(ctx, map[string]attr.Type{
			"allowed_cloud_namespace_policy": types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"namespace_id": types.StringType,
				},
			},
		}, policy)
		if diags.HasError() {
			return diags
		}
		// Assign the converted attr.Value to the policySpecs slice
		policySpecs[i] = policySpecValue
	}

	// Convert the policySpecs slice to a Set and assign it to state.EndPointPolicy
	if len(policySpecs) == 0 {
		state.EndPointPolicy = types.SetNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"allowed_cloud_namespace_policy": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"namespace_id": types.StringType,
					},
				},
			},
		})
	} else {
		state.EndPointPolicy = types.SetValueMust(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"allowed_cloud_namespace_policy": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"namespace_id": types.StringType,
					},
				},
			},
		}, policySpecs)
	}

	return diags
}

func getEndPointPoliciesFromModel(ctx context.Context, model *nexusResourceModel) ([]*nexusv1.EndpointPolicySpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	elements := make([]types.Object, 0, len(model.EndPointPolicy.Elements()))
	diags.Append(model.EndPointPolicy.ElementsAs(ctx, &elements, false)...)
	if diags.HasError() {
		return nil, diags
	}

	if len(elements) == 0 {
		return nil, diags
	}

	policies := make([]*nexusv1.EndpointPolicySpec, len(elements))
	for i, filter := range elements {
		var model nexusEndPointPolicyModel
		diags.Append(filter.As(ctx, &model, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		policies[i] = &nexusv1.EndpointPolicySpec{
			Variant: &nexusv1.EndpointPolicySpec_AllowedCloudNamespacePolicySpec{
				AllowedCloudNamespacePolicySpec: &nexusv1.AllowedCloudNamespacePolicySpec{
					NamespaceId: model.AllowedCloudNamespacePolicy.NamespaceID.ValueString(),
				},
			},
		}
	}

	return policies, diags
}
