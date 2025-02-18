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
	namespaceName := fmt.Sprintf("tf-test-ns-export-%s", randomString(8))
	region := "us-east-1"
	fullRegion := fmt.Sprintf("aws-%s", region)
	ns_config := func(name string, region string, retention int) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["%s"]
  api_key_auth 	 = true
  retention_days     = %d
}`, name, region, retention)
	}

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ns_config(namespaceName, fullRegion, 1),
			},
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkS3Config(namespaceName, region, sinkName),
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
				Config: testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, region, sinkName),
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
	namespaceName := fmt.Sprintf("tf-test-ns-export-%s", randomString(8))

	region := "us-central1"
	fullRegion := fmt.Sprintf("gcp-%s", region)
	ns_config := func(name string, region string, retention int) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["%s"]
  api_key_auth 	 = true
  retention_days     = %d
}`, name, region, retention)
	}

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ns_config(namespaceName, fullRegion, 1),
			},
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkGCSConfig(namespaceName, region, sinkName),
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
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, region, sinkName),
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
resource "temporalcloud_namespace" "test" {
  name = %[1]q
  regions = %[2]q
}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.test.name
  spec = {
    name    = %[3]q
    enabled = true
    s3 = {
      bucket_name    = "test-bucket"
      region         = "us-east-1"
      role_name      = "test-role"
      aws_account_id = "123456789012"
      kms_arn        = "arn:aws:kms:us-east-1:123456789012:key/test-key"
    }
  }
}
`, namespaceName, region, sinkName)
}

func testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, region, sinkName string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" "test" {
  name = %[1]q
  regions = %[2]q
}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.test.name
  spec = {
    name    = %[3]q
    enabled = false
    s3 = {
      bucket_name    = "updated-bucket"
      region         = "us-east-1"
      role_name      = "test-role"
      aws_account_id = "123456789012"
      kms_arn        = "arn:aws:kms:us-east-1:123456789012:key/test-key"
    }
  }
}
`, namespaceName, region, sinkName)
}

func testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, region, sinkName string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" "test" {
  name = %[1]q
  regions = %[2]q
}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.test.name
  spec = {
    name    = %[3]q
    enabled = false
	gcs = {
		bucket_name     = "updated-bucket"
		region          = "us-central1"
		sa_id           = "test-sa"
		gcp_project_id  = "test-project"
	}
  }
}
`, namespaceName, region, sinkName)
}

func testAccNamespaceExportSinkGCSConfig(namespaceName, region, sinkName string) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" "test" {
  name = %[1]q
  regions = %[2]q
}

resource "temporalcloud_namespace_export_sink" "test" {
  namespace = temporalcloud_namespace.test.name
  spec = {
    name    = %[3]q
    enabled = true
    gcs = {
      bucket_name     = "test-bucket"
      region          = "us-central1"
      sa_id           = "test-sa"
      gcp_project_id  = "test-project"
    }
  }
}
`, namespaceName, region, sinkName)
}
