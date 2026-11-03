package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
	namespacev1 "go.temporal.io/cloud-sdk/api/namespace/v1"
	sinkv1 "go.temporal.io/cloud-sdk/api/sink/v1"
	"google.golang.org/protobuf/proto"
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
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.storage_account", "saascicdexportprod"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.container_name", "saas-cicd-export-prod"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.subscription_id", subscriptionID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.tenant_id", tenantID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.resource_group", "rg-saas-cicd-export-prod"),
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
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.storage_account", "saascicdexportprod"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.container_name", "saas-cicd-export-prod-updated"),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.region", sinkRegion),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.subscription_id", subscriptionID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.tenant_id", tenantID),
					resource.TestCheckResourceAttr("temporalcloud_namespace_export_sink.test", "azure_blob.resource_group", "rg-saas-cicd-export-prod"),
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
  azure_blob = {
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
  azure_blob = {
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

func TestNamespaceExportSink_FilterRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, destination := range []struct {
		name string
		spec *namespacev1.ExportSinkSpec
	}{
		{name: "S3", spec: &namespacev1.ExportSinkSpec{S3: &sinkv1.S3Spec{
			RoleName: "role", BucketName: "bucket", Region: "us-west-2", AwsAccountId: "123456789012",
		}}},
		{name: "GCS", spec: &namespacev1.ExportSinkSpec{Gcs: &sinkv1.GCSSpec{
			SaId: "service-account", BucketName: "bucket", GcpProjectId: "project", Region: "us-central1",
		}}},
		{name: "AzureBlob", spec: &namespacev1.ExportSinkSpec{AzureBlob: &sinkv1.AzureBlobSpec{
			TenantId: "tenant", StorageAccount: "storage", ContainerName: "container",
			Region: "westus", SubscriptionId: "subscription", ResourceGroup: "group",
		}}},
	} {
		for _, tt := range []struct {
			name   string
			filter *namespacev1.ExportSinkSpec_SinkFilter
		}{
			{name: "Absent"},
			{name: "Disabled", filter: &namespacev1.ExportSinkSpec_SinkFilter{
				DisableStandaloneActivityExport: true,
			}},
			{name: "Enabled", filter: &namespacev1.ExportSinkSpec_SinkFilter{}},
		} {
			t.Run(destination.name+"/"+tt.name, func(t *testing.T) {
				expected := proto.Clone(destination.spec).(*namespacev1.ExportSinkSpec)
				expected.Name = "test-sink"
				expected.Enabled = true
				expected.Filter = tt.filter
				var model namespaceExportSinkResourceModel
				diags := updateSinkModelFromSpec(ctx, &model, &namespacev1.ExportSink{
					Name: "test-sink", Spec: expected,
				}, "test.account")
				if diags.HasError() {
					t.Fatalf("read model: %v", diags)
				}
				spec, diags := getSinkSpecFromModel(ctx, &model)
				if diags.HasError() {
					t.Fatalf("build spec: %v", diags)
				}
				// An absent filter has the same meaning as an explicit false.
				if expected.Filter == nil {
					expected.Filter = &namespacev1.ExportSinkSpec_SinkFilter{}
				}
				if !proto.Equal(expected, spec) {
					t.Fatalf("expected %v, got %v", expected, spec)
				}

				model.Filter = types.ObjectNull(internaltypes.ExportSinkFilterModelAttrTypes)
				spec, diags = getSinkSpecFromModel(ctx, &model)
				if diags.HasError() {
					t.Fatalf("omitted filter: %v", diags)
				}
				expected.Filter = &namespacev1.ExportSinkSpec_SinkFilter{}
				if !proto.Equal(expected, spec) {
					t.Fatalf("omitted filter should default to false: expected %v, got %v", expected, spec)
				}

				model.Filter = types.ObjectUnknown(internaltypes.ExportSinkFilterModelAttrTypes)
				spec, diags = getSinkSpecFromModel(ctx, &model)
				if diags.HasError() {
					t.Fatalf("unknown filter: %v", diags)
				}
				if spec.Filter != nil {
					t.Fatalf("unknown filter should be unset, got %v", spec.Filter)
				}
			})
		}
	}
}

func TestNamespaceExportSink_FilterUpgradePlan(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	schemaResp := &fwresource.SchemaResponse{}
	NewNamespaceExportSinkResource().Schema(ctx, fwresource.SchemaRequest{}, schemaResp)
	schema := schemaResp.Schema
	resourceType := schema.Type().TerraformType(ctx)
	for _, tt := range []struct {
		name   string
		filter *namespacev1.ExportSinkSpec_SinkFilter
	}{
		{name: "ServerWithoutFilter"},
		{name: "ServerExportDisabled", filter: &namespacev1.ExportSinkSpec_SinkFilter{
			DisableStandaloneActivityExport: true,
		}},
		{name: "ServerExportEnabled", filter: &namespacev1.ExportSinkSpec_SinkFilter{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server, err := testAccProtoV6ProviderFactories["temporalcloud"]()
			if err != nil {
				t.Fatal(err)
			}
			// State from the old provider has no filter attribute at all.
			upgraded, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{
				TypeName: "temporalcloud_namespace_export_sink", Version: 0,
				RawState: &tfprotov6.RawState{JSON: []byte(`{
                    "id":"test.account,test-sink", "namespace":"test.account", "sink_name":"test-sink",
                    "enabled":true, "gcs":null, "azure_blob":null, "timeouts":null,
                    "s3":{"role_name":"role","bucket_name":"bucket","region":"us-west-2",
                          "aws_account_id":"123456789012","kms_arn":null}
                }`)},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range upgraded.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					t.Fatalf("upgrade: %s: %s", d.Summary, d.Detail)
				}
			}
			raw, err := upgraded.UpgradedState.Unmarshal(resourceType)
			if err != nil {
				t.Fatal(err)
			}
			state := tfsdk.State{Schema: schema, Raw: raw}
			var model namespaceExportSinkResourceModel
			if diags := state.Get(ctx, &model); diags.HasError() {
				t.Fatalf("decode old state: %v", diags)
			}
			if !model.Filter.IsNull() {
				t.Fatal("new attribute should be null in upgraded old state")
			}
			spec, diags := getSinkSpecFromModel(ctx, &model)
			if diags.HasError() {
				t.Fatalf("build old spec: %v", diags)
			}
			spec.Filter = tt.filter
			// Use the same conversion as Read, including the pre-existing timeouts value.
			diags = updateSinkModelFromSpec(ctx, &model, &namespacev1.ExportSink{Name: "test-sink", Spec: spec}, "test.account")
			if diags.HasError() {
				t.Fatalf("refresh: %v", diags)
			}
			if diags := state.Set(ctx, &model); diags.HasError() {
				t.Fatalf("refreshed state: %v", diags)
			}
			prior, err := tfprotov6.NewDynamicValue(resourceType, state.Raw)
			if err != nil {
				t.Fatal(err)
			}

			filterValue := func(value types.Bool) types.Object {
				return types.ObjectValueMust(internaltypes.ExportSinkFilterModelAttrTypes,
					map[string]attr.Value{"disable_standalone_activity_export": value})
			}
			for _, configCase := range []struct {
				name     string
				filter   types.Object
				expected types.Object
			}{
				{name: "Omitted", filter: types.ObjectNull(internaltypes.ExportSinkFilterModelAttrTypes), expected: filterValue(types.BoolValue(false))},
				{name: "Disable", filter: filterValue(types.BoolValue(true)), expected: filterValue(types.BoolValue(true))},
				{name: "Enable", filter: filterValue(types.BoolValue(false)), expected: filterValue(types.BoolValue(false))},
				{name: "EmptyFilterDefaultsToFalse", filter: filterValue(types.BoolNull()), expected: filterValue(types.BoolValue(false))},
			} {
				t.Run(configCase.name, func(t *testing.T) {
					configPlan := tfsdk.Plan{Schema: schema, Raw: raw}
					if diags := configPlan.SetAttribute(ctx, path.Root("id"), types.StringNull()); diags.HasError() {
						t.Fatalf("config id: %v", diags)
					}
					if diags := configPlan.SetAttribute(ctx, path.Root("filter"), configCase.filter); diags.HasError() {
						t.Fatalf("config filter: %v", diags)
					}
					configValue, err := tfprotov6.NewDynamicValue(resourceType, configPlan.Raw)
					if err != nil {
						t.Fatal(err)
					}
					proposed := tfsdk.Plan{Schema: schema, Raw: state.Raw}
					// Start with Terraform's computed-state merge; the parent default must override it when omitted.
					if !configCase.filter.IsNull() {
						if diags := proposed.SetAttribute(ctx, path.Root("filter"), configCase.filter); diags.HasError() {
							t.Fatalf("proposed filter: %v", diags)
						}
					}
					proposedValue, err := tfprotov6.NewDynamicValue(resourceType, proposed.Raw)
					if err != nil {
						t.Fatal(err)
					}
					planned, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
						TypeName: "temporalcloud_namespace_export_sink",
						Config:   &configValue, PriorState: &prior, ProposedNewState: &proposedValue,
					})
					if err != nil {
						t.Fatal(err)
					}
					for _, d := range planned.Diagnostics {
						if d.Severity == tfprotov6.DiagnosticSeverityError {
							t.Fatalf("plan: %s: %s", d.Summary, d.Detail)
						}
					}
					plannedRaw, err := planned.PlannedState.Unmarshal(resourceType)
					if err != nil {
						t.Fatal(err)
					}
					expectedPlan := tfsdk.Plan{Schema: schema, Raw: state.Raw}
					if diags := expectedPlan.SetAttribute(ctx, path.Root("filter"), configCase.expected); diags.HasError() {
						t.Fatalf("expected filter: %v", diags)
					}
					if !plannedRaw.Equal(expectedPlan.Raw) {
						t.Fatalf("unexpected plan: %v", plannedRaw)
					}
					if len(planned.RequiresReplace) != 0 {
						t.Fatalf("plan requires replacement: %v", planned.RequiresReplace)
					}
				})
			}

		})
	}
}
