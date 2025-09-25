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
  capacity = %s
}`, name, retention, deleteProtection, variable)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// New namespace with on demand capacity
				Config: config(name, "null"),
			},
			// cannot do provisioned capacity because the test environment doesn't have enough capacity
			// {
			// 	Config: config(name, "var.on_demand"),
			// },
			// {
			// 	Config: config(name, "var.provisioned"),
			// },
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "temporalcloud_namespace.capacitytest",
			},
			// {
			// 	Config: config(name, "var.on_demand"),
			// },
			// Delete testing automatically occurs in TestCase
		},
	})

}
