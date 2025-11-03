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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	accountv1 "go.temporal.io/cloud-sdk/api/account/v1"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	sinkv1 "go.temporal.io/cloud-sdk/api/sink/v1"
)

type (
	accountAuditLogSinkResource struct {
		client *client.Client
	}

	accountAuditLogSinkResourceModel struct {
		ID       types.String   `tfsdk:"id"`
		SinkName types.String   `tfsdk:"sink_name"`
		Enabled  types.Bool     `tfsdk:"enabled"`
		Kinesis  types.Object   `tfsdk:"kinesis"`
		PubSub   types.Object   `tfsdk:"pubsub"`
		Timeouts timeouts.Value `tfsdk:"timeouts"`
	}
)

var (
	_ resource.Resource                = (*accountAuditLogSinkResource)(nil)
	_ resource.ResourceWithConfigure   = (*accountAuditLogSinkResource)(nil)
	_ resource.ResourceWithImportState = (*accountAuditLogSinkResource)(nil)
)

func NewAccountAuditLogSinkResource() resource.Resource {
	return &accountAuditLogSinkResource{}
}

func (r *accountAuditLogSinkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accountAuditLogSinkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_audit_log_sink"
}

func (r *accountAuditLogSinkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions an account audit log sink.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the account audit log sink.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sink_name": schema.StringAttribute{
				Description: "The unique name of the audit log sink, it can't be changed once set.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "A flag indicating whether the audit log sink is enabled or not.",
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Optional:    true,
			},
			"kinesis": schema.SingleNestedAttribute{
				Description: "The Kinesis configuration details when destination_type is Kinesis.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"role_name": schema.StringAttribute{
						Description: "The IAM role that Temporal Cloud assumes for writing records to the customer's Kinesis stream.",
						Required:    true,
					},
					"destination_uri": schema.StringAttribute{
						Description: "The destination URI of the Kinesis stream where Temporal will send data.",
						Required:    true,
					},
					"region": schema.StringAttribute{
						Description: "The region of the Kinesis stream.",
						Required:    true,
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("pubsub"),
					}...),
				},
			},
			"pubsub": schema.SingleNestedAttribute{
				Description: "The PubSub configuration details when destination_type is PubSub.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"service_account_id": schema.StringAttribute{
						Description: "The customer service account ID that Temporal Cloud impersonates for writing records to the customer's PubSub topic.",
						Required:    true,
					},
					"topic_name": schema.StringAttribute{
						Description: "The destination PubSub topic name for Temporal.",
						Required:    true,
					},
					"gcp_project_id": schema.StringAttribute{
						Description: "The GCP project ID of the PubSub topic and service account.",
						Required:    true,
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("kinesis"),
					}...),
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

func (r *accountAuditLogSinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accountAuditLogSinkResourceModel
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

	sinkSpec, d := getAccountAuditLogSinkSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() || sinkSpec == nil {
		return
	}

	svcResp, err := r.client.CloudService().CreateAccountAuditLogSink(ctx, &cloudservicev1.CreateAccountAuditLogSinkRequest{
		Spec:             sinkSpec,
		AsyncOperationId: uuid.New().String(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create account audit log sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get account audit log sink creation status", err.Error())
		return
	}

	sink, err := r.client.CloudService().GetAccountAuditLogSink(ctx, &cloudservicev1.GetAccountAuditLogSinkRequest{
		Name: sinkSpec.GetName(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to get account audit log sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateAccountAuditLogSinkModelFromSpec(ctx, &plan, sink.GetSink())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func updateAccountAuditLogSinkModelFromSpec(ctx context.Context, state *accountAuditLogSinkResourceModel, sink *accountv1.AuditLogSink) diag.Diagnostics {
	var diags diag.Diagnostics

	kinesisObj := types.ObjectNull(internaltypes.KinesisSpecModelAttrTypes)
	if sink.GetSpec().GetKinesisSink() != nil {
		kinesisSpec := internaltypes.KinesisSpecModel{
			RoleName:       types.StringValue(sink.GetSpec().GetKinesisSink().GetRoleName()),
			DestinationUri: types.StringValue(sink.GetSpec().GetKinesisSink().GetDestinationUri()),
			Region:         types.StringValue(sink.GetSpec().GetKinesisSink().GetRegion()),
		}

		kinesisObj, diags = types.ObjectValueFrom(ctx, internaltypes.KinesisSpecModelAttrTypes, kinesisSpec)
		if diags.HasError() {
			return diags
		}
	}

	pubsubObj := types.ObjectNull(internaltypes.PubSubSpecModelAttrTypes)
	if sink.GetSpec().GetPubSubSink() != nil {
		pubsubSpec := internaltypes.PubSubSpecModel{
			ServiceAccountId: types.StringValue(sink.GetSpec().GetPubSubSink().GetServiceAccountId()),
			TopicName:        types.StringValue(sink.GetSpec().GetPubSubSink().GetTopicName()),
			GcpProjectId:     types.StringValue(sink.GetSpec().GetPubSubSink().GetGcpProjectId()),
		}

		pubsubObj, diags = types.ObjectValueFrom(ctx, internaltypes.PubSubSpecModelAttrTypes, pubsubSpec)
		if diags.HasError() {
			return diags
		}
	}

	state.SinkName = types.StringValue(sink.GetName())
	state.Enabled = types.BoolValue(sink.GetSpec().GetEnabled())
	state.Kinesis = kinesisObj
	state.PubSub = pubsubObj
	state.ID = types.StringValue(sink.GetName())

	return diags
}

func (r *accountAuditLogSinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan accountAuditLogSinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := plan.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sinkName := plan.ID.ValueString()
	currentSink, err := r.client.CloudService().GetAccountAuditLogSink(ctx, &cloudservicev1.GetAccountAuditLogSinkRequest{
		Name: sinkName,
	})

	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Account Audit Log Sink Resource not found, removing from state", map[string]interface{}{
				"id": plan.ID.ValueString(),
			})
			return
		}

		resp.Diagnostics.AddError("Failed to get account audit log sink", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteAccountAuditLogSink(ctx, &cloudservicev1.DeleteAccountAuditLogSinkRequest{
		Name:             sinkName,
		ResourceVersion:  currentSink.GetSink().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})

	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Account Audit Log Sink Resource not found, removing from state", map[string]interface{}{
				"id": plan.ID.ValueString(),
			})
			return
		}

		resp.Diagnostics.AddError("Failed to delete account audit log sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get account audit log sink deletion status", err.Error())
		return
	}
}

func (r *accountAuditLogSinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getAccountAuditLogSinkSpecFromModel(ctx context.Context, plan *accountAuditLogSinkResourceModel) (*accountv1.AuditLogSinkSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Check that only one of Kinesis or PubSub is set
	if !plan.Kinesis.IsNull() && !plan.PubSub.IsNull() {
		diags.AddError("Invalid sink configuration", "Only one of Kinesis or PubSub can be configured")
		return nil, diags
	}

	if !plan.Kinesis.IsNull() {
		var kinesisSpec internaltypes.KinesisSpecModel
		diags.Append(plan.Kinesis.As(ctx, &kinesisSpec, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		kinesisSinkSpec := &sinkv1.KinesisSpec{
			RoleName:       kinesisSpec.RoleName.ValueString(),
			DestinationUri: kinesisSpec.DestinationUri.ValueString(),
			Region:         kinesisSpec.Region.ValueString(),
		}

		return &accountv1.AuditLogSinkSpec{
			Name:    plan.SinkName.ValueString(),
			Enabled: plan.Enabled.ValueBool(),
			SinkType: &accountv1.AuditLogSinkSpec_KinesisSink{
				KinesisSink: kinesisSinkSpec,
			},
		}, nil
	} else if !plan.PubSub.IsNull() {
		var pubsubSpec internaltypes.PubSubSpecModel
		diags.Append(plan.PubSub.As(ctx, &pubsubSpec, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		pubsubSinkSpec := &sinkv1.PubSubSpec{
			ServiceAccountId: pubsubSpec.ServiceAccountId.ValueString(),
			TopicName:        pubsubSpec.TopicName.ValueString(),
			GcpProjectId:     pubsubSpec.GcpProjectId.ValueString(),
		}

		return &accountv1.AuditLogSinkSpec{
			Name:    plan.SinkName.ValueString(),
			Enabled: plan.Enabled.ValueBool(),
			SinkType: &accountv1.AuditLogSinkSpec_PubSubSink{
				PubSubSink: pubsubSinkSpec,
			},
		}, nil
	}

	diags.AddError("Invalid sink configuration", "Either Kinesis or PubSub must be configured")
	return nil, diags
}

func (r *accountAuditLogSinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accountAuditLogSinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sinkName := state.ID.ValueString()

	sink, err := r.client.CloudService().GetAccountAuditLogSink(ctx, &cloudservicev1.GetAccountAuditLogSinkRequest{
		Name: sinkName,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Account Audit Log Sink Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to get account audit log sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateAccountAuditLogSinkModelFromSpec(ctx, &state, sink.GetSink())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *accountAuditLogSinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan accountAuditLogSinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sinkSpec, diags := getAccountAuditLogSinkSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || sinkSpec == nil {
		return
	}

	sinkName := plan.ID.ValueString()

	currentSink, err := r.client.CloudService().GetAccountAuditLogSink(ctx, &cloudservicev1.GetAccountAuditLogSinkRequest{
		Name: sinkName,
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to get account audit log sink", err.Error())
		return
	}

	svcResp, err := r.client.CloudService().UpdateAccountAuditLogSink(ctx, &cloudservicev1.UpdateAccountAuditLogSinkRequest{
		Spec:             sinkSpec,
		ResourceVersion:  currentSink.GetSink().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update account audit log sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get account audit log sink update status", err.Error())
		return
	}

	sink, err := r.client.CloudService().GetAccountAuditLogSink(ctx, &cloudservicev1.GetAccountAuditLogSinkRequest{
		Name: sinkName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account audit log sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateAccountAuditLogSinkModelFromSpec(ctx, &plan, sink.GetSink())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}
