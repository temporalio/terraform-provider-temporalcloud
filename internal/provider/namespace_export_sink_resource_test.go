package provider

import (
	"context"
	"fmt"
	"os"
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
	awsAccountID := os.Getenv("INTEGRATION_TEST_AWS_ACCOUNT_ID")

	namespaceName := fmt.Sprintf("tf-test-ns-export-aws-%s", randomString(8))
	sinkRegion := "ca-central-1"
	namespaceRegion := fmt.Sprintf("aws-%s", sinkRegion)
	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if awsAccountID == "" {
				t.Fatal("INTEGRATION_TEST_AWS_ACCOUNT_ID must be set for S3 export sink tests")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkS3Config(namespaceName, sinkName, namespaceRegion, sinkRegion, awsAccountID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.bucket_name", "cloud-cicd-export-prod-cacentral1"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.role_name", "cloud-cicd-export-external-trust-prod-cacentral1"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.aws_account_id", awsAccountID),
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
				Config: testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, awsAccountID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "false"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.bucket_name", "cloud-cicd-export-prod-cacentral1-updated"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.role_name", "cloud-cicd-export-external-trust-prod-cacentral1"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "s3.aws_account_id", awsAccountID),
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
	gcpProjectID := os.Getenv("INTEGRATION_TEST_GCP_PROJECT_ID")

	namespaceName := fmt.Sprintf("tf-test-ns-export-gcp-%s", randomString(8))
	sinkRegion := "us-central1"
	namespaceRegion := fmt.Sprintf("gcp-%s", sinkRegion)

	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	creationGCSCheckFun := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "sink_name", sinkName),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "true"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.bucket_name", "prod-export-saas-cicd"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.region", sinkRegion),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.service_account_id", "export-prod"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.gcp_project_id", gcpProjectID),
	)

	updateGCSCheckFun := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "false"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.bucket_name", "prod-export-saas-cicd-updated"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.region", sinkRegion),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.service_account_id", "export-prod"),
		resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "gcs.gcp_project_id", gcpProjectID),
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if gcpProjectID == "" {
				t.Fatal("INTEGRATION_TEST_GCP_PROJECT_ID must be set for GCS export sink tests")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkGCSConfig(namespaceName, namespaceRegion, sinkName, sinkRegion, gcpProjectID, false),
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
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, gcpProjectID, true),
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
				Config: testAccNamespaceExportSinkGCSConfig(namespaceName, namespaceRegion, sinkName, sinkRegion, gcpProjectID, true),
				Check:  creationGCSCheckFun,
			},
			// Update with not SA email
			{
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, gcpProjectID, false),
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
				Config: testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, gcpProjectID, true),
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

func TestAccNamespaceExportSink_Azure(t *testing.T) {
	subscriptionID := os.Getenv("INTEGRATION_TEST_AZURE_SUBSCRIPTION_ID")
	tenantID := os.Getenv("INTEGRATION_TEST_AZURE_TENANT_ID")

	namespaceName := fmt.Sprintf("tf-test-ns-export-azure-%s", randomString(8))
	sinkRegion := "centralus"
	namespaceRegion := fmt.Sprintf("azure-%s", sinkRegion)
	sinkName := fmt.Sprintf("tf-test-sink-%s", randomString(8))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if subscriptionID == "" || tenantID == "" {
				t.Fatal("INTEGRATION_TEST_AZURE_SUBSCRIPTION_ID and INTEGRATION_TEST_AZURE_TENANT_ID must be set for Azure export sink tests")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccNamespaceExportSinkAzureConfig(namespaceName, sinkName, namespaceRegion, sinkRegion, subscriptionID, tenantID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "sink_name", sinkName),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "true"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.storage_account", "saascicdexportprod"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.container_name", "saas-cicd-export-prod"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.subscription_id", subscriptionID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.tenant_id", tenantID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.resource_group", "rg-saas-cicd-export-prod"),
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
				Config: testAccNamespaceExportSinkAzureConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, subscriptionID, tenantID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "enabled", "false"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.storage_account", "saascicdexportprod"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.container_name", "saas-cicd-export-prod-updated"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.subscription_id", subscriptionID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.tenant_id", tenantID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure.resource_group", "rg-saas-cicd-export-prod"),
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

func testAccNamespaceExportSinkAzureConfig(namespaceName, sinkName, namespaceRegion, sinkRegion, subscriptionID, tenantID string) string {
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
  azure = {
    storage_account = "saascicdexportprod"
    container_name  = "saas-cicd-export-prod"
    region          = %[4]q
    subscription_id = %[5]q
    tenant_id       = %[6]q
    resource_group  = "rg-saas-cicd-export-prod"
  }

}
`, namespaceName, namespaceRegion, sinkName, sinkRegion, subscriptionID, tenantID)
}

func testAccNamespaceExportSinkAzureConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, subscriptionID, tenantID string) string {
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
  azure = {
    storage_account = "saascicdexportprod"
    container_name  = "saas-cicd-export-prod-updated"
    region          = %[4]q
    subscription_id = %[5]q
    tenant_id       = %[6]q
    resource_group  = "rg-saas-cicd-export-prod"
  }
}
`, namespaceName, namespaceRegion, sinkName, sinkRegion, subscriptionID, tenantID)
}

func testAccNamespaceExportSinkS3Config(namespaceName, sinkName, namespaceRegion, sinkregion, awsAccountID string) string {
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
    bucket_name    = "cloud-cicd-export-prod-cacentral1"
    region         = %[4]q
    role_name      = "cloud-cicd-export-external-trust-prod-cacentral1"
    aws_account_id = %[5]q
  }

}
`, namespaceName, namespaceRegion, sinkName, sinkregion, awsAccountID)
}

func testAccNamespaceExportSinkS3ConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, awsAccountID string) string {
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
    bucket_name    = "cloud-cicd-export-prod-cacentral1-updated"
    region         = %[4]q
    role_name      = "cloud-cicd-export-external-trust-prod-cacentral1"
    aws_account_id = %[5]q
  }
}
`, namespaceName, namespaceRegion, sinkName, sinkRegion, awsAccountID)
}

func testAccNamespaceExportSinkGCSConfig(namespaceName, namespaceRegion, sinkName, sinkRegion, gcpProjectID string, isSAEmail bool) string {
	var export_config string
	if !isSAEmail {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name         = "prod-export-saas-cicd"
    region              = %[1]q
    service_account_id  = "export-prod"
    gcp_project_id      = %[2]q
  }	
`, sinkRegion, gcpProjectID)
	} else {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name     = "prod-export-saas-cicd"
    region          = %[1]q
    service_account_email = "export-prod@%[2]s.iam.gserviceaccount.com"
  }
`, sinkRegion, gcpProjectID)
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

func testAccNamespaceExportSinkGCSConfigUpdate(namespaceName, namespaceRegion, sinkName, sinkRegion, gcpProjectID string, isSAEmail bool) string {
	var export_config string
	if !isSAEmail {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name         = "prod-export-saas-cicd-updated"
    region              = %[1]q
    service_account_id  = "export-prod"
    gcp_project_id      = %[2]q
  }
`, sinkRegion, gcpProjectID)
	} else {
		export_config = fmt.Sprintf(`
  gcs = {
    bucket_name     = "prod-export-saas-cicd-updated"
    region          = %[1]q
    service_account_email = "export-prod@%[2]s.iam.gserviceaccount.com"
  }
`, sinkRegion, gcpProjectID)
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
