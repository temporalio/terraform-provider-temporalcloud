package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
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
			TLSAuth              bool
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

	{{ if .TLSAuth }}
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
				ExpectError: regexp.MustCompile("Namespace not configured with authentication"),
			},
			{
				// Error on empty cert filers
				Config: config(configArgs{
					Name:                 name,
					RetentionDays:        7,
					TLSAuth:              true,
					CertFiltersEmptyList: true,
				}),
				ExpectError: regexp.MustCompile("certificate_filters list must contain at least 1 elements"),
			},
			{
				Config: config(configArgs{
					Name:          name,
					RetentionDays: 7,
					TLSAuth:       true,
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
					TLSAuth:       true,
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
			{
				// both auth methods
				Config: config(configArgs{
					Name:          name,
					RetentionDays: 7,
					ApiKeyAuth:    true,
					TLSAuth:       true,
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
	client, err := client.NewConnectionWithAPIKey(endpoint, allowInsecure, apiKey, "test")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client.CloudService()
}

func TestAccNamespaceWithConnectivityRuleIds(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-connectivity-rules", randomString(10))
	allRules := []string{"rule1", "rule2", "rule3", "rule4"}

	// Configuration for namespace with specific connectivity rules
	config := func(name string, includeRules []string, retentionDays int) string {
		var connectivityRulesConfig string
		var ruleRefs []string

		// Create all connectivity rule resources (same as rulesOnlyConfig but with namespace)
		rulesResources := ""
		for _, ruleName := range allRules {
			rulesResources += fmt.Sprintf(`
	resource "temporalcloud_connectivity_rule" "%s" {
	  connectivity_type = "private"
	  connection_id     = "vpce-tftest%s"
	  region            = "aws-us-east-1"
	}
	`, ruleName, ruleName)
		}

		// Reference the rules specified in includeRules in the namespace
		if len(includeRules) > 0 {
			for _, ruleName := range includeRules {
				ruleRefs = append(ruleRefs, fmt.Sprintf("temporalcloud_connectivity_rule.%s.id", ruleName))
			}
			connectivityRulesConfig = fmt.Sprintf("connectivity_rule_ids = [%s]", strings.Join(ruleRefs, ", "))
		}

		config := fmt.Sprintf(`
	provider "temporalcloud" {}
	
	%s
		
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
	  retention_days     = %d
	  %s
	}
	`, rulesResources, name, retentionDays, connectivityRulesConfig)
		return config
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create namespace with connectivity rule IDs
				Config: config(name, allRules[:2], 1),
				Check: func(s *terraform.State) error {
					namespaceId := s.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					rule1Id := s.RootModule().Resources["temporalcloud_connectivity_rule.rule1"].Primary.Attributes["id"]
					rule2Id := s.RootModule().Resources["temporalcloud_connectivity_rule.rule2"].Primary.Attributes["id"]

					conn := newConnection(t)
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: namespaceId,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := ns.Namespace.GetSpec()
					expectedRules := []string{rule1Id, rule2Id}
					actualRules := spec.GetConnectivityRuleIds()

					if len(actualRules) != len(expectedRules) {
						return fmt.Errorf("expected %d connectivity rule IDs, got %d", len(expectedRules), len(actualRules))
					}

					// Convert to maps for easier comparison since order might differ
					expectedMap := make(map[string]bool)
					for _, rule := range expectedRules {
						expectedMap[rule] = true
					}

					for _, rule := range actualRules {
						if !expectedMap[rule] {
							return fmt.Errorf("unexpected connectivity rule ID: %s", rule)
						}
					}

					return nil
				},
			},
			{
				Config: config(name, []string{"rule1", "rule2"}, 2),
			},
			{
				// Update connectivity rule IDs
				Config: config(name, allRules, 1),
				Check: func(s *terraform.State) error {
					namespaceId := s.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					rule1Id := s.RootModule().Resources["temporalcloud_connectivity_rule.rule1"].Primary.Attributes["id"]
					rule2Id := s.RootModule().Resources["temporalcloud_connectivity_rule.rule2"].Primary.Attributes["id"]
					rule3Id := s.RootModule().Resources["temporalcloud_connectivity_rule.rule3"].Primary.Attributes["id"]
					rule4Id := s.RootModule().Resources["temporalcloud_connectivity_rule.rule4"].Primary.Attributes["id"]

					conn := newConnection(t)
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: namespaceId,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := ns.Namespace.GetSpec()
					expectedRules := []string{rule1Id, rule2Id, rule3Id, rule4Id}
					actualRules := spec.GetConnectivityRuleIds()

					if len(actualRules) != len(expectedRules) {
						return fmt.Errorf("expected %d connectivity rule IDs, got %d", len(expectedRules), len(actualRules))
					}

					// Convert to maps for easier comparison since order might differ
					expectedMap := make(map[string]bool)
					for _, rule := range expectedRules {
						expectedMap[rule] = true
					}

					for _, rule := range actualRules {
						if !expectedMap[rule] {
							return fmt.Errorf("unexpected connectivity rule ID: %s", rule)
						}
					}

					return nil
				},
			},
			{
				// Remove all connectivity rule IDs for namespace
				Config: config(name, []string{}, 1),
				Check: func(s *terraform.State) error {
					namespaceId := s.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					conn := newConnection(t)
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: namespaceId,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}

					spec := ns.Namespace.GetSpec()
					actualRules := spec.GetConnectivityRuleIds()

					if len(actualRules) != 0 {
						return fmt.Errorf("expected 0 connectivity rule IDs, got %d", len(actualRules))
					}

					return nil
				},
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_namespace.test",
			},
		},
	})
}
