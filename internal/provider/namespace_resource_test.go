package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBasicNamespace(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-basic-namespace", randomString(10))
	config := func(name string, retention int) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIByTCCAVCgAwIBAgIRAWHkC+6JUf3s9Tq43mdp2zgwCgYIKoZIzj0EAwMwEzER
MA8GA1UEChMIdGVtcG9yYWwwHhcNMjMwODEwMDAwOTQ1WhcNMjQwODA5MDAxMDQ1
WjATMREwDwYDVQQKEwh0ZW1wb3JhbDB2MBAGByqGSM49AgEGBSuBBAAiA2IABCzQ
7DwwGSQKM6Zrx3Qtw7IubfxiJ3RSXCqmcGhEbFVeocwAdEgMYlwSlUiWtDZVR2dM
XM9UZLWK4aGGnDNS5Mhcz6ibSBS7Owf4tRZZA9SpFCjNw2HraaiUVV+EUgxoe6No
MGYwDgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFG4N
8lIXqQKxwVs/ixVzdF6XGZm+MCQGA1UdEQQdMBuCGWNsaWVudC5yb290LnRlbXBv
cmFsLlB1VHMwCgYIKoZIzj0EAwMDZwAwZAIwRLfm9S7rKGd30KdQvUMcOcDJlmDw
6/oM6UOJFxLeGcpYbgxQ/bFize+Yx9Q9kNeMAjA7GiFsaipaKtWHy5MCOCas3ZP6
+ttLaXNXss3Z5Wk5vhDQnyE8JR3rPeQ2cHXLiA0=
-----END CERTIFICATE-----
PEM
)

  retention_days     = %d
}`, name, retention)
	}

	resource.Test(t, resource.TestCase{
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
			// Delete testing automatically occurs in TestCase
		},
	})

}

func TestAccBasicNamespaceWithCertFilters(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-cert-filters", randomString(10))
	config := func(name string, retention int) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIByTCCAVCgAwIBAgIRAWHkC+6JUf3s9Tq43mdp2zgwCgYIKoZIzj0EAwMwEzER
MA8GA1UEChMIdGVtcG9yYWwwHhcNMjMwODEwMDAwOTQ1WhcNMjQwODA5MDAxMDQ1
WjATMREwDwYDVQQKEwh0ZW1wb3JhbDB2MBAGByqGSM49AgEGBSuBBAAiA2IABCzQ
7DwwGSQKM6Zrx3Qtw7IubfxiJ3RSXCqmcGhEbFVeocwAdEgMYlwSlUiWtDZVR2dM
XM9UZLWK4aGGnDNS5Mhcz6ibSBS7Owf4tRZZA9SpFCjNw2HraaiUVV+EUgxoe6No
MGYwDgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFG4N
8lIXqQKxwVs/ixVzdF6XGZm+MCQGA1UdEQQdMBuCGWNsaWVudC5yb290LnRlbXBv
cmFsLlB1VHMwCgYIKoZIzj0EAwMDZwAwZAIwRLfm9S7rKGd30KdQvUMcOcDJlmDw
6/oM6UOJFxLeGcpYbgxQ/bFize+Yx9Q9kNeMAjA7GiFsaipaKtWHy5MCOCas3ZP6
+ttLaXNXss3Z5Wk5vhDQnyE8JR3rPeQ2cHXLiA0=
-----END CERTIFICATE-----
PEM
)

  certificate_filters = [
	{
	  subject_alternative_name = "example.com"
	}
  ]
  retention_days     = %d

}
	`, name, retention)

	}

	resource.Test(t, resource.TestCase{
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
			// Delete testing automatically occurs in TestCase
		},
	})
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
