package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"
	"text/template"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	cloudservicev1 "go.temporal.io/cloud-sdk/api/cloudservice/v1"

	"github.com/temporalio/terraform-provider-temporalcloud/internal/client"
)

func TestNamespaceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwresource.SchemaRequest{}
	schemaResponse := &fwresource.SchemaResponse{}

	// Instantiate the resource.Resource and call its Schema method
	NewNamespaceResource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

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
MIIBxjCCAU2gAwIBAgIRAlyZ5KUmunPLeFAupDwGL8AwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNDA4MTMyMzQ2NThaFw0yNTA4MTMyMzQ3NTha
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARG+EuL
uKRsNWs7Rbz6ciaJQB7QINTRLmTgGGE8H/wAs+KjvctjPdDdqFPZrxShRY3PUdk2
pgQKRugMTe3N52pxBx4Iablz8felfdv4kyLQbdsJzY9XmCYX3D68/9Hxsl2jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSYC5/u
K78bK1M8Fv1M6ELMjF2ZMDAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjBycDUwCgYIKoZIzj0EAwMDZwAwZAIwSycjxxmYTgV5eSJbaGMINr5LQgyKQUHQ
ryBKSGLKASa/e2ntyhsqRhj77gJ8DmkZAjAIlpDacF+Sq1kpZ5tMV7ZLElcujzj4
US8pEmNuIiCguEGwi+pb5CWfabETEHApxmo=
-----END CERTIFICATE-----
PEM
)

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

func TestAccBasicNamespaceWithApiKeyAuth(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-basic-namespace", randomString(10))
	config := func(name string, retention int) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
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
MIIBxjCCAU2gAwIBAgIRAlyZ5KUmunPLeFAupDwGL8AwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNDA4MTMyMzQ2NThaFw0yNTA4MTMyMzQ3NTha
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARG+EuL
uKRsNWs7Rbz6ciaJQB7QINTRLmTgGGE8H/wAs+KjvctjPdDdqFPZrxShRY3PUdk2
pgQKRugMTe3N52pxBx4Iablz8felfdv4kyLQbdsJzY9XmCYX3D68/9Hxsl2jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSYC5/u
K78bK1M8Fv1M6ELMjF2ZMDAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjBycDUwCgYIKoZIzj0EAwMDZwAwZAIwSycjxxmYTgV5eSJbaGMINr5LQgyKQUHQ
ryBKSGLKASa/e2ntyhsqRhj77gJ8DmkZAjAIlpDacF+Sq1kpZ5tMV7ZLElcujzj4
US8pEmNuIiCguEGwi+pb5CWfabETEHApxmo=
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
				ResourceName:      "temporalcloud_namespace.test",
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
			Name                 string
			RetentionDays        int
			CodecServer          *codecServer
			ApiKeyAuth           bool
			CertFiltersEmptyList bool
		}
	)

	name := fmt.Sprintf("%s-%s", "tf-codec-server", randomString(10))
	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "{{ .Name }}-{{ .ApiKeyAuth }}"
  regions            = ["aws-us-east-1"]

	  {{ if .ApiKeyAuth }}
	  api_key_auth = true
	  {{ end }}

	{{ if not .ApiKeyAuth }}
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxjCCAU2gAwIBAgIRAlyZ5KUmunPLeFAupDwGL8AwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNDA4MTMyMzQ2NThaFw0yNTA4MTMyMzQ3NTha
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARG+EuL
uKRsNWs7Rbz6ciaJQB7QINTRLmTgGGE8H/wAs+KjvctjPdDdqFPZrxShRY3PUdk2
pgQKRugMTe3N52pxBx4Iablz8felfdv4kyLQbdsJzY9XmCYX3D68/9Hxsl2jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSYC5/u
K78bK1M8Fv1M6ELMjF2ZMDAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjBycDUwCgYIKoZIzj0EAwMDZwAwZAIwSycjxxmYTgV5eSJbaGMINr5LQgyKQUHQ
ryBKSGLKASa/e2ntyhsqRhj77gJ8DmkZAjAIlpDacF+Sq1kpZ5tMV7ZLElcujzj4
US8pEmNuIiCguEGwi+pb5CWfabETEHApxmo=
-----END CERTIFICATE-----
PEM
)
	{{ end }}

  retention_days     = {{ .RetentionDays }}

  {{ if .CertFiltersEmptyList }}
  certificate_filters = []
  {{ end }}

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

	resource.ParallelTest(t, resource.TestCase{
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
				// Error on empty cert filers
				Config: config(configArgs{
					Name:                 name,
					RetentionDays:        7,
					CertFiltersEmptyList: true,
				}),
				ExpectError: regexp.MustCompile("certificate_filters list must contain at least 1 elements"),
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
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_namespace.test",
			},
			{
				// remove codec server
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
			// use API key auth
			{
				Config: config(configArgs{
					Name:          name,
					RetentionDays: 7,
					CodecServer: &codecServer{
						Endpoint:                      "https://example.com",
						PassAccessToken:               true,
						IncludeCrossOriginCredentials: true,
					},
					ApiKeyAuth: true,
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
				// remove codec server
				Config: config(configArgs{
					Name:          name,
					RetentionDays: 7,
					ApiKeyAuth:    true,
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

func TestAccNamespaceRenameForcesReplacement(t *testing.T) {
	oldName := fmt.Sprintf("%s-%s", "tf-rename-replace", randomString(10))
	newName := fmt.Sprintf("%s-%s", "tf-rename-replace-new", randomString(10))
	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {
}
resource "temporalcloud_namespace" "test" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxjCCAU2gAwIBAgIRAlyZ5KUmunPLeFAupDwGL8AwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNDA4MTMyMzQ2NThaFw0yNTA4MTMyMzQ3NTha
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARG+EuL
uKRsNWs7Rbz6ciaJQB7QINTRLmTgGGE8H/wAs+KjvctjPdDdqFPZrxShRY3PUdk2
pgQKRugMTe3N52pxBx4Iablz8felfdv4kyLQbdsJzY9XmCYX3D68/9Hxsl2jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSYC5/u
K78bK1M8Fv1M6ELMjF2ZMDAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjBycDUwCgYIKoZIzj0EAwMDZwAwZAIwSycjxxmYTgV5eSJbaGMINr5LQgyKQUHQ
ryBKSGLKASa/e2ntyhsqRhj77gJ8DmkZAjAIlpDacF+Sq1kpZ5tMV7ZLElcujzj4
US8pEmNuIiCguEGwi+pb5CWfabETEHApxmo=
-----END CERTIFICATE-----
PEM
)
  retention_days     = 7
}
`, name)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(oldName),
			},
			{
				Config: config(newName),
			},
		},
	})
}

func TestAccNamespaceImport(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-rename-replace", randomString(10))
	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {
}
resource "temporalcloud_namespace" "test" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIBxjCCAU2gAwIBAgIRAlyZ5KUmunPLeFAupDwGL8AwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNDA4MTMyMzQ2NThaFw0yNTA4MTMyMzQ3NTha
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARG+EuL
uKRsNWs7Rbz6ciaJQB7QINTRLmTgGGE8H/wAs+KjvctjPdDdqFPZrxShRY3PUdk2
pgQKRugMTe3N52pxBx4Iablz8felfdv4kyLQbdsJzY9XmCYX3D68/9Hxsl2jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSYC5/u
K78bK1M8Fv1M6ELMjF2ZMDAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjBycDUwCgYIKoZIzj0EAwMDZwAwZAIwSycjxxmYTgV5eSJbaGMINr5LQgyKQUHQ
ryBKSGLKASa/e2ntyhsqRhj77gJ8DmkZAjAIlpDacF+Sq1kpZ5tMV7ZLElcujzj4
US8pEmNuIiCguEGwi+pb5CWfabETEHApxmo=
-----END CERTIFICATE-----
PEM
)
  retention_days     = 7
}
`, name)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name),
			},
			{
				ResourceName:      "temporalcloud_namespace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccSpacesBetweenCertificateStrings(t *testing.T) {
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
MIIBxjCCAU2gAwIBAgIRAlyZ5KUmunPLeFAupDwGL8AwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNDA4MTMyMzQ2NThaFw0yNTA4MTMyMzQ3NTha
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAARG+EuL
uKRsNWs7Rbz6ciaJQB7QINTRLmTgGGE8H/wAs+KjvctjPdDdqFPZrxShRY3PUdk2
pgQKRugMTe3N52pxBx4Iablz8felfdv4kyLQbdsJzY9XmCYX3D68/9Hxsl2jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSYC5/u
K78bK1M8Fv1M6ELMjF2ZMDAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjBycDUwCgYIKoZIzj0EAwMDZwAwZAIwSycjxxmYTgV5eSJbaGMINr5LQgyKQUHQ
ryBKSGLKASa/e2ntyhsqRhj77gJ8DmkZAjAIlpDacF+Sq1kpZ5tMV7ZLElcujzj4
US8pEmNuIiCguEGwi+pb5CWfabETEHApxmo=
-----END CERTIFICATE-----


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

	return client.CloudService()
}
