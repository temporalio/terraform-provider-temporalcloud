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
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"
	namespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	sinkv1 "go.temporal.io/cloud-sdk/api/sink/v1"
)

type (
	namespaceExportSinkResource struct {
		client *client.Client
	}

	namespaceExportSinkResourceModel struct {
		ID        types.String   `tfsdk:"id"`
		Namespace types.String   `tfsdk:"namespace"`
		SinkName  types.String   `tfsdk:"sink_name"`
		Enabled   types.Bool     `tfsdk:"enabled"`
		S3        types.Object   `tfsdk:"s3"`
		Gcs       types.Object   `tfsdk:"gcs"`
		Timeouts  timeouts.Value `tfsdk:"timeouts"`
	}
)

func NewNamespaceExportSinkResource() resource.Resource {
	return &namespaceExportSinkResource{}
}

var (
	_ resource.Resource                = (*namespaceExportSinkResource)(nil)
	_ resource.ResourceWithConfigure   = (*namespaceExportSinkResource)(nil)
	_ resource.ResourceWithImportState = (*namespaceExportSinkResource)(nil)
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
				Description: "The namespace under which the sink is configured. It's needed to be in the format of <namespace>.<account_id>",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier of the namespace export sink.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sink_name": schema.StringAttribute{
				Description: "The unique name of the export sink, it can't be changed once set.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "A flag indicating whether the export sink is enabled or not.",
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Optional:    true,
			},
			"s3": schema.SingleNestedAttribute{
				Description: "The S3 configuration details when destination_type is S3.",
				Optional:    true,
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
						Optional:    true,
					},
					"aws_account_id": schema.StringAttribute{
						Description: "The AWS account ID associated with the S3 bucket and the assumed role.",
						Required:    true,
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("gcs"),
					}...),
				},
			},
			"gcs": schema.SingleNestedAttribute{
				Description: "The GCS configuration details when destination_type is GCS.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"service_account_id": schema.StringAttribute{
						Description: "The customer service account ID that Temporal Cloud impersonates for writing records to the customer's GCS bucket. If not provided, the service_account_email must be provided.",
						Optional:    true,
						Computed:    true,
					},
					"bucket_name": schema.StringAttribute{
						Description: "The name of the destination GCS bucket where Temporal will send data.",
						Required:    true,
					},
					"gcp_project_id": schema.StringAttribute{
						Description: "The GCP project ID associated with the GCS bucket and service account. If not provided, the service_account_email must be provided.",
						Optional:    true,
						Computed:    true,
					},
					"region": schema.StringAttribute{
						Description: "The region of the gcs bucket",
						Required:    true,
					},
					"service_account_email": schema.StringAttribute{
						Description: "The service account email associated with the GCS bucket and service account. If not provided, the service_account_id and gcp_project_id must be provided.",
						Optional:    true,
						Validators: []validator.String{
							stringvalidator.RegexMatches(
								regexp.MustCompile(`^(\S+)@(\S+).iam.gserviceaccount.com$`),
								"Service account email must be in the format of '<sa>@<gcp_project>.iam.gserviceaccount.com' where <sa> is the service account ID and <gcp_project> is a valid GCP project ID",
							),
						},
						Computed: true,
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("s3"),
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
		resp.Diagnostics.AddError("Failed to get namespace export sink creation status", err.Error())
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

	resp.Diagnostics.Append(updateSinkModelFromSpec(ctx, &plan, sink.GetSink(), plan.Namespace.ValueString())...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func updateSinkModelFromSpec(ctx context.Context, state *namespaceExportSinkResourceModel, sink *namespacev1.ExportSink, namespace string) diag.Diagnostics {
	var diags diag.Diagnostics

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
		diags.Append(diags...)
		if diags.HasError() {
			return diags
		}
	}

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
		diags.Append(diags...)
		if diags.HasError() {
			return diags
		}
	}

	state.SinkName = types.StringValue(sink.GetName())
	state.Enabled = types.BoolValue(sink.GetSpec().GetEnabled())
	state.S3 = s3Obj
	state.Gcs = gcsObj
	state.Namespace = types.StringValue(namespace)
	state.ID = types.StringValue(fmt.Sprintf("%s,%s", namespace, sink.GetName()))

	return diags
}

func (r *namespaceExportSinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan namespaceExportSinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := plan.Timeouts.Delete(ctx, defaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespace, sinkName := getNamespaceAndSinkNameFromID(plan.ID.ValueString())
	currentSink, err := r.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: namespace,
		Name:      sinkName,
	})

	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace Export Sink Resource not found, removing from state", map[string]interface{}{
				"id": plan.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	svcResp, err := r.client.CloudService().DeleteNamespaceExportSink(ctx, &cloudservicev1.DeleteNamespaceExportSinkRequest{
		Namespace:        namespace,
		Name:             sinkName,
		ResourceVersion:  currentSink.GetSink().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})

	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace Export Sink Resource not found, removing from state", map[string]interface{}{
				"id": plan.ID.ValueString(),
			})

			return
		}

		resp.Diagnostics.AddError("Failed to delete namespace export sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink deletion status", err.Error())
		return
	}
}

func (r *namespaceExportSinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func parseSAPrincipal(saPrincipal string) (string, string) {
	var gcpProjectId, saId string
	saPrincipalPattern := regexp.MustCompile(`^(\S+)@(\S+).iam.gserviceaccount.com$`)

	submatch := saPrincipalPattern.FindStringSubmatch(saPrincipal)

	if len(submatch) != 3 {
		return "", ""
	}

	saId = submatch[1]
	gcpProjectId = submatch[2]

	return saId, gcpProjectId
}

func getSinkSpecFromModel(ctx context.Context, plan *namespaceExportSinkResourceModel) (*namespacev1.ExportSinkSpec, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Check that only one of S3 or GCS is set
	if !plan.S3.IsNull() && !plan.Gcs.IsNull() {
		diags.AddError("Invalid sink configuration", "Only one of S3 or GCS can be configured")
		return nil, diags
	}

	if !plan.S3.IsNull() {
		var s3Spec internaltypes.S3SpecModel
		diags.Append(plan.S3.As(ctx, &s3Spec, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		return &namespacev1.ExportSinkSpec{
			Name:    plan.SinkName.ValueString(),
			Enabled: plan.Enabled.ValueBool(),
			S3: &sinkv1.S3Spec{
				RoleName:     s3Spec.RoleName.ValueString(),
				BucketName:   s3Spec.BucketName.ValueString(),
				Region:       s3Spec.Region.ValueString(),
				KmsArn:       s3Spec.KmsArn.ValueString(),
				AwsAccountId: s3Spec.AwsAccountId.ValueString(),
			},
		}, nil
	} else if !plan.Gcs.IsNull() {
		var gcsSpec internaltypes.GCSSpecModel
		diags.Append(plan.Gcs.As(ctx, &gcsSpec, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}

		saId := gcsSpec.SaId.ValueString()
		gcpProjectId := gcsSpec.GcpProjectId.ValueString()
		if saId == "" && gcpProjectId == "" && gcsSpec.ServiceAccountEmail.ValueString() != "" {
			saId, gcpProjectId = parseSAPrincipal(gcsSpec.ServiceAccountEmail.ValueString())

		}

		if saId == "" || gcpProjectId == "" {
			diags.AddError(
				"Missing Service Account Configuration",
				"Either provide both service_account_id and gcp_project_id, or provide a valid service_account_email",
			)
			return nil, diags
		}

		return &namespacev1.ExportSinkSpec{
			Name:    plan.SinkName.ValueString(),
			Enabled: plan.Enabled.ValueBool(),
			Gcs: &sinkv1.GCSSpec{
				SaId:         saId,
				BucketName:   gcsSpec.BucketName.ValueString(),
				GcpProjectId: gcpProjectId,
				Region:       gcsSpec.Region.ValueString(),
			},
		}, nil
	}

	return nil, diags
}

func (r *namespaceExportSinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceExportSinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespace, sinkName := getNamespaceAndSinkNameFromID(state.ID.ValueString())

	sink, err := r.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: namespace,
		Name:      sinkName,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			tflog.Warn(ctx, "Namespace Export Sink Resource not found, removing from state", map[string]interface{}{
				"id": state.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateSinkModelFromSpec(ctx, &state, sink.GetSink(), namespace)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
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

	namespace, sinkName := getNamespaceAndSinkNameFromID(plan.ID.ValueString())

	currentSink, err := r.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: namespace,
		Name:      sinkName,
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	svcResp, err := r.client.CloudService().UpdateNamespaceExportSink(ctx, &cloudservicev1.UpdateNamespaceExportSinkRequest{
		Namespace:        namespace,
		Spec:             sinkSpec,
		ResourceVersion:  currentSink.GetSink().GetResourceVersion(),
		AsyncOperationId: uuid.New().String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update namespace export sink", err.Error())
		return
	}

	if err := client.AwaitAsyncOperation(ctx, r.client, svcResp.AsyncOperation); err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink update status", err.Error())
		return
	}

	sink, err := r.client.CloudService().GetNamespaceExportSink(ctx, &cloudservicev1.GetNamespaceExportSinkRequest{
		Namespace: namespace,
		Name:      sinkName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace export sink", err.Error())
		return
	}

	resp.Diagnostics.Append(updateSinkModelFromSpec(ctx, &plan, sink.GetSink(), namespace)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func getNamespaceAndSinkNameFromID(id string) (string, string) {
	splits := strings.Split(id, ",")
	if len(splits) != 2 {
		return "", ""
	}
	return splits[0], splits[1]
}
