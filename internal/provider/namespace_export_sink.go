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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	cloudservicev1 "go.temporal.io/api/cloud/cloudservice/v1"
	namespacev1 "go.temporal.io/api/cloud/namespace/v1"
	"go.temporal.io/api/cloud/sink/v1"
)

type (
	namespaceExportSinkResource struct {
		client *client.Client
	}

	namespaceExportSinkResourceModel struct {
		Namespace types.String   `tfsdk:"namespace"`
		Spec      types.Object   `tfsdk:"sink_spec"`
		Timeouts  timeouts.Value `tfsdk:"timeouts"`
	}

	namespaceExportSinkSpecModel struct {
		Name    types.String `tfsdk:"name"`
		Enabled types.Bool   `tfsdk:"enabled"`
		S3      types.Object `tfsdk:"s3"`
		Gcs     types.Object `tfsdk:"gcs"`
	}

	s3SpecModel struct {
		RoleName     types.String `tfsdk:"role_name"`
		BucketName   types.String `tfsdk:"bucket_name"`
		Region       types.String `tfsdk:"region"`
		KmsArn       types.String `tfsdk:"kms_arn"`
		AwsAccountId types.String `tfsdk:"aws_account_id"`
	}

	gcsSpecModel struct {
		SaId         types.String `tfsdk:"sa_id"`
		BucketName   types.String `tfsdk:"bucket_name"`
		GcpProjectId types.String `tfsdk:"gcp_project_id"`
		Region       types.String `tfsdk:"region"`
	}
)

func NewNamespaceExportSinkResource() resource.Resource {
	return &namespaceExportSinkResource{}
}

var (
	_ resource.Resource                = (*namespaceExportSinkResource)(nil)
	_ resource.ResourceWithConfigure   = (*namespaceExportSinkResource)(nil)
	_ resource.ResourceWithImportState = (*namespaceExportSinkResource)(nil)

	namespaceExportSinkSpecModelAttrTypes = map[string]attr.Type{
		"name":    types.StringType,
		"enabled": types.BoolType,
		"s3": types.ObjectType{
			AttrTypes: s3SpecModelAttrTypes,
		},
		"gcs": types.ObjectType{
			AttrTypes: gcsSpecModelAttrTypes,
		},
	}

	s3SpecModelAttrTypes = map[string]attr.Type{
		"role_name":      types.StringType,
		"bucket_name":    types.StringType,
		"region":         types.StringType,
		"kms_arn":        types.StringType,
		"aws_account_id": types.StringType,
	}

	gcsSpecModelAttrTypes = map[string]attr.Type{
		"sa_id":          types.StringType,
		"bucket_name":    types.StringType,
		"gcp_project_id": types.StringType,
		"region":         types.StringType,
	}
)

func (r *namespaceExportSinkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *namespaceExportSinkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace_export_sink"
}

func (r *namespaceExportSinkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provisions a namespace export sink.",
		Attributes: map[string]schema.Attribute{
			"namespace": schema.StringAttribute{
				Description: "The namespace under which the sink is configured.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"spec": schema.SingleNestedAttribute{
				Description: "The specification for the export sink.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Description: "The unique name of the export sink, it can't be changed once set.",
						Required:    true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"enabled": schema.BoolAttribute{
						Description: "A flag indicating whether the export sink is enabled or not.",
						Required:    false,
						Default:     booldefault.StaticBool(true),
					},
					"s3": schema.SingleNestedAttribute{
						Description: "The S3 configuration details when destination_type is S3.",
						Required:    true,
						Attributes: map[string]schema.Attribute{
							"role_name": schema.StringAttribute{
								Description: "The IAM role that Temporal Cloud assumes for writing records to the customer's S3 bucket.",
								Required:    true,
							},
							"bucket_name": schema.StringAttribute{
								Description: "The name of the destination S3 bucket where Temporal will send data.",
								Required:    true,
							},
							"region": schema.StringAttribute{
								Description: "The region where the S3 bucket is located.",
								Required:    true,
							},
							"kms_arn": schema.StringAttribute{
								Description: "The AWS Key Management Service (KMS) ARN used for encryption.",
								Required:    true,
							},
							"aws_account_id": schema.StringAttribute{
								Description: "The AWS account ID associated with the S3 bucket and the assumed role.",
								Required:    true,
							},
						},
					},
					"gcs": schema.SingleNestedAttribute{
						Description: " The GCS configuration details when destination_type is GCS.",
						Required:    true,
						Attributes: map[string]schema.Attribute{
							"sa_id": schema.StringAttribute{
								Description: "The customer service account ID that Temporal Cloud impersonates for writing records to the customer's GCS bucket.",
								Required:    true,
							},
							"bucket_name": schema.StringAttribute{
								Description: "The name of the destination GCS bucket where Temporal will send data.",
								Required:    true,
							},
							"gcp_project_id": schema.StringAttribute{
								Description: "The GCP project ID associated with the GCS bucket and service account.",
								Required:    true,
							},
							"region": schema.StringAttribute{
								Description: "The region of the gcs bucket",
								Required:    true,
							},
						},
					},
				},
			},
		},
	}
}

func (r *namespaceExportSinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceExportSinkResourceModel
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

	sinkSpec, d := getSinkSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() || sinkSpec == nil {
		return
	}

	svcResp, err := r.client.CloudService().CreateNamespaceExportSink(ctx, &cloudservicev1.CreateNamespaceExportSinkRequest{
		Namespace:        plan.Namespace.ValueString(),
		Spec:             sinkSpec,
		AsyncOperationId: uuid.New().String(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create namespace export sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to create namespace export sink", err.Error())
		return
	}

	sink, err := r.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: plan.Namespace.ValueString(),
		Name:      sinkSpec.GetName(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateSinkModelFromSpec(ctx, &plan, sink.GetSink())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func updateSinkModelFromSpec(ctx context.Context, plan *namespaceExportSinkResourceModel, sink *namespacev1.ExportSink) diag.Diagnostics {
	var diags diag.Diagnostics

	s3Obj := types.ObjectNull(s3SpecModelAttrTypes)
	if sink.GetSpec().GetS3() != nil {
		s3Spec := s3SpecModel{
			RoleName:     types.StringValue(sink.GetSpec().GetS3().GetRoleName()),
			BucketName:   types.StringValue(sink.GetSpec().GetS3().GetBucketName()),
			Region:       types.StringValue(sink.GetSpec().GetS3().GetRegion()),
			KmsArn:       types.StringValue(sink.GetSpec().GetS3().GetKmsArn()),
			AwsAccountId: types.StringValue(sink.GetSpec().GetS3().GetAwsAccountId()),
		}
		s3Obj, diags = types.ObjectValueFrom(ctx, s3SpecModelAttrTypes, s3Spec)
		diags.Append(diags...)
		if diags.HasError() {
			return diags
		}
	}

	gcsObj := types.ObjectNull(gcsSpecModelAttrTypes)
	if sink.GetSpec().GetGcs() != nil {
		gcsSpec := gcsSpecModel{
			SaId:         types.StringValue(sink.GetSpec().GetGcs().GetSaId()),
			BucketName:   types.StringValue(sink.GetSpec().GetGcs().GetBucketName()),
			GcpProjectId: types.StringValue(sink.GetSpec().GetGcs().GetGcpProjectId()),
			Region:       types.StringValue(sink.GetSpec().GetGcs().GetRegion()),
		}
		gcsObj, diags = types.ObjectValueFrom(ctx, gcsSpecModelAttrTypes, gcsSpec)
		diags.Append(diags...)
		if diags.HasError() {
			return diags
		}
	}

	plan.Spec, diags = types.ObjectValueFrom(ctx, namespaceExportSinkSpecModelAttrTypes, namespaceExportSinkSpecModel{
		Name:    types.StringValue(sink.GetName()),
		Enabled: types.BoolValue(sink.GetSpec().GetEnabled()),
		S3:      s3Obj,
		Gcs:     gcsObj,
	})

	if diags.HasError() {
		return diags
	}

	return diags
}

func (r *namespaceExportSinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan namespaceExportSinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sinkSpec, diags := getSinkSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := plan.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteNamespaceExportSink(ctx, &cloudservicev1.DeleteNamespaceExportSinkRequest{
		Namespace:        plan.Namespace.ValueString(),
		Name:             sinkSpec.GetName(),
		AsyncOperationId: uuid.New().String(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace export sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to delete namespace export sink", err.Error())
		return
	}
}

func (r *namespaceExportSinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getSinkSpecFromModel(ctx context.Context, plan *namespaceExportSinkResourceModel) (*namespacev1.ExportSinkSpec, diag.Diagnostics) {
	var diags diag.Diagnostics
	var spec internaltypes.ExportSinkSpecModel
	var s3Spec internaltypes.S3SpecModel
	var gcsSpec internaltypes.GCSSpecModel
	diags.Append(plan.Spec.As(ctx, &spec, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	// Check that only one of S3 or GCS is set
	if !spec.S3.IsNull() && !spec.Gcs.IsNull() {
		diags.AddError("Invalid sink configuration", "Only one of S3 or GCS can be configured")
		return nil, diags
	}

	if !spec.S3.IsNull() {
		return &namespacev1.ExportSinkSpec{
			Name:    spec.Name,
			Enabled: spec.Enabled,
			S3: &sink.S3Spec{
				RoleName:     s3Spec.RoleName,
				BucketName:   s3Spec.BucketName,
				Region:       s3Spec.Region,
				KmsArn:       s3Spec.KmsArn,
				AwsAccountId: s3Spec.AwsAccountId,
			},
		}, nil
	} else if !spec.Gcs.IsNull() {
		return &namespacev1.ExportSinkSpec{
			Name:    spec.Name,
			Enabled: spec.Enabled,
			Gcs: &sink.GCSSpec{
				SaId:         gcsSpec.SaId,
				BucketName:   gcsSpec.BucketName,
				GcpProjectId: gcsSpec.GcpProjectId,
				Region:       gcsSpec.Region,
			},
		}, nil
	}

	return nil, diags
}

func (r *namespaceExportSinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan namespaceExportSinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := getSinkSpecFromModel(ctx, &plan)
	if diags.HasError() {
		return
	}

	sink, err := r.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: plan.Namespace.ValueString(),
		Name:      spec.GetName(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateSinkModelFromSpec(ctx, &plan, sink.GetSink())...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *namespaceExportSinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan namespaceExportSinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sinkSpec, diags := getSinkSpecFromModel(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	svcResp, err := r.client.CloudService().UpdateNamespaceExportSink(ctx, &cloudservicev1.UpdateNamespaceExportSinkRequest{
		Namespace:        plan.Namespace.ValueString(),
		Spec:             sinkSpec,
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update namespace export sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to update namespace export sink", err.Error())
		return
	}

	sink, err := r.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: plan.Namespace.ValueString(),
		Name:      sinkSpec.GetName(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateSinkModelFromSpec(ctx, &plan, sink.GetSink())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}
