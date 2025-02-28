package types

import "github.com/hashicorp/terraform-plugin-framework/types"

type ExportSinkSpecModel struct {
	// The unique name of the export sink, it can't be changed once set
	Name string `tfsdk:"name"`

	// A flag indicating whether the export sink is enabled or not
	Enabled bool `tfsdk:"enabled"`

	// The S3 configuration details when destination_type is S3
	S3 types.Object `tfsdk:"s3"`

	// The GCS configuration details when destination_type is GCS
	Gcs types.Object `tfsdk:"gcs"`
}
type ExportSinkModel struct {
	// The unique name of the export sink
	Name string `tfsdk:"name"`

	// The version of the export sink resource
	ResourceVersion string `tfsdk:"resource_version"`

	// The current state of the export sink
	State string `tfsdk:"state"`

	// The specification details of the export sink
	Spec ExportSinkSpecModel `tfsdk:"spec"`

	// The health status of the export sink
	Health string `tfsdk:"health"`

	// An error message describing any issues with the export sink, if applicable
	ErrorMessage string `tfsdk:"error_message"`

	// The timestamp of the latest successful data export
	LatestDataExportTime string `tfsdk:"latest_data_export_time"`

	// The timestamp of the last health check performed on the export sink
	LastHealthCheckTime string `tfsdk:"last_health_check_time"`
}

type S3SpecModel struct {
	// The IAM role that Temporal Cloud assumes for writing records to the customer's S3 bucket
	RoleName string `tfsdk:"role_name"`

	// The name of the destination S3 bucket where Temporal will send data
	BucketName string `tfsdk:"bucket_name"`

	// The region where the S3 bucket is located
	Region string `tfsdk:"region"`

	// The AWS Key Management Service (KMS) ARN used for encryption
	KmsArn string `tfsdk:"kms_arn"`

	// The AWS account ID associated with the S3 bucket and the assumed role
	AwsAccountId string `tfsdk:"aws_account_id"`
}

type GCSSpecModel struct {
	// The customer service account ID that Temporal Cloud impersonates for writing records to the customer's GCS bucket
	SaId string `tfsdk:"sa_id"`

	// The name of the destination GCS bucket where Temporal will send data
	BucketName string `tfsdk:"bucket_name"`

	// The GCP project ID associated with the GCS bucket and service account
	GcpProjectId string `tfsdk:"gcp_project_id"`

	// The region of the gcs bucket
	Region string `tfsdk:"region"`
}
