package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

func TestAccBasicNamespaceWithApiKeyAuth(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-basic-namespace", randomString(10))
	config := func(name string, retention int) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1", "aws-us-east-2"]
  api_key_auth 	 = true
  retention_days     = %d
}`, name, retention)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// New namespace with retention of 7
				Config: config(name, 7),
			},
			{
				Config: config(name, 14),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_namespace.terraform",
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func newConnection(t *testing.T) cloudservicev1.CloudServiceClient {
	apiKey := os.Getenv("TEMPORAL_CLOUD_API_KEY")
	endpoint := os.Getenv("TEMPORAL_CLOUD_ENDPOINT")
	if endpoint == "" {
		endpoint = "saas-api.tmprl.cloud:443"
	}
	allowInsecure := os.Getenv("TEMPORAL_CLOUD_ALLOW_INSECURE") == "true"
	client, err := client.NewConnectionWithAPIKey(endpoint, allowInsecure, apiKey, "test")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client.CloudService()
}
