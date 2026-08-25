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
  regions            = ["aws-ca-central-1"]
  retention_days     = 7
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxzCCAUygAwIBAgIQTDDt0NmKHovp4Wnapcup5zAKBggqhkjOPQQDAzASMRAw
DgYDVQQKEwd0ZXN0aW5nMB4XDTI2MDgyNTE5MjQzN1oXDTI3MDgyNTE5MjUzN1ow
EjEQMA4GA1UEChMHdGVzdGluZzB2MBAGByqGSM49AgEGBSuBBAAiA2IABEHNFYQ7
Sg2tw+zMv/iFjitL8NC/SV/y1h98hiKD8pkuEoWcrd7ZZAzM4xxJv0OIwOVrZT/U
9jA/y82W/SmWskjauStJPIkhSaO4CZdF2EbNJN4I2ae5iSuMXtQfRx1T8qNnMGUw
DgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFK/qJ3Kg
eXKG3wewJF9sf0AEy3TJMCMGA1UdEQQcMBqCGGNsaWVudC5yb290LnRlc3Rpbmcu
SnZkazAKBggqhkjOPQQDAwNpADBmAjEAmXCXw/w998v0nwx7JgG4awX9bm8joncQ
NCZ7EYQzibLk0gHCOy/i4hIt8EtqN4anAjEA55XoJnLEo+p65Gg2jc/xtY8t1O00
fKxp60bf8UE1MWAyXI/pXYrsFHLAbN64XZuY
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
  regions            = ["aws-ca-central-1"]
  retention_days     = 7
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxzCCAUygAwIBAgIQTDDt0NmKHovp4Wnapcup5zAKBggqhkjOPQQDAzASMRAw
DgYDVQQKEwd0ZXN0aW5nMB4XDTI2MDgyNTE5MjQzN1oXDTI3MDgyNTE5MjUzN1ow
EjEQMA4GA1UEChMHdGVzdGluZzB2MBAGByqGSM49AgEGBSuBBAAiA2IABEHNFYQ7
Sg2tw+zMv/iFjitL8NC/SV/y1h98hiKD8pkuEoWcrd7ZZAzM4xxJv0OIwOVrZT/U
9jA/y82W/SmWskjauStJPIkhSaO4CZdF2EbNJN4I2ae5iSuMXtQfRx1T8qNnMGUw
DgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFK/qJ3Kg
eXKG3wewJF9sf0AEy3TJMCMGA1UdEQQcMBqCGGNsaWVudC5yb290LnRlc3Rpbmcu
SnZkazAKBggqhkjOPQQDAwNpADBmAjEAmXCXw/w998v0nwx7JgG4awX9bm8joncQ
NCZ7EYQzibLk0gHCOy/i4hIt8EtqN4anAjEA55XoJnLEo+p65Gg2jc/xtY8t1O00
fKxp60bf8UE1MWAyXI/pXYrsFHLAbN64XZuY
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
  regions            = ["aws-ca-central-1"]
  retention_days     = %d
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxzCCAUygAwIBAgIQTDDt0NmKHovp4Wnapcup5zAKBggqhkjOPQQDAzASMRAw
DgYDVQQKEwd0ZXN0aW5nMB4XDTI2MDgyNTE5MjQzN1oXDTI3MDgyNTE5MjUzN1ow
EjEQMA4GA1UEChMHdGVzdGluZzB2MBAGByqGSM49AgEGBSuBBAAiA2IABEHNFYQ7
Sg2tw+zMv/iFjitL8NC/SV/y1h98hiKD8pkuEoWcrd7ZZAzM4xxJv0OIwOVrZT/U
9jA/y82W/SmWskjauStJPIkhSaO4CZdF2EbNJN4I2ae5iSuMXtQfRx1T8qNnMGUw
DgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFK/qJ3Kg
eXKG3wewJF9sf0AEy3TJMCMGA1UdEQQcMBqCGGNsaWVudC5yb290LnRlc3Rpbmcu
SnZkazAKBggqhkjOPQQDAwNpADBmAjEAmXCXw/w998v0nwx7JgG4awX9bm8joncQ
NCZ7EYQzibLk0gHCOy/i4hIt8EtqN4anAjEA55XoJnLEo+p65Gg2jc/xtY8t1O00
fKxp60bf8UE1MWAyXI/pXYrsFHLAbN64XZuY
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
