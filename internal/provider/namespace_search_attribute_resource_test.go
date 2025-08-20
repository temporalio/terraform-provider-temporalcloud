package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/temporalio/terraform-provider-temporalcloud/internal/provider/enums"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestSearchAttrSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewNamespaceSearchAttributeResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestAccNamespaceWithSearchAttributes(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-search-attributes", randomString(10))
	config := func(name string, saName string, saType string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  retention_days     = 7
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBzDCCAVKgAwIBAgIQRmAH64LjxSw1SHHpf6qtUTAKBggqhkjOPQQDAzAUMRIw
EAYDVQQKEwl0ZW1wb3JhbDIwHhcNMjQwODE0MDAwNDUyWhcNMjUwODE0MDAwNTUy
WjAUMRIwEAYDVQQKEwl0ZW1wb3JhbDIwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAQG
dddtNqAEafHGLt7jaYQzcoG1FUmxePvQFDRer1vcPEZn+S6nOW/sjMTCHm8XlUFs
kVW7b5Hdh6qUF+AdNSxOply46hbyuM/hDwxRujmjiQ/UR/dae63RNQFYWwYzAZ2j
aTBnMA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTh
lLKpKimriiGgsmuqSXqABANELDAlBgNVHREEHjAcghpjbGllbnQucm9vdC50ZW1w
b3JhbDIuLTF3ejAKBggqhkjOPQQDAwNoADBlAjAci9pnDlRbQxeFffa0+STARaPg
9yWm3Owbb1Fc43hOLkSVvsoC1dBk8uEEmb+wy+QCMQCLx0/+6f5dJZ0zELrBdMBK
Wv+Bi/k7uS5ZUOewkXOMRy5cWs701t2CikmvNJ6m2yA=
-----END CERTIFICATE-----
PEM
)
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "%s"
  type         = "%s"
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute2" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "CustomSearchAttribute2"
  type         = "Text"
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute3" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "CustomSearchAttribute3"
  type         = "text"
}`, name, saName, saType)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config(name, "CustomSearchAttribute", "KeywordList"),
				ExpectError: regexp.MustCompile(enums.ErrInvalidNamespaceSearchAttribute.Error()),
			},
			{
				Config: config(name, "CustomSearchAttribute", "text"),
			},
			{
				Config: config(name, "CustomSearchAttribute9", "text"),
			},
		},
	})
}

func TestAccNamespaceImportSearchAttribute(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-search-attribute-import", randomString(10))
	config := func(name string, saName string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  retention_days     = 7
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBzDCCAVKgAwIBAgIQRmAH64LjxSw1SHHpf6qtUTAKBggqhkjOPQQDAzAUMRIw
EAYDVQQKEwl0ZW1wb3JhbDIwHhcNMjQwODE0MDAwNDUyWhcNMjUwODE0MDAwNTUy
WjAUMRIwEAYDVQQKEwl0ZW1wb3JhbDIwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAQG
dddtNqAEafHGLt7jaYQzcoG1FUmxePvQFDRer1vcPEZn+S6nOW/sjMTCHm8XlUFs
kVW7b5Hdh6qUF+AdNSxOply46hbyuM/hDwxRujmjiQ/UR/dae63RNQFYWwYzAZ2j
aTBnMA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTh
lLKpKimriiGgsmuqSXqABANELDAlBgNVHREEHjAcghpjbGllbnQucm9vdC50ZW1w
b3JhbDIuLTF3ejAKBggqhkjOPQQDAwNoADBlAjAci9pnDlRbQxeFffa0+STARaPg
9yWm3Owbb1Fc43hOLkSVvsoC1dBk8uEEmb+wy+QCMQCLx0/+6f5dJZ0zELrBdMBK
Wv+Bi/k7uS5ZUOewkXOMRy5cWs701t2CikmvNJ6m2yA=
-----END CERTIFICATE-----
PEM
)
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "%s"
  type         = "text"
}
`, name, saName)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, "CustomSearchAttribute"),
			},
			{
				ResourceName: "temporalcloud_namespace_search_attribute.custom_search_attribute",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					ns := s.Modules[0].Resources["temporalcloud_namespace.terraform"]
					id := ns.Primary.Attributes["id"]
					return fmt.Sprintf("%s/%s", id, "CustomSearchAttribute"), nil
				},
			},
		},
	})
}

func TestAccNamespaceWithSearchAttributesUpdate(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-search-attributes", randomString(10))
	config := func(name string, retentionDays int) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  retention_days     = %d
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBzDCCAVKgAwIBAgIQRmAH64LjxSw1SHHpf6qtUTAKBggqhkjOPQQDAzAUMRIw
EAYDVQQKEwl0ZW1wb3JhbDIwHhcNMjQwODE0MDAwNDUyWhcNMjUwODE0MDAwNTUy
WjAUMRIwEAYDVQQKEwl0ZW1wb3JhbDIwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAQG
dddtNqAEafHGLt7jaYQzcoG1FUmxePvQFDRer1vcPEZn+S6nOW/sjMTCHm8XlUFs
kVW7b5Hdh6qUF+AdNSxOply46hbyuM/hDwxRujmjiQ/UR/dae63RNQFYWwYzAZ2j
aTBnMA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTh
lLKpKimriiGgsmuqSXqABANELDAlBgNVHREEHjAcghpjbGllbnQucm9vdC50ZW1w
b3JhbDIuLTF3ejAKBggqhkjOPQQDAwNoADBlAjAci9pnDlRbQxeFffa0+STARaPg
9yWm3Owbb1Fc43hOLkSVvsoC1dBk8uEEmb+wy+QCMQCLx0/+6f5dJZ0zELrBdMBK
Wv+Bi/k7uS5ZUOewkXOMRy5cWs701t2CikmvNJ6m2yA=
-----END CERTIFICATE-----
PEM
)
}

resource "temporalcloud_namespace_search_attribute" "custom_search_attribute" {
  namespace_id = temporalcloud_namespace.terraform.id
  name         = "CustomSearchAttribute"
  type         = "text"
}
`, name, retentionDays)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name, 14),
			},
			{
				Config: config(name, 15),
			},
		},
	})
}
