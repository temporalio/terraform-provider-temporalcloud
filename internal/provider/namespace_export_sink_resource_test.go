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
	region := "us-east-1"
	fullRegion := fmt.Sprintf("aws-%s", region)
	ns_config := func(name string, region string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["%s"]
  api_key_auth 	 	 = true
  retention_days     = 1
}`, name, region)
	}

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ns_config(namespaceName, fullRegion),
			},
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkS3Config(namespaceName, sinkName, region),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "namespace", namespaceName),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.s3.bucket_name", "test-bucket"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.s3.region", region),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.s3.role_name", "test-role"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.s3.aws_account_id", "123456789012"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.s3.kms_arn", "arn:aws:kms:us-east-1:123456789012:key/test-key"),
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
				Config: testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, sinkName, region),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.enabled", "false"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.s3.bucket_name", "updated-bucket"),
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

	region := "us-west1"
	fullRegion := fmt.Sprintf("gcp-%s", region)

	ns_config := func(name string, region string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
    name           = "%s"
    regions        = ["%s"]
    api_key_auth   = true
    retention_days = 1
}`, name, region)
	}

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ns_config(namespaceName, fullRegion),
			},
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkGCSConfig(namespaceName, sinkName, region),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "namespace", namespaceName),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.gcs.bucket_name", "test-bucket"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.gcs.region", region),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.gcs.sa_id", "test-sa"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.gcs.gcp_project_id", "test-project"),
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
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, sinkName, region),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.enabled", "false"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "spec.gcs.bucket_name", "updated-bucket"),
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

func testAccNamespaceExportSinkS3Config(namespaceName, region, sinkName string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace_export_sink" "test" {
  namespace = %[1]q
  spec = {
    name    = %[2]q
    enabled = true
    s3 = {
      bucket_name    = "test-bucket"
      region         = %[3]q
      role_name      = "test-role"
      aws_account_id = "123456789012"
      kms_arn        = "arn:aws:kms:us-east-1:123456789012:key/test-key"
    }
  }
}
`, namespaceName, sinkName, region)
}

func testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, sinkName, region string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace_export_sink" "test" {
  namespace = %[1]q
  spec = {
    name    = %[2]q
    enabled = false
    s3 = {
      bucket_name    = "updated-bucket"
      region         = %[3]q
      role_name      = "test-role"
      aws_account_id = "123456789012"
      kms_arn        = "arn:aws:kms:us-east-1:123456789012:key/test-key"
    }
  }
}
`, namespaceName, sinkName, region)
}

func testAccNamespaceExportSinkGCSConfig(namespaceName, sinkName, region string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace_export_sink" "test" {
  namespace = %[1]q
  spec = {
    name    = %[2]q
    enabled = true
    gcs = {
      bucket_name     = "test-bucket"
      region          = %[3]q
      sa_id           = "test-sa"
      gcp_project_id  = "test-project"
    }
  }
}
`, namespaceName, sinkName, region)
}

func testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, sinkName, region string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace_export_sink" "test" {
  namespace = %[1]q
  spec = {
    name    = %[2]q
    enabled = false
	gcs = {
		bucket_name     = "updated-bucket"
		region          = %[3]q
		sa_id           = "test-sa"
		gcp_project_id  = "test-project"
	}
  }
}
`, namespaceName, sinkName, region)
}
