package types

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	S3SpecModelAttrTypes = map[string]attr.Type{
		"role_name":      types.StringType,
		"bucket_name":    types.StringType,
		"region":         types.StringType,
		"kms_arn":        types.StringType,
		"aws_account_id": types.StringType,
	}

	GcsSpecModelAttrTypes = map[string]attr.Type{
		"service_account_id": types.StringType,
		"bucket_name":        types.StringType,
		"gcp_project_id":     types.StringType,
		"region":             types.StringType,
	}
)

type S3SpecModel struct {
	// The IAM role that Temporal Cloud assumes for writing records to the customer's S3 bucket
	RoleName types.String `tfsdk:"role_name"`

	// The name of the destination S3 bucket where Temporal will send data
	BucketName types.String `tfsdk:"bucket_name"`

	// The region where the S3 bucket is located
	Region types.String `tfsdk:"region"`

	// The AWS Key Management Service (KMS) ARN used for encryption
	KmsArn types.String `tfsdk:"kms_arn"`

	// The AWS account ID associated with the S3 bucket and the assumed role
	AwsAccountId types.String `tfsdk:"aws_account_id"`
}

type GCSSpecModel struct {
	// The customer service account ID that Temporal Cloud impersonates for writing records to the customer's GCS bucket
	SaId types.String `tfsdk:"service_account_id"`

	// The name of the destination GCS bucket where Temporal will send data
	BucketName types.String `tfsdk:"bucket_name"`

	// The GCP project ID associated with the GCS bucket and service account
	GcpProjectId types.String `tfsdk:"gcp_project_id"`

	// The region of the gcs bucket
	Region types.String `tfsdk:"region"`

	// The service account email associated with the GCS bucket and service account
	ServiceAccountEmail types.String `tfsdk:"service_account_email"`
}
