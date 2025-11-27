package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	accountv1 "go.temporal.io/cloud-sdk/api/account/v1"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
)

var (
	_ datasource.DataSource              = &accountAuditLogSinkDataSource{}
	_ datasource.DataSourceWithConfigure = &accountAuditLogSinkDataSource{}
)

func NewAccountAuditLogSinkDataSource() datasource.DataSource {
	return &accountAuditLogSinkDataSource{}
}

type (
	accountAuditLogSinkDataSource struct {
		client *client.Client
	}

	accountAuditLogSinkDataModel struct {
		SinkName types.String `tfsdk:"sink_name"`
		Enabled  types.Bool   `tfsdk:"enabled"`
		Kinesis  types.Object `tfsdk:"kinesis"`
		PubSub   types.Object `tfsdk:"pubsub"`
		State    types.String `tfsdk:"state"`
	}
)

func accountAuditLogSinkDataSourceSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"sink_name": schema.StringAttribute{
			Description: "The unique name of the audit log sink.",
			Required:    true,
		},
		"enabled": schema.BoolAttribute{
			Description: "A flag indicating whether the audit log sink is enabled or not.",
			Computed:    true,
		},
		"kinesis": schema.SingleNestedAttribute{
			Description: "The Kinesis configuration details when destination_type is Kinesis.",
			Computed:    true,
			Attributes: map[string]schema.Attribute{
				"role_name": schema.StringAttribute{
					Description: "The IAM role that Temporal Cloud assumes for writing records to the customer's Kinesis stream.",
					Computed:    true,
				},
				"destination_uri": schema.StringAttribute{
					Description: "The destination URI of the Kinesis stream where Temporal will send data.",
					Computed:    true,
				},
				"region": schema.StringAttribute{
					Description: "The region of the Kinesis stream.",
					Computed:    true,
				},
			},
		},
		"pubsub": schema.SingleNestedAttribute{
			Description: "The PubSub configuration details when destination_type is PubSub.",
			Computed:    true,
			Attributes: map[string]schema.Attribute{
				"service_account_id": schema.StringAttribute{
					Description: "The customer service account ID that Temporal Cloud impersonates for writing records to the customer's PubSub topic.",
					Computed:    true,
				},
				"topic_name": schema.StringAttribute{
					Description: "The destination PubSub topic name for Temporal.",
					Computed:    true,
				},
				"gcp_project_id": schema.StringAttribute{
					Description: "The GCP project ID of the PubSub topic and service account.",
					Computed:    true,
				},
			},
		},
		"state": schema.StringAttribute{
			Description: "The current state of the audit log sink.",
			Computed:    true,
		},
	}
}

func (d *accountAuditLogSinkDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_audit_log_sink"
}

func (d *accountAuditLogSinkDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}

	d.client = client
}

func (d *accountAuditLogSinkDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about an account audit log sink.",
		Attributes:  accountAuditLogSinkDataSourceSchema(),
	}
}

func (d *accountAuditLogSinkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input accountAuditLogSinkDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.SinkName.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid account audit log sink sink_name", "account audit log sink sink_name is required")
		return
	}

	auditLogSinkResp, err := d.client.CloudService().GetAccountAuditLogSink(ctx, &cloudservicev1.GetAccountAuditLogSinkRequest{
		Name: input.SinkName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get account audit log sink", err.Error())
		return
	}

	model, diags := accountAuditLogSinkToAccountAuditLogSinkDataModel(ctx, auditLogSinkResp.GetSink())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
}

func accountAuditLogSinkToAccountAuditLogSinkDataModel(ctx context.Context, auditLogSink *accountv1.AuditLogSink) (*accountAuditLogSinkDataModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	stateStr, err := enums.FromResourceState(auditLogSink.State)
	if err != nil {
		diags.AddError("Failed to convert resource state", err.Error())
		return nil, diags
	}

	model := new(accountAuditLogSinkDataModel)
	model.SinkName = types.StringValue(auditLogSink.GetName())
	model.Enabled = types.BoolValue(auditLogSink.GetSpec().GetEnabled())
	model.State = types.StringValue(stateStr)

	kinesisObj := types.ObjectNull(internaltypes.KinesisSpecModelAttrTypes)
	if auditLogSink.GetSpec().GetKinesisSink() != nil {
		kinesisSpec := internaltypes.KinesisSpecModel{
			RoleName:       types.StringValue(auditLogSink.GetSpec().GetKinesisSink().GetRoleName()),
			DestinationUri: types.StringValue(auditLogSink.GetSpec().GetKinesisSink().GetDestinationUri()),
			Region:         types.StringValue(auditLogSink.GetSpec().GetKinesisSink().GetRegion()),
		}

		kinesisObj, diags = types.ObjectValueFrom(ctx, internaltypes.KinesisSpecModelAttrTypes, kinesisSpec)
		if diags.HasError() {
			return nil, diags
		}
	}

	pubsubObj := types.ObjectNull(internaltypes.PubSubSpecModelAttrTypes)
	if auditLogSink.GetSpec().GetPubSubSink() != nil {
		pubsubSpec := internaltypes.PubSubSpecModel{
			ServiceAccountId: types.StringValue(auditLogSink.GetSpec().GetPubSubSink().GetServiceAccountId()),
			TopicName:        types.StringValue(auditLogSink.GetSpec().GetPubSubSink().GetTopicName()),
			GcpProjectId:     types.StringValue(auditLogSink.GetSpec().GetPubSubSink().GetGcpProjectId()),
		}

		pubsubObj, diags = types.ObjectValueFrom(ctx, internaltypes.PubSubSpecModelAttrTypes, pubsubSpec)
		if diags.HasError() {
			return nil, diags
		}
	}

	model.Kinesis = kinesisObj
	model.PubSub = pubsubObj
	return model, diags
}
