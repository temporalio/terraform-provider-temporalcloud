package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	"go.temporal.io/sdk/converter"

	nexusv1 "go.temporal.io/cloud-sdk/api/nexus/v1"
)

type (
	nexusEndpointDataModel struct {
		ID                      types.String `tfsdk:"id"`
		Name                    types.String `tfsdk:"name"`
		Description             types.String `tfsdk:"description"`
		WorkerTarget            types.Object `tfsdk:"worker_target"`
		AllowedCallerNamespaces types.Set    `tfsdk:"allowed_caller_namespaces"`

		State     types.String `tfsdk:"state"`
		CreatedAt types.String `tfsdk:"created_at"`
		UpdatedAt types.String `tfsdk:"updated_at"`
	}
)

func nexusEndpointSchema(idRequired bool) map[string]schema.Attribute {
	idAttribute := schema.StringAttribute{
		Description: "The unique identifier of the Nexus Endpoint.",
	}

	switch idRequired {
	case true:
		idAttribute.Required = true
	case false:
		idAttribute.Computed = true
	}

	return map[string]schema.Attribute{
		"id": idAttribute,
		"name": schema.StringAttribute{
			Description: "The name of the endpoint. Unique within an account and match `^[a-zA-Z][a-zA-Z0-9\\-]*[a-zA-Z0-9]$`",
			Computed:    true,
		},
		"description": schema.StringAttribute{
			Description: "The description of the Nexus Endpoint.",
			Optional:    true,
			Sensitive:   true,
			Computed:    true,
		},
		"worker_target": schema.SingleNestedAttribute{
			Description: "The target spec for routing nexus requests to a specific cloud namespace worker.",
			Attributes: map[string]schema.Attribute{
				"namespace_id": schema.StringAttribute{
					Description: "The target cloud namespace to route requests to. Namespace is in same account as the endpoint.",
					Computed:    true,
				},
				"task_queue": schema.StringAttribute{
					Description: "The task queue on the cloud namespace to route requests to.",
					Computed:    true,
				},
			},
			Computed: true,
		},
		"allowed_caller_namespaces": schema.SetAttribute{
			Description: "Namespace Id(s) that are allowed to call this Endpoint.",
			ElementType: types.StringType,
			Computed:    true,
		},
		"state": schema.StringAttribute{
			Description: "The current state of the Nexus Endpoint.",
			Computed:    true,
		},
		"created_at": schema.StringAttribute{
			Description: "The creation time of the Nexus Endpoint.",
			Computed:    true,
		},
		"updated_at": schema.StringAttribute{
			Description: "The last update time of the Nexus Endpoint.",
			Computed:    true,
		},
	}
}

func nexusEndpointToNexusEndpointDataModel(ctx context.Context, endpoint *nexusv1.Endpoint) (*nexusEndpointDataModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	stateStr, err := enums.FromResourceState(endpoint.State)
	if err != nil {
		diags.AddError("Unable to convert nexus endpoint state", err.Error())
		return nil, diags
	}

	nexusEndpointModel := &nexusEndpointDataModel{
		ID:   types.StringValue(endpoint.Id),
		Name: types.StringValue(endpoint.GetSpec().GetName()),

		State:     types.StringValue(stateStr),
		CreatedAt: types.StringValue(endpoint.GetCreatedTime().AsTime().GoString()),
		UpdatedAt: types.StringValue(endpoint.GetLastModifiedTime().AsTime().GoString()),
	}

	if endpoint.GetSpec().GetDescription() != nil {
		var description string
		err := converter.GetDefaultDataConverter().FromPayload(endpoint.GetSpec().GetDescription(), &description)
		if err != nil {
			diags.AddError("Failed to convert Nexus endpoint description "+endpoint.GetSpec().GetName()+" %s", err.Error())
			return nil, diags
		}
		nexusEndpointModel.Description = types.StringValue(description)
	}

	nexusEndpointTargetSpec := endpoint.GetSpec().GetTargetSpec()
	if workerTargetSpec := nexusEndpointTargetSpec.GetWorkerTargetSpec(); workerTargetSpec != nil {
		workerTarget := &nexusEndpointWorkerTargetModel{
			NamespaceID: types.StringValue(workerTargetSpec.GetNamespaceId()),
			TaskQueue:   types.StringValue(workerTargetSpec.GetTaskQueue()),
		}
		nexusEndpointModel.WorkerTarget, diags = types.ObjectValueFrom(ctx, workerTargetAttrs, workerTarget)
		if diags.HasError() {
			return nil, diags
		}
	}

	allowedNamespaces := make([]types.String, 0)
	nexusEndpointPolicySpecs := endpoint.GetSpec().GetPolicySpecs()
	for _, policySpec := range nexusEndpointPolicySpecs {
		if policySpec.GetAllowedCloudNamespacePolicySpec() != nil {
			allowedNamespaces = append(allowedNamespaces, types.StringValue(policySpec.GetAllowedCloudNamespacePolicySpec().GetNamespaceId()))
		}
	}
	nexusEndpointModel.AllowedCallerNamespaces, diags = types.SetValueFrom(ctx, types.StringType, allowedNamespaces)
	if diags.HasError() {
		return nil, diags
	}

	return nexusEndpointModel, diags
}
