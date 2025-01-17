package provider

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNexusEndpointResource(t *testing.T) {
	timeSuffix := time.Now().Format("060102150405")
	endpointName := fmt.Sprintf("tf-nexus-endpoint-%s-%s", timeSuffix, randomStringWithLength(3))
	description := "test description"
	targetNamespaceName := fmt.Sprintf("tf-nexus-target-%s-%s", timeSuffix, randomStringWithLength(4))
	taskQueue := "task-queue-1"
	callerNamespaceName := fmt.Sprintf("tf-nexus-caller-%s-%s", timeSuffix, randomStringWithLength(4))
	callerNamespace2Name := fmt.Sprintf("tf-nexus-caller2-%s-%s", timeSuffix, randomStringWithLength(3))

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
	region := "aws-us-west-2"
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
`, namespacesConfig, name, description, taskQueue, allowedNamespaceIDsStr)
}
