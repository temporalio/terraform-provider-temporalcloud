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
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/temporalio/tcld/protogen/api/namespace/v1"
	"github.com/temporalio/tcld/protogen/api/namespaceservice/v1"
)

const (
	defaultCreateTimeout time.Duration = 5 * time.Minute
	defaultDeleteTimeout time.Duration = 5 * time.Minute
)

type (
	namespaceResource struct {
		client *Client
	}

	namespaceResourceModel struct {
		Name             types.String   `tfsdk:"name"`
		Namespace        types.String   `tfsdk:"namespace"`
		Region           types.String   `tfsdk:"region"`
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

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
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
			"namespace": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"region": schema.StringAttribute{
				Required: true,
			},
			"accepted_client_ca": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
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
	svc := r.client.NamespaceService()
	svcResp, err := svc.CreateNamespace(ctx, &namespaceservice.CreateNamespaceRequest{
		Namespace: plan.Name.ValueString(),
		Spec: &namespace.NamespaceSpec{
			Region:           plan.Region.ValueString(),
			AcceptedClientCa: plan.AcceptedClientCA.ValueString(),
			RetentionDays:    int32(plan.RetentionDays.ValueInt64()),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	if err = r.client.AwaitResponse(ctx, svcResp.RequestStatus.RequestId); err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	nsName := svcResp.RequestStatus.ResourceId
	tflog.Debug(ctx, "querying namespace for existence", map[string]any{
		"name": nsName,
	})
	ns, err := svc.GetNamespace(ctx, &namespaceservice.GetNamespaceRequest{
		Namespace: nsName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace after creation", err.Error())
		return
	}

	plan.Namespace = types.StringValue(ns.Namespace.Namespace)
	plan.Region = types.StringValue(ns.Namespace.Spec.Region)
	plan.AcceptedClientCA = types.StringValue(ns.Namespace.Spec.AcceptedClientCa)
	plan.RetentionDays = types.Int64Value(int64(ns.Namespace.Spec.RetentionDays))
	plan.ResourceVersion = types.StringValue(ns.Namespace.ResourceVersion)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *namespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc := r.client.NamespaceService()
	ns, err := svc.GetNamespace(ctx, &namespaceservice.GetNamespaceRequest{
		Namespace: state.Namespace.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	tflog.Debug(ctx, "namespace resource version", map[string]any{
		"version": ns.Namespace.ResourceVersion,
	})
	state.Namespace = types.StringValue(ns.Namespace.Namespace)
	state.Region = types.StringValue(ns.Namespace.Spec.Region)
	state.AcceptedClientCA = types.StringValue(ns.Namespace.Spec.AcceptedClientCa)
	state.RetentionDays = types.Int64Value(int64(ns.Namespace.Spec.RetentionDays))
	state.ResourceVersion = types.StringValue(ns.Namespace.ResourceVersion)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *namespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan namespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc := r.client.NamespaceService()
	svcResp, err := svc.UpdateNamespace(ctx, &namespaceservice.UpdateNamespaceRequest{
		Namespace:       plan.Namespace.ValueString(),
		ResourceVersion: plan.ResourceVersion.ValueString(),
		Spec: &namespace.NamespaceSpec{
			Region:           plan.Region.ValueString(),
			AcceptedClientCa: plan.AcceptedClientCA.ValueString(),
			RetentionDays:    int32(plan.RetentionDays.ValueInt64()),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update namespace", err.Error())
		return
	}

	if err = r.client.AwaitResponse(ctx, svcResp.RequestStatus.RequestId); err != nil {
		resp.Diagnostics.AddError("Failed to update namespace", err.Error())
		return
	}

	ns, err := svc.GetNamespace(ctx, &namespaceservice.GetNamespaceRequest{
		Namespace: plan.Namespace.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	plan.Namespace = types.StringValue(ns.Namespace.Namespace)
	plan.Region = types.StringValue(ns.Namespace.Spec.Region)
	plan.AcceptedClientCA = types.StringValue(ns.Namespace.Spec.AcceptedClientCa)
	plan.RetentionDays = types.Int64Value(int64(ns.Namespace.Spec.RetentionDays))
	plan.ResourceVersion = types.StringValue(ns.Namespace.ResourceVersion)
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
	svc := r.client.NamespaceService()
	svcResp, err := svc.DeleteNamespace(ctx, &namespaceservice.DeleteNamespaceRequest{
		Namespace:       state.Namespace.ValueString(),
		ResourceVersion: state.ResourceVersion.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
		return
	}

	if err = r.client.AwaitResponse(ctx, svcResp.RequestStatus.RequestId); err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
	}
}
