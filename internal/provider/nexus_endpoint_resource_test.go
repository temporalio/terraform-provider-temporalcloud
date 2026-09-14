package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccNexusEndpointResource(t *testing.T) {
	timeSuffix := time.Now().Format("060102150405")
	endpointName := fmt.Sprintf("tf-nexus-endpoint-%s-%s", timeSuffix, randomString(3))
	description := "test description"
	targetNamespaceName := fmt.Sprintf("tf-nexus-target-%s-%s", timeSuffix, randomString(4))
	taskQueue := "task-queue-1"
	callerNamespaceName := fmt.Sprintf("tf-nexus-caller-%s-%s", timeSuffix, randomString(4))
	callerNamespace2Name := fmt.Sprintf("tf-nexus-caller2-%s-%s", timeSuffix, randomString(3))

	updatedDescription := "updated description"
	updatedTaskQueue := "task-queue-2"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccNexusEndpointResourceConfig(endpointName, description, targetNamespaceName, taskQueue, []string{callerNamespaceName, callerNamespace2Name}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "name", endpointName),
					resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "description", description),
					resource.TestCheckResourceAttrSet("temporalcloud_nexus_endpoint.test", "worker_target.namespace_id"),
					// resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "worker_target.namespace_id", targetNamespaceName + "." + accountID),
					resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "worker_target.task_queue", taskQueue),
					resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "allowed_caller_namespaces.#", "2"),
					resource.TestCheckResourceAttrSet("temporalcloud_nexus_endpoint.test", "id"),
					// Omitting project_id places the endpoint in the account's default project, and
					// the read populates that project's real ID rather than leaving it empty.
					resource.TestCheckResourceAttrSet("temporalcloud_nexus_endpoint.test", "project_id"),
				),
			},
			{
				Config:             testAccNexusEndpointResourceConfig(endpointName, description, targetNamespaceName, taskQueue, []string{callerNamespace2Name, callerNamespaceName}),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			// ImportState testing
			{
				ResourceName:      "temporalcloud_nexus_endpoint.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccNexusEndpointResourceConfig(endpointName, updatedDescription, targetNamespaceName, updatedTaskQueue, []string{callerNamespaceName}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "description", updatedDescription),
					resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "worker_target.task_queue", updatedTaskQueue),
					resource.TestCheckResourceAttr("temporalcloud_nexus_endpoint.test", "allowed_caller_namespaces.#", "1"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccNamespaceResourceConfig(resourceName, name, region string, retentionDays int) string {
	return fmt.Sprintf(`
resource "temporalcloud_namespace" %[1]q {
  name           = %[2]q
  regions        = [%[3]q]
  api_key_auth   = true
  retention_days = %[4]d
  timeouts {
    create = "15m"
    delete = "15m"
  }
}
`, resourceName, name, region, retentionDays)
}

func testAccNexusEndpointResourceConfig(name, description, targetNamespaceName, taskQueue string, allowedNamespaces []string) string {
	region := "aws-ca-central-1"
	retentionDays := 1
	allowedNamespaceIDs := []string{}
	namespacesConfig := testAccNamespaceResourceConfig("target_namespace", targetNamespaceName, region, retentionDays)
	for _, allowedNamespace := range allowedNamespaces {
		namespacesConfig += testAccNamespaceResourceConfig("allowed_namespace_"+allowedNamespace, allowedNamespace, region, retentionDays)
		allowedNamespaceIDs = append(allowedNamespaceIDs, "temporalcloud_namespace.allowed_namespace_"+allowedNamespace+".id")
	}
	allowedNamespaceIDsStr := fmt.Sprintf("[%s]", strings.Join(allowedNamespaceIDs, ", "))

	return fmt.Sprintf(`
%[1]s

# nexus endpoint with all fields
resource "temporalcloud_nexus_endpoint" "test" {
  name        = %[2]q
  description = %[3]q

  worker_target = {
    namespace_id = temporalcloud_namespace.target_namespace.id
    task_queue   = %[4]q
  }

  allowed_caller_namespaces = %[5]s

  timeouts {
    create = "4m"
    delete = "4m"
  }
}

# nexus endpoint with empty description
resource "temporalcloud_nexus_endpoint" "test_2" {
  name        = %[6]q
  description = ""

  worker_target = {
    namespace_id = temporalcloud_namespace.target_namespace.id
    task_queue   = %[4]q
  }

  allowed_caller_namespaces = %[5]s

  timeouts {
    create = "4m"
    delete = "4m"
  }
}

# nexus endpoint without optional fields
resource "temporalcloud_nexus_endpoint" "test_3" {
  name        = %[7]q

  worker_target = {
    namespace_id = temporalcloud_namespace.target_namespace.id
    task_queue   = %[4]q
  }

  allowed_caller_namespaces = %[5]s

  timeouts {
    create = "4m"
    delete = "4m"
  }
}

`, namespacesConfig, name, description, taskQueue, allowedNamespaceIDsStr, name+"-2", name+"-3")
}

func TestAccNexusEndpointResource_Project(t *testing.T) {
	timeSuffix := time.Now().Format("060102150405")
	endpointName := fmt.Sprintf("tf-nexus-proj-%s-%s", timeSuffix, randomString(3))
	targetNamespaceName := fmt.Sprintf("tf-nexus-proj-ns-%s-%s", timeSuffix, randomString(4))
	projectAName := createRandomName()
	projectBName := createRandomName()

	// The endpoint's project and its target namespace's project are independent; the API does not
	// compare them. The namespace here stays in the default project on purpose.
	config := func(projectResource string) string {
		return fmt.Sprintf(`
%[1]s

resource "temporalcloud_project" "project_a" {
  display_name = %[2]q
}

resource "temporalcloud_project" "project_b" {
  display_name = %[3]q
}

resource "temporalcloud_nexus_endpoint" "test_project" {
  name       = %[4]q
  project_id = %[5]s

  worker_target = {
    namespace_id = temporalcloud_namespace.target_namespace.id
    task_queue   = "task-queue-1"
  }

  allowed_caller_namespaces = [temporalcloud_namespace.target_namespace.id]

  timeouts {
    create = "4m"
    delete = "4m"
  }
}
`, testAccNamespaceResourceConfig("target_namespace", targetNamespaceName, "aws-ca-central-1", 1),
			projectAName, projectBName, endpointName, projectResource)
	}

	var firstEndpointID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("temporalcloud_project.project_a.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"temporalcloud_nexus_endpoint.test_project", "project_id",
						"temporalcloud_project.project_a", "id",
					),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["temporalcloud_nexus_endpoint.test_project"]
						if !ok {
							return errors.New("nexus endpoint not found in state")
						}
						firstEndpointID = rs.Primary.ID
						return nil
					},
				),
			},
			{
				ResourceName:      "temporalcloud_nexus_endpoint.test_project",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// project_id is not part of the endpoint spec, so it cannot be updated in place.
				// Rejected at plan time rather than replaced: destroying the endpoint interrupts
				// Nexus callers, so it has to be an explicit choice via -replace.
				Config:      config("temporalcloud_project.project_b.id"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile("cannot be moved between projects"),
			},
			{
				// The endpoint is untouched by the rejected plan: same project, same id.
				Config: config("temporalcloud_project.project_a.id"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"temporalcloud_nexus_endpoint.test_project", "project_id",
						"temporalcloud_project.project_a", "id",
					),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["temporalcloud_nexus_endpoint.test_project"]
						if !ok {
							return errors.New("nexus endpoint not found in state")
						}
						if rs.Primary.ID != firstEndpointID {
							return fmt.Errorf("endpoint id changed from %s to %s; it should not have been recreated", firstEndpointID, rs.Primary.ID)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestNexusEndpointSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	NewNexusEndpointResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

// An explicitly empty project_id is rejected during config validation. Omitting the attribute is
// still valid and means the default project; an empty string would otherwise be planned as a real
// value, while the server substitutes the default project on create, failing the post-apply
// consistency check.
// Hermetic: validation fails before any resource is created, so no namespace or endpoint is made.
func TestAccNexusEndpointResource_EmptyProjectIDRejected(t *testing.T) {
	config := `
provider "temporalcloud" {
}

resource "temporalcloud_nexus_endpoint" "test_empty_project" {
  name       = "tf-nexus-empty-project"
  project_id = ""

  worker_target = {
    namespace_id = "tf-does-not-exist.00000"
    task_queue   = "task-queue-1"
  }

  allowed_caller_namespaces = ["tf-does-not-exist.00000"]
}`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`string length must be at least 1`),
			},
		},
	})
}
