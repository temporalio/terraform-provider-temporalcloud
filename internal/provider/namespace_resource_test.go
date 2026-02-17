package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"text/template"
	"time"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"go.temporal.io/cloud-sdk/api/namespace/v1"
	operationv1 "go.temporal.io/cloud-sdk/api/operation/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

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

func TestAccNamespaceNameValidation(t *testing.T) {
	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {
}

resource "temporalcloud_namespace" "test" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  api_key_auth       = true
  retention_days     = 7
}`, name)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Name too short (1 char)
				Config:      config("a"),
				ExpectError: regexp.MustCompile(`string length must be between 2 and 64`),
			},
			{
				// Name too long (65 chars)
				Config:      config("a" + strings.Repeat("b", 64)),
				ExpectError: regexp.MustCompile(`string length must be between 2 and 64`),
			},
			{
				// Name starts with number
				Config:      config("1invalid"),
				ExpectError: regexp.MustCompile(`must start with a lowercase letter`),
			},
			{
				// Name starts with hyphen
				Config:      config("-invalid"),
				ExpectError: regexp.MustCompile(`must start with a lowercase letter`),
			},
			{
				// Name ends with hyphen
				Config:      config("invalid-"),
				ExpectError: regexp.MustCompile(`(?s)must start with a lowercase letter.*end with a letter or number`),
			},
			{
				// Name contains uppercase
				Config:      config("Invalid"),
				ExpectError: regexp.MustCompile(`must start with a lowercase letter`),
			},
			{
				// Name contains underscore
				Config:      config("invalid_name"),
				ExpectError: regexp.MustCompile(`must start with a lowercase letter`),
			},
		},
	})
}

func TestAccBasicNamespace(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-basic-namespace", randomString(10))
	config := func(name string, retention int, deleteProtection bool) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIByDCCAU2gAwIBAgIRAuOeFDeADUx5O53PRIsIPZIwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDAwMzNaFw0yNjA4MjAxNDAxMzNa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATRWwv2
nVfToOR59QuRHk5jAVhu991AQWXwLFSzHzjmZ8XIkiVzh3EhPwybsnm+uV6XN/xe
1+KJ/0NyiVL91KFwS0y5xLKqdvy/mOv0eSUy/blJpLR66diTqPDMlYntuBmjZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTNvOjx
e/IC/jxLZvXGQT4fmj0eMTAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjJ5cU4wCgYIKoZIzj0EAwMDaQAwZgIxALwxPDblJQ9R65G9/M7Tyx1H/7EUTeo9
ThGIAJ5f8VReP9T7155ri5sRCUTBdgFHVAIxAOrtnTo8uRjEs8HdUW0e9H7E2nyW
5hWHcfGvGFFkZn3TkJIX3kdJslSDmxOXhn7D/w==
-----END CERTIFICATE-----
PEM
)

  retention_days     = %d
  namespace_lifecycle = {
	  enable_delete_protection = %t
  }
}`, name, retention, deleteProtection)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// New namespace with retention of 7
				Config: config(name, 7, true),
			},
			{
				Config: config(name, 14, true),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_namespace.terraform",
			},
			{
				Config: config(name, 14, false), // disable delete protection for deletion to succeed
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
MIIByDCCAU2gAwIBAgIRAuOeFDeADUx5O53PRIsIPZIwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDAwMzNaFw0yNjA4MjAxNDAxMzNa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATRWwv2
nVfToOR59QuRHk5jAVhu991AQWXwLFSzHzjmZ8XIkiVzh3EhPwybsnm+uV6XN/xe
1+KJ/0NyiVL91KFwS0y5xLKqdvy/mOv0eSUy/blJpLR66diTqPDMlYntuBmjZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTNvOjx
e/IC/jxLZvXGQT4fmj0eMTAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjJ5cU4wCgYIKoZIzj0EAwMDaQAwZgIxALwxPDblJQ9R65G9/M7Tyx1H/7EUTeo9
ThGIAJ5f8VReP9T7155ri5sRCUTBdgFHVAIxAOrtnTo8uRjEs8HdUW0e9H7E2nyW
5hWHcfGvGFFkZn3TkJIX3kdJslSDmxOXhn7D/w==
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
MIIByDCCAU2gAwIBAgIRAuOeFDeADUx5O53PRIsIPZIwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDAwMzNaFw0yNjA4MjAxNDAxMzNa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATRWwv2
nVfToOR59QuRHk5jAVhu991AQWXwLFSzHzjmZ8XIkiVzh3EhPwybsnm+uV6XN/xe
1+KJ/0NyiVL91KFwS0y5xLKqdvy/mOv0eSUy/blJpLR66diTqPDMlYntuBmjZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTNvOjx
e/IC/jxLZvXGQT4fmj0eMTAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjJ5cU4wCgYIKoZIzj0EAwMDaQAwZgIxALwxPDblJQ9R65G9/M7Tyx1H/7EUTeo9
ThGIAJ5f8VReP9T7155ri5sRCUTBdgFHVAIxAOrtnTo8uRjEs8HdUW0e9H7E2nyW
5hWHcfGvGFFkZn3TkJIX3kdJslSDmxOXhn7D/w==
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
MIIByDCCAU2gAwIBAgIRAuOeFDeADUx5O53PRIsIPZIwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDAwMzNaFw0yNjA4MjAxNDAxMzNa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATRWwv2
nVfToOR59QuRHk5jAVhu991AQWXwLFSzHzjmZ8XIkiVzh3EhPwybsnm+uV6XN/xe
1+KJ/0NyiVL91KFwS0y5xLKqdvy/mOv0eSUy/blJpLR66diTqPDMlYntuBmjZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTNvOjx
e/IC/jxLZvXGQT4fmj0eMTAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjJ5cU4wCgYIKoZIzj0EAwMDaQAwZgIxALwxPDblJQ9R65G9/M7Tyx1H/7EUTeo9
ThGIAJ5f8VReP9T7155ri5sRCUTBdgFHVAIxAOrtnTo8uRjEs8HdUW0e9H7E2nyW
5hWHcfGvGFFkZn3TkJIX3kdJslSDmxOXhn7D/w==
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
MIIByDCCAU2gAwIBAgIRAuOeFDeADUx5O53PRIsIPZIwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDAwMzNaFw0yNjA4MjAxNDAxMzNa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATRWwv2
nVfToOR59QuRHk5jAVhu991AQWXwLFSzHzjmZ8XIkiVzh3EhPwybsnm+uV6XN/xe
1+KJ/0NyiVL91KFwS0y5xLKqdvy/mOv0eSUy/blJpLR66diTqPDMlYntuBmjZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTNvOjx
e/IC/jxLZvXGQT4fmj0eMTAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjJ5cU4wCgYIKoZIzj0EAwMDaQAwZgIxALwxPDblJQ9R65G9/M7Tyx1H/7EUTeo9
ThGIAJ5f8VReP9T7155ri5sRCUTBdgFHVAIxAOrtnTo8uRjEs8HdUW0e9H7E2nyW
5hWHcfGvGFFkZn3TkJIX3kdJslSDmxOXhn7D/w==
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
MIIByDCCAU2gAwIBAgIRAuOeFDeADUx5O53PRIsIPZIwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDAwMzNaFw0yNjA4MjAxNDAxMzNa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATRWwv2
nVfToOR59QuRHk5jAVhu991AQWXwLFSzHzjmZ8XIkiVzh3EhPwybsnm+uV6XN/xe
1+KJ/0NyiVL91KFwS0y5xLKqdvy/mOv0eSUy/blJpLR66diTqPDMlYntuBmjZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTNvOjx
e/IC/jxLZvXGQT4fmj0eMTAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjJ5cU4wCgYIKoZIzj0EAwMDaQAwZgIxALwxPDblJQ9R65G9/M7Tyx1H/7EUTeo9
ThGIAJ5f8VReP9T7155ri5sRCUTBdgFHVAIxAOrtnTo8uRjEs8HdUW0e9H7E2nyW
5hWHcfGvGFFkZn3TkJIX3kdJslSDmxOXhn7D/w==
-----END CERTIFICATE-----


-----BEGIN CERTIFICATE-----
MIIBxzCCAU2gAwIBAgIRAnkbVL6hHp218oB9UlQtN7wwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDQxMDBaFw0yNjA4MjAxNDQyMDBa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAQ0zwd8
FIaSYahXebDHEd3EywawWF087ZKz9Tbg6Qp+JQ8YhjJI4/QWrem/6cGnBchmnstC
WrR1g7D5EKU7HGh4xaJm06vnl1xzOfL7kcehulgQKTqCi7dquu8WkPRoh22jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRasbRA
UFbkQyc1p9G45bDXcwQrSzAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LlV2MWcwCgYIKoZIzj0EAwMDaAAwZQIwFv/1HkzCYi3rGcWWW5NNnEoBvdASedAQ
21CJqdeZp58YDadO7bWX3ov62kg3ocLIAjEAqMFQCFGow9iPDzgCpv2U1kpvWlMp
fhMROGKW4FAw96+jvpcXwbHYgOHN0pf1Bde1
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
MIIBxzCCAU2gAwIBAgIRAnkbVL6hHp218oB9UlQtN7wwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDQxMDBaFw0yNjA4MjAxNDQyMDBa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAQ0zwd8
FIaSYahXebDHEd3EywawWF087ZKz9Tbg6Qp+JQ8YhjJI4/QWrem/6cGnBchmnstC
WrR1g7D5EKU7HGh4xaJm06vnl1xzOfL7kcehulgQKTqCi7dquu8WkPRoh22jZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRasbRA
UFbkQyc1p9G45bDXcwQrSzAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LlV2MWcwCgYIKoZIzj0EAwMDaAAwZQIwFv/1HkzCYi3rGcWWW5NNnEoBvdASedAQ
21CJqdeZp58YDadO7bWX3ov62kg3ocLIAjEAqMFQCFGow9iPDzgCpv2U1kpvWlMp
fhMROGKW4FAw96+jvpcXwbHYgOHN0pf1Bde1
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

func TestAccNamespaceWithCapacity(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-capacity", randomString(10))
	config := func(name string, variable string) string {
		return fmt.Sprintf(`
variable "provisioned" {
  type = object({
	mode = string
    value = number
  })
  default = {
	mode = "provisioned"
	value = 2
  }
}

variable "on_demand" {
  type = object({
	mode = string
	value = number
  })
  default = {
	mode = "on_demand"
	value = 0
  }
}

provider "temporalcloud" {

}

resource "temporalcloud_namespace" "capacitytest" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  accepted_client_ca = base64encode(<<PEM
-----BEGIN CERTIFICATE-----
MIIByDCCAU2gAwIBAgIRAuOeFDeADUx5O53PRIsIPZIwCgYIKoZIzj0EAwMwEjEQ
MA4GA1UEChMHdGVzdGluZzAeFw0yNTA4MjAxNDAwMzNaFw0yNjA4MjAxNDAxMzNa
MBIxEDAOBgNVBAoTB3Rlc3RpbmcwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAATRWwv2
nVfToOR59QuRHk5jAVhu991AQWXwLFSzHzjmZ8XIkiVzh3EhPwybsnm+uV6XN/xe
1+KJ/0NyiVL91KFwS0y5xLKqdvy/mOv0eSUy/blJpLR66diTqPDMlYntuBmjZzBl
MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTNvOjx
e/IC/jxLZvXGQT4fmj0eMTAjBgNVHREEHDAaghhjbGllbnQucm9vdC50ZXN0aW5n
LjJ5cU4wCgYIKoZIzj0EAwMDaQAwZgIxALwxPDblJQ9R65G9/M7Tyx1H/7EUTeo9
ThGIAJ5f8VReP9T7155ri5sRCUTBdgFHVAIxAOrtnTo8uRjEs8HdUW0e9H7E2nyW
5hWHcfGvGFFkZn3TkJIX3kdJslSDmxOXhn7D/w==
-----END CERTIFICATE-----
PEM
)
  retention_days = 7
  capacity = %s
}`, name, variable)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// New namespace with on demand capacity
				Config: config(name, "null"),
			},
			{
				Config: config(name, "var.provisioned"),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_namespace.capacitytest"].Primary.Attributes["id"]
					conn := newConnection(t)
					for i := 0; i < 60; i++ {
						ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
							Namespace: id,
						})
						time.Sleep(1 * time.Second)
						if err != nil {
							return fmt.Errorf("failed to get namespace: %v", err)
						}
						if ns.GetNamespace().GetCapacity().GetLatestRequest().GetState() == namespace.Capacity_Request_STATE_CAPACITY_REQUEST_IN_PROGRESS {
							continue
						}
						if ns.GetNamespace().GetCapacity().GetLatestRequest().GetState() == namespace.Capacity_Request_STATE_CAPACITY_REQUEST_FAILED {
							return fmt.Errorf("capacity request failed: %v", ns.GetNamespace().GetCapacity().GetLatestRequest())
						}
						if ns.GetNamespace().GetCapacity().GetLatestRequest().GetState() == namespace.Capacity_Request_STATE_CAPACITY_REQUEST_COMPLETED {
							if ns.GetNamespace().GetCapacity().GetProvisioned() == nil {
								return fmt.Errorf("expected provisioned capacity, got nil")
							} else {
								value := ns.GetNamespace().GetCapacity().GetProvisioned().GetCurrentValue()
								if value != 2.0 {
									return fmt.Errorf("expected provisioned capacity of 2, got %f", value)
								}
								// success
								return nil
							}
						}
					}
					return fmt.Errorf("timed out waiting for capacity change")
				},
			},
			{
				Config: config(name, "var.on_demand"),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_namespace.capacitytest"].Primary.Attributes["id"]
					conn := newConnection(t)
					for i := 0; i < 60; i++ {
						ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
							Namespace: id,
						})
						time.Sleep(1 * time.Second)
						if err != nil {
							return fmt.Errorf("failed to get namespace: %v", err)
						}
						if ns.GetNamespace().GetCapacity().GetLatestRequest().GetState() == namespace.Capacity_Request_STATE_CAPACITY_REQUEST_IN_PROGRESS {
							continue
						}
						if ns.GetNamespace().GetCapacity().GetLatestRequest().GetState() == namespace.Capacity_Request_STATE_CAPACITY_REQUEST_FAILED {
							return fmt.Errorf("capacity request failed: %v", ns.GetNamespace().GetCapacity().GetLatestRequest())
						}
						if ns.GetNamespace().GetCapacity().GetLatestRequest().GetState() == namespace.Capacity_Request_STATE_CAPACITY_REQUEST_COMPLETED {
							if ns.GetNamespace().GetCapacity().GetOnDemand() == nil {
								return fmt.Errorf("expected on_demand capacity, got nil")
							} else {
								// success
								return nil
							}
						}
					}
					return fmt.Errorf("timed out waiting for capacity change")
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestClassifyRegionChanges(t *testing.T) {
	tests := []struct {
		name            string
		current         []string
		planned         []string
		expectedAdded   []string
		expectedRemoval bool
	}{
		{
			name:            "no change same order",
			current:         []string{"aws-us-east-1", "aws-us-west-2"},
			planned:         []string{"aws-us-east-1", "aws-us-west-2"},
			expectedAdded:   nil,
			expectedRemoval: false,
		},
		{
			name:            "no change different order",
			current:         []string{"aws-us-east-1", "aws-us-west-2"},
			planned:         []string{"aws-us-west-2", "aws-us-east-1"},
			expectedAdded:   nil,
			expectedRemoval: false,
		},
		{
			name:            "region addition",
			current:         []string{"aws-us-east-1"},
			planned:         []string{"aws-us-east-1", "aws-us-west-2"},
			expectedAdded:   []string{"aws-us-west-2"},
			expectedRemoval: false,
		},
		{
			name:            "region addition with prepended region",
			current:         []string{"aws-us-east-1"},
			planned:         []string{"aws-us-west-2", "aws-us-east-1"},
			expectedAdded:   []string{"aws-us-west-2"},
			expectedRemoval: false,
		},
		{
			name:            "region replacement is treated as removal",
			current:         []string{"aws-us-east-1"},
			planned:         []string{"aws-us-west-2"},
			expectedAdded:   nil,
			expectedRemoval: true,
		},
		{
			name:            "region removal is blocked",
			current:         []string{"aws-us-east-1", "aws-us-west-2"},
			planned:         []string{"aws-us-east-1"},
			expectedAdded:   nil,
			expectedRemoval: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			added, hasRemoval := classifyRegionChanges(tc.current, tc.planned)

			if !slices.Equal(added, tc.expectedAdded) {
				t.Errorf("added = %v, want %v", added, tc.expectedAdded)
			}
			if hasRemoval != tc.expectedRemoval {
				t.Errorf("hasRemoval = %v, want %v", hasRemoval, tc.expectedRemoval)
			}
		})
	}
}

type fakeNamespaceRegionUpdateClient struct {
	updateRequests       []*cloudservicev1.UpdateNamespaceRequest
	getNamespaceRequests []*cloudservicev1.GetNamespaceRequest
	addRequests          []*cloudservicev1.AddNamespaceRegionRequest
	getResourceVersion   string
	updateErr            error
}

func (f *fakeNamespaceRegionUpdateClient) GetNamespace(_ context.Context, req *cloudservicev1.GetNamespaceRequest, _ ...grpc.CallOption) (*cloudservicev1.GetNamespaceResponse, error) {
	f.getNamespaceRequests = append(f.getNamespaceRequests, &cloudservicev1.GetNamespaceRequest{
		Namespace: req.GetNamespace(),
	})

	resourceVersion := f.getResourceVersion
	if resourceVersion == "" {
		resourceVersion = "resource-version-default"
	}

	return &cloudservicev1.GetNamespaceResponse{
		Namespace: &namespace.Namespace{
			ResourceVersion: resourceVersion,
		},
	}, nil
}

func (f *fakeNamespaceRegionUpdateClient) UpdateNamespace(_ context.Context, req *cloudservicev1.UpdateNamespaceRequest, _ ...grpc.CallOption) (*cloudservicev1.UpdateNamespaceResponse, error) {
	f.updateRequests = append(f.updateRequests, &cloudservicev1.UpdateNamespaceRequest{
		Namespace:       req.GetNamespace(),
		ResourceVersion: req.GetResourceVersion(),
		Spec: &namespace.NamespaceSpec{
			Regions: slices.Clone(req.GetSpec().GetRegions()),
		},
	})

	if f.updateErr != nil {
		return nil, f.updateErr
	}

	return &cloudservicev1.UpdateNamespaceResponse{
		AsyncOperation: &operationv1.AsyncOperation{
			Id: "op-update",
		},
	}, nil
}

func (f *fakeNamespaceRegionUpdateClient) AddNamespaceRegion(_ context.Context, req *cloudservicev1.AddNamespaceRegionRequest, _ ...grpc.CallOption) (*cloudservicev1.AddNamespaceRegionResponse, error) {
	f.addRequests = append(f.addRequests, &cloudservicev1.AddNamespaceRegionRequest{
		Namespace:       req.GetNamespace(),
		Region:          req.GetRegion(),
		ResourceVersion: req.GetResourceVersion(),
	})

	return &cloudservicev1.AddNamespaceRegionResponse{
		AsyncOperation: &operationv1.AsyncOperation{
			Id: "op-add",
		},
	}, nil
}

func TestUpdateNamespaceWithRegions_RegionAdditionSequence(t *testing.T) {
	fakeClient := &fakeNamespaceRegionUpdateClient{
		getResourceVersion: "resource-version-after-update",
	}

	var awaitedOps []string
	awaitFn := func(_ context.Context, op *operationv1.AsyncOperation) error {
		if op == nil {
			return fmt.Errorf("unexpected nil async operation")
		}
		awaitedOps = append(awaitedOps, op.GetId())
		return nil
	}

	spec := &namespace.NamespaceSpec{
		Regions: []string{"aws-us-west-2", "aws-us-east-1"},
	}

	err := updateNamespaceWithRegions(
		context.Background(),
		fakeClient,
		"namespace-id",
		[]string{"aws-us-east-1"},
		"resource-version-initial",
		spec,
		awaitFn,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(fakeClient.updateRequests) != 1 {
		t.Fatalf("expected 1 UpdateNamespace call, got %d", len(fakeClient.updateRequests))
	}

	updateReq := fakeClient.updateRequests[0]
	if updateReq.GetNamespace() != "namespace-id" {
		t.Fatalf("expected update namespace %q, got %q", "namespace-id", updateReq.GetNamespace())
	}
	if updateReq.GetResourceVersion() != "resource-version-initial" {
		t.Fatalf("expected update resource version %q, got %q", "resource-version-initial", updateReq.GetResourceVersion())
	}
	if !slices.Equal(updateReq.GetSpec().GetRegions(), []string{"aws-us-east-1"}) {
		t.Fatalf("expected update regions to preserve current order, got %v", updateReq.GetSpec().GetRegions())
	}

	if len(fakeClient.getNamespaceRequests) != 1 {
		t.Fatalf("expected 1 GetNamespace call before region add, got %d", len(fakeClient.getNamespaceRequests))
	}
	if len(fakeClient.addRequests) != 1 {
		t.Fatalf("expected 1 AddNamespaceRegion call, got %d", len(fakeClient.addRequests))
	}

	addReq := fakeClient.addRequests[0]
	if addReq.GetRegion() != "aws-us-west-2" {
		t.Fatalf("expected added region %q, got %q", "aws-us-west-2", addReq.GetRegion())
	}
	if addReq.GetResourceVersion() != "resource-version-after-update" {
		t.Fatalf("expected add resource version %q, got %q", "resource-version-after-update", addReq.GetResourceVersion())
	}

	if !slices.Equal(awaitedOps, []string{"op-update", "op-add"}) {
		t.Fatalf("expected awaited ops [op-update op-add], got %v", awaitedOps)
	}
}

func TestUpdateNamespaceWithRegions_RemovalBlockedBeforeApiCalls(t *testing.T) {
	fakeClient := &fakeNamespaceRegionUpdateClient{}

	spec := &namespace.NamespaceSpec{
		Regions: []string{"aws-us-east-1"},
	}

	err := updateNamespaceWithRegions(
		context.Background(),
		fakeClient,
		"namespace-id",
		[]string{"aws-us-east-1", "aws-us-west-2"},
		"resource-version-initial",
		spec,
		func(_ context.Context, _ *operationv1.AsyncOperation) error { return nil },
	)
	if !errors.Is(err, errNamespaceRegionRemovalNotSupported) {
		t.Fatalf("expected removal not supported error, got %v", err)
	}

	if len(fakeClient.updateRequests) != 0 {
		t.Fatalf("expected 0 UpdateNamespace calls, got %d", len(fakeClient.updateRequests))
	}
	if len(fakeClient.getNamespaceRequests) != 0 {
		t.Fatalf("expected 0 GetNamespace calls, got %d", len(fakeClient.getNamespaceRequests))
	}
	if len(fakeClient.addRequests) != 0 {
		t.Fatalf("expected 0 AddNamespaceRegion calls, got %d", len(fakeClient.addRequests))
	}
}

func TestUpdateNamespaceWithRegions_NothingToChangeSkipsToAddRegion(t *testing.T) {
	// When the only change is a region addition, UpdateNamespace returns
	// "nothing to change" because the spec (with preserved current regions)
	// is identical to the server state. The function should treat this as a
	// no-op and proceed to AddNamespaceRegion.
	fakeClient := &fakeNamespaceRegionUpdateClient{
		getResourceVersion: "resource-version-current",
		updateErr:          grpcstatus.Error(codes.InvalidArgument, "nothing to change"),
	}

	var awaitedOps []string
	awaitFn := func(_ context.Context, op *operationv1.AsyncOperation) error {
		if op == nil {
			return fmt.Errorf("unexpected nil async operation")
		}
		awaitedOps = append(awaitedOps, op.GetId())
		return nil
	}

	spec := &namespace.NamespaceSpec{
		Regions: []string{"aws-us-east-1", "aws-us-west-2"},
	}

	err := updateNamespaceWithRegions(
		context.Background(),
		fakeClient,
		"namespace-id",
		[]string{"aws-us-east-1"},
		"resource-version-initial",
		spec,
		awaitFn,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// UpdateNamespace was called but returned "nothing to change"
	if len(fakeClient.updateRequests) != 1 {
		t.Fatalf("expected 1 UpdateNamespace call, got %d", len(fakeClient.updateRequests))
	}

	// Should still proceed to add the region
	if len(fakeClient.addRequests) != 1 {
		t.Fatalf("expected 1 AddNamespaceRegion call, got %d", len(fakeClient.addRequests))
	}

	addReq := fakeClient.addRequests[0]
	if addReq.GetRegion() != "aws-us-west-2" {
		t.Fatalf("expected added region %q, got %q", "aws-us-west-2", addReq.GetRegion())
	}

	// Only the add operation should have been awaited (update was a no-op)
	if !slices.Equal(awaitedOps, []string{"op-add"}) {
		t.Fatalf("expected awaited ops [op-add], got %v", awaitedOps)
	}
}
