package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBasicAccountMetrics(t *testing.T) {
	config := func(cert string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_metrics_endpoint" "terraform" {
	accepted_client_ca  = base64encode(<<PEM
%s
PEM
) 
}`, cert)
	}

	cert, err := generateTestCACertificate("temporal-terraform-test")
	if err != nil {
		t.Fatal(err)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(string(cert)),
			},
		},
	})
}
