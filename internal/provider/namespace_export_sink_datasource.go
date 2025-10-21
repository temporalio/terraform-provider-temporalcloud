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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	namespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
)

var (
	_ datasource.DataSource              = &namespaceExportSinkDataSource{}
	_ datasource.DataSourceWithConfigure = &namespaceExportSinkDataSource{}
)

func NewNamespaceExportSinkDataSource() datasource.DataSource {
	return &namespaceExportSinkDataSource{}
}

type (
	namespaceExportSinkDataSource struct {
		client *client.Client
	}

	namespaceExportSinkDataModel struct {
		ID        types.String `tfsdk:"id"`
		Namespace types.String `tfsdk:"namespace"`
		SinkName  types.String `tfsdk:"sink_name"`
		Enabled   types.Bool   `tfsdk:"enabled"`
		S3        types.Object `tfsdk:"s3"`
		Gcs       types.Object `tfsdk:"gcs"`
	}
)

func namespaceExportSinkDataSourceSchema(idRequired bool) map[string]schema.Attribute {
	idAttribute := schema.StringAttribute{
		Description: "The unique identifier of the namespace export sink.",
	}

	switch idRequired {
	case true:
		idAttribute.Required = true
	case false:
		idAttribute.Computed = true
	}

	return map[string]schema.Attribute{
		"id": idAttribute,
		"namespace": schema.StringAttribute{
			Description: "The namespace under which the sink is configured.",
			Computed:    true,
		},
		"sink_name": schema.StringAttribute{
			Description: "The unique name of the export sink.",
			Computed:    true,
		},
		"enabled": schema.BoolAttribute{
			Description: "A flag indicating whether the export sink is enabled or not.",
			Computed:    true,
		},
		"s3": schema.SingleNestedAttribute{
			Description: "The S3 configuration details when destination_type is S3.",
			Computed:    true,
			Attributes: map[string]schema.Attribute{
				"role_name": schema.StringAttribute{
					Description: "The IAM role that Temporal Cloud assumes for writing records to the customer's S3 bucket.",
					Computed:    true,
				},
				"bucket_name": schema.StringAttribute{
					Description: "The name of the destination S3 bucket where Temporal will send data.",
					Computed:    true,
				},
				"region": schema.StringAttribute{
					Description: "The region where the S3 bucket is located.",
					Computed:    true,
				},
				"kms_arn": schema.StringAttribute{
					Description: "The AWS Key Management Service (KMS) ARN used for encryption.",
					Computed:    true,
				},
				"aws_account_id": schema.StringAttribute{
					Description: "The AWS account ID associated with the S3 bucket and the assumed role.",
					Computed:    true,
				},
			},
		},
		"gcs": schema.SingleNestedAttribute{
			Description: "The GCS configuration details when destination_type is GCS.",
			Computed:    true,
			Attributes: map[string]schema.Attribute{
				"service_account_id": schema.StringAttribute{
					Description: "The customer service account ID that Temporal Cloud impersonates for writing records to the customer's GCS bucket.",
					Computed:    true,
				},
				"bucket_name": schema.StringAttribute{
					Description: "The name of the destination GCS bucket where Temporal will send data.",
					Computed:    true,
				},
				"gcp_project_id": schema.StringAttribute{
					Description: "The GCP project ID associated with the GCS bucket and service account.",
					Computed:    true,
				},
				"region": schema.StringAttribute{
					Description: "The region of the gcs bucket.",
					Computed:    true,
				},
				"service_account_email": schema.StringAttribute{
					Description: "The service account email associated with the GCS bucket and service account.",
					Computed:    true,
				},
			},
		},
	}
}

func (d *namespaceExportSinkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = client
}

func (d *namespaceExportSinkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace_export_sink"
}

func (d *namespaceExportSinkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches details about a namespace export sink.",
		Attributes:  namespaceExportSinkDataSourceSchema(true),
	}
}

func (d *namespaceExportSinkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var input namespaceExportSinkDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &input)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(input.ID.ValueString()) == 0 {
		resp.Diagnostics.AddError("invalid namespace export sink id", "namespace export sink id is required")
		return
	}

	namespace, sinkName := getNamespaceAndSinkNameFromID(input.ID.ValueString())
	if namespace == "" || sinkName == "" {
		resp.Diagnostics.AddError("invalid namespace export sink id", "namespace export sink id must be in format '<namespace>,<sink_name>'")
		return
	}

	sinkResp, err := d.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: namespace,
		Name:      sinkName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	model, diags := namespaceExportSinkToDataModel(ctx, sinkResp.GetSink(), namespace)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, model)
	resp.Diagnostics.Append(diags...)
}

func namespaceExportSinkToDataModel(ctx context.Context, sink *namespacev1.ExportSink, namespace string) (*namespaceExportSinkDataModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := &namespaceExportSinkDataModel{
		ID:        types.StringValue(fmt.Sprintf("%s,%s", namespace, sink.GetName())),
		Namespace: types.StringValue(namespace),
		SinkName:  types.StringValue(sink.GetName()),
		Enabled:   types.BoolValue(sink.GetSpec().GetEnabled()),
	}

	// Handle S3 configuration
	s3Obj := types.ObjectNull(internaltypes.S3SpecModelAttrTypes)
	if sink.GetSpec().GetS3() != nil {
		s3Spec := internaltypes.S3SpecModel{
			RoleName:     types.StringValue(sink.GetSpec().GetS3().GetRoleName()),
			BucketName:   types.StringValue(sink.GetSpec().GetS3().GetBucketName()),
			Region:       types.StringValue(sink.GetSpec().GetS3().GetRegion()),
			KmsArn:       types.StringValue(sink.GetSpec().GetS3().GetKmsArn()),
			AwsAccountId: types.StringValue(sink.GetSpec().GetS3().GetAwsAccountId()),
		}
		s3Obj, diags = types.ObjectValueFrom(ctx, internaltypes.S3SpecModelAttrTypes, s3Spec)
		if diags.HasError() {
			return nil, diags
		}
	}
	model.S3 = s3Obj

	// Handle GCS configuration
	gcsObj := types.ObjectNull(internaltypes.GcsSpecModelAttrTypes)
	if sink.GetSpec().GetGcs() != nil {
		saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", sink.GetSpec().GetGcs().GetSaId(), sink.GetSpec().GetGcs().GetGcpProjectId())
		gcsSpec := internaltypes.GCSSpecModel{
			SaId:                types.StringValue(sink.GetSpec().GetGcs().GetSaId()),
			BucketName:          types.StringValue(sink.GetSpec().GetGcs().GetBucketName()),
			GcpProjectId:        types.StringValue(sink.GetSpec().GetGcs().GetGcpProjectId()),
			Region:              types.StringValue(sink.GetSpec().GetGcs().GetRegion()),
			ServiceAccountEmail: types.StringValue(saEmail),
		}

		gcsObj, diags = types.ObjectValueFrom(ctx, internaltypes.GcsSpecModelAttrTypes, gcsSpec)
		if diags.HasError() {
			return nil, diags
		}
	}
	model.Gcs = gcsObj

	return model, diags
}

func getNamespaceAndSinkNameFromID(id string) (string, string) {
	splits := strings.Split(id, ",")
	if len(splits) != 2 {
		return "", ""
	}
	return splits[0], splits[1]
}
