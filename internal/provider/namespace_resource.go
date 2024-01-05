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
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "github.com/temporalio/terraform-provider-temporalcloud/proto/go/temporal/api/cloud/cloudservice/v1"
	namespacev1 "github.com/temporalio/terraform-provider-temporalcloud/proto/go/temporal/api/cloud/namespace/v1"
)

const (
	defaultCreateTimeout time.Duration = 5 * time.Minute
	defaultDeleteTimeout time.Duration = 5 * time.Minute
)

type (
	namespaceResource struct {
		client cloudservicev1.CloudServiceClient
	}

	namespaceResourceModel struct {
		ID               types.String   `tfsdk:"id"`
		Name             types.String   `tfsdk:"name"`
		Regions          types.List     `tfsdk:"regions"`
		AcceptedClientCA types.String   `tfsdk:"accepted_client_ca"`
		RetentionDays    types.Int64    `tfsdk:"retention_days"`
		ResourceVersion  types.String   `tfsdk:"resource_version"`
		Timeouts         timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource              = (*namespaceResource)(nil)
	_ resource.ResourceWithConfigure = (*namespaceResource)(nil)
)

func NewNamespaceResource() resource.Resource {
	return &namespaceResource{}
}

func (r *namespaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Metadata returns the resource type name.
func (r *namespaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

// Schema defines the schema for the resource.
func (r *namespaceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"regions": schema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
			},
			"accepted_client_ca": schema.StringAttribute{
				Required: true,
			},
			"retention_days": schema.Int64Attribute{
				Required: true,
			},
			"resource_version": schema.StringAttribute{
				Computed: true,
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
func (r *namespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceResourceModel
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

	regions := getRegionsFromModel(ctx, resp.Diagnostics, &plan)
	if resp.Diagnostics.HasError() {
		return
	}
	svcResp, err := r.client.CreateNamespace(ctx, &cloudservicev1.CreateNamespaceRequest{
		Spec: &namespacev1.NamespaceSpec{
			Name:          plan.Name.ValueString(),
			Regions:       regions,
			RetentionDays: int32(plan.RetentionDays.ValueInt64()),
			MtlsAuth: &namespacev1.MtlsAuthSpec{
				AcceptedClientCa: plan.AcceptedClientCA.ValueString(),
			},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	ns, err := r.client.GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: svcResp.Namespace,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after creation", err.Error())
		return
	}

	tflog.Debug(ctx, "responded with namespace model", map[string]any{
		"resource_version": ns.GetNamespace().GetResourceVersion(),
	})
	updateModelFromSpec(ctx, resp.Diagnostics, &plan, ns.Namespace)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *namespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.client.GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	updateModelFromSpec(ctx, resp.Diagnostics, &state, model.Namespace)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *namespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan namespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	regions := getRegionsFromModel(ctx, resp.Diagnostics, &plan)
	if resp.Diagnostics.HasError() {
		return
	}
	svcResp, err := r.client.UpdateNamespace(ctx, &cloudservicev1.UpdateNamespaceRequest{
		Namespace: plan.ID.ValueString(),
		Spec: &namespacev1.NamespaceSpec{
			Name:          plan.Name.ValueString(),
			Regions:       regions,
			RetentionDays: int32(plan.RetentionDays.ValueInt64()),
			MtlsAuth: &namespacev1.MtlsAuthSpec{
				AcceptedClientCa: plan.AcceptedClientCA.ValueString(),
			},
		},
		ResourceVersion: plan.ResourceVersion.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update namespace", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to update namespace", err.Error())
		return
	}

	ns, err := r.client.GetNamespace(ctx, &cloudservicev1.GetNamespaceRequest{
		Namespace: plan.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after update", err.Error())
		return
	}

	updateModelFromSpec(ctx, resp.Diagnostics, &plan, ns.Namespace)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *namespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state namespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	svcResp, err := r.client.DeleteNamespace(ctx, &cloudservicev1.DeleteNamespaceRequest{
		Namespace:       state.ID.ValueString(),
		ResourceVersion: state.ResourceVersion.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
	}
}

func getRegionsFromModel(ctx context.Context, diags diag.Diagnostics, plan *namespaceResourceModel) []string {
	regions := make([]types.String, 0, len(plan.Regions.Elements()))
	diags.Append(plan.Regions.ElementsAs(ctx, &regions, false)...)
	if diags.HasError() {
		return nil
	}

	requestRegions := make([]string, len(regions))
	for i, region := range regions {
		requestRegions[i] = region.ValueString()
	}

	return requestRegions
}

func updateModelFromSpec(ctx context.Context, diags diag.Diagnostics, state *namespaceResourceModel, ns *namespacev1.Namespace) {
	state.ID = types.StringValue(ns.GetNamespace())
	planRegions, listDiags := types.ListValueFrom(ctx, types.StringType, ns.GetSpec().GetRegions())
	diags.Append(listDiags...)
	if diags.HasError() {
		return
	}

	state.Regions = planRegions
	state.AcceptedClientCA = types.StringValue(ns.GetSpec().GetMtlsAuth().GetAcceptedClientCa())
	state.RetentionDays = types.Int64Value(int64(ns.GetSpec().GetRetentionDays()))
	state.ResourceVersion = types.StringValue(ns.GetResourceVersion())
}
