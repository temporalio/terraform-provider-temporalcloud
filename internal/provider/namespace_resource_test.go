package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"text/template"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
	cloudservicev1 "github.com/temporalio/terraform-provider-temporalcloud/proto/go/temporal/api/cloud/cloudservice/v1"
)

func TestAccBasicNamespace(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestAccNamespaceWithCodecServer(t *testing.T) {
	type (
		codecServer struct {
			Endpoint                      string
			PassAccessToken               bool
			IncludeCrossOriginCredentials bool
		}

		configArgs struct {
			Name          string
			RetentionDays int
			CodecServer   *codecServer
		}
	)

	t.Parallel()
	name := fmt.Sprintf("%s-%s", "tf-codec-server", randomString(10))
	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "{{ .Name }}"
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

  retention_days     = {{ .RetentionDays }}

  {{ with .CodecServer }}
  codec_server = {
    endpoint                         = "{{ .Endpoint }}"
    pass_access_token                = {{ .PassAccessToken }}
    include_cross_origin_credentials = {{ .IncludeCrossOriginCredentials }}
  }
  {{ end }}
}`))

	config := func(args configArgs) string {
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		if err := tmpl.Execute(writer, args); err != nil {
			t.Errorf("failed to execute template: %v", err)
			t.FailNow()
		}

		writer.Flush()
		return buf.String()
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					Name:          name,
					RetentionDays: 7,
				}),
			},
			{
				Config: config(configArgs{
					Name:          name,
					RetentionDays: 7,
					CodecServer: &codecServer{
						Endpoint:                      "https://example.com",
						PassAccessToken:               true,
						IncludeCrossOriginCredentials: true,
					},
				}),
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					conn := newConnection(t)
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := ns.Namespace.GetSpec()
					if spec.GetCodecServer().GetEndpoint() != "https://example.com" {
						return fmt.Errorf("unexpected endpoint: %s", spec.GetCodecServer().GetEndpoint())
					}
					if !spec.GetCodecServer().GetPassAccessToken() {
						return errors.New("expected pass_access_token to be true")
					}
					if !spec.GetCodecServer().GetIncludeCrossOriginCredentials() {
						return errors.New("expected include_cross_origin_credentials to be true")
					}
					return nil
				},
			},
			{
				Config: config(configArgs{
					Name:          name,
					RetentionDays: 7,
				}),
				Check: func(s *terraform.State) error {
					id := s.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					conn := newConnection(t)
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := ns.Namespace.GetSpec()
					if spec.GetCodecServer().GetEndpoint() != "" {
						return fmt.Errorf("unexpected endpoint: %s", spec.GetCodecServer().GetEndpoint())
					}
					return nil
				},
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

	client, err := client.NewConnectionWithAPIKey(endpoint, allowInsecure, apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}

func randomString(length int) string {
	r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	const charset = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}
