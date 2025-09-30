package provider

import (
	"context"
	"fmt"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestNamespaceExportSinkResource_Schema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewNamespaceExportSinkResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccNamespaceExportSink_S3(t *testing.T) {

	namespaceName := fmt.Sprintf("tf-test-ns-export-aws-%s", randomString(8))
	sinkRegion := "us-east-1"
	namespaceRegion := fmt.Sprintf("aws-%s", sinkRegion)
	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkS3Config(namespaceName, sinkName, namespaceRegion, sinkRegion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.bucket_name", "test-bucket"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.role_name", "test-role"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.aws_account_id", "123456789012"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.kms_arn", ""),
				),
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_namespace_export_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update testing
			{
				Config: testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "false"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.bucket_name", "updated-bucket"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.role_name", "test-updated-role"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.aws_account_id", "123456789013"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.kms_arn", "arn:aws:kms:us-east-1:123456789013:key/test-updated-key"),
				),
			},
			// Delete testing
			{
				ResourceName:      "temporalcloud_namespace_export_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}

func TestAccNamespaceExportSink_GCS(t *testing.T) {
	namespaceName := fmt.Sprintf("tf-test-ns-export-gcp-%s", randomString(8))
	sinkRegion := "us-central1"
	namespaceRegion := fmt.Sprintf("gcp-%s", sinkRegion)

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	creationGCSCheckFun := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "sink_name", sinkName),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "true"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.bucket_name", "test-bucket"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.region", sinkRegion),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.service_account_id", "test-sa"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.gcp_project_id", "test-project"),
	)

	updateGCSCheckFun := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "false"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.bucket_name", "updated-bucket"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.region", sinkRegion),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.service_account_id", "test-updated-sa"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.gcp_project_id", "test-updated-project"),
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkGCSConfig(namespaceName, namespaceRegion, sinkName, sinkRegion, false),
				Check:  creationGCSCheckFun,
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_namespace_export_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update with SA email
			{
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, true),
				Check:  updateGCSCheckFun,
			},
			// Delete testing
			{
				ResourceName:      "temporalcloud_namespace_export_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
			// Create with SA email
			{
				Config: testAccNamespaceExportSinkGCSConfig(namespaceName, namespaceRegion, sinkName, sinkRegion, true),
				Check:  creationGCSCheckFun,
			},
			// Update with not SA email
			{
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, false),
				Check:  updateGCSCheckFun,
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_namespace_export_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update with SA email
			{
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, true),
				Check:  updateGCSCheckFun,
			},
			// Delete testing
			{
				ResourceName:      "temporalcloud_namespace_export_sink.test",
				ImportState:       true,
				ImportStateVerify: true,
				Destroy:           true,
			},
		},
	})
}

func testAccNamespaceExportSinkS3Config(namespaceName, sinkName, namespaceRegion, sinkregion string) string {
	return fmt.Sprintf(`
provider "temporalcloud" {
}

resource "temporalcloud_namespace" "terraform" {
  name               = %[1]q
  regions            = [%[2]q]
  api_key_auth 	 	 = true
  retention_days     = 1
}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.terraform.id
  sink_name    = %[3]q
  enabled = true
  s3 = {
    bucket_name    = "test-bucket"
    region         = %[4]q
    role_name      = "test-role"
    aws_account_id = "123456789012"
  }

}
`, namespaceName, namespaceRegion, sinkName, sinkregion)
}

func testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" "terraform" {
  name               = %[1]q
  regions            = [%[2]q]
  api_key_auth       = true
  retention_days     = 1
}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.terraform.id
  sink_name    = %[3]q
  enabled = false
  s3 = {
    bucket_name    = "updated-bucket"
    region         = %[4]q
    role_name      = "test-updated-role"
    aws_account_id = "123456789013"
    kms_arn        = "arn:aws:kms:us-east-1:123456789013:key/test-updated-key"
  }
}
`, namespaceName, namespaceRegion, sinkName, sinkRegion)
}

func testAccNamespaceExportSinkGCSConfig(namespaceName, namespaceRegion, sinkName, sinkRegion string, isSAEmail bool) string {
	var export_config string
	if !isSAEmail {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name         = "test-bucket"
    region              = %[1]q
    service_account_id  = "test-sa"
    gcp_project_id      = "test-project"
  }	
`, sinkRegion)
	} else {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name     = "test-bucket"
    region          = %[1]q
    service_account_email = "test-sa@test-project.iam.gserviceaccount.com"
  }
`, sinkRegion)
	}

	return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
    name           = %[1]q
    regions        = [%[2]q]
    api_key_auth   = true
    retention_days = 1
}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.terraform.id
  sink_name    = %[3]q
  enabled = true
  %[4]s
}
`, namespaceName, namespaceRegion, sinkName, export_config)
}

func testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion string, isSAEmail bool) string {
	var export_config string
	if !isSAEmail {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name         = "updated-bucket"
    region              = %[1]q
    service_account_id  = "test-updated-sa"
    gcp_project_id      = "test-updated-project"
  }
`, sinkRegion)
	} else {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name     = "updated-bucket"
    region          = %[1]q
    service_account_email = "test-updated-sa@test-updated-project.iam.gserviceaccount.com"
  }
`, sinkRegion)
	}

	return fmt.Sprintf(`
resource "temporalcloud_namespace" "terraform" {
    name           = %[1]q
    regions        = [%[2]q]
    api_key_auth   = true
    retention_days = 1
}
resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.terraform.id
  sink_name    = %[3]q
  enabled = false
  %[4]s
}
`, namespaceName, namespaceRegion, sinkName, export_config)
}
