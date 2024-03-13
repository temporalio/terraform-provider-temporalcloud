package provider

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	cloudservicev1 "github.com/temporalio/terraform-provider-temporalcloud/proto/go/temporal/api/cloud/cloudservice/v1"
)

const (
	emailDomain   = "temporal.io"
	emailBaseAddr = "saas-cicd-prod"
)

func createRandomEmail() string {
	return fmt.Sprintf("%s+terraformprovider-%s@%s", emailBaseAddr, randomString(), emailDomain)
}

func TestAccBasicUser(t *testing.T) {
	emailAddr := createRandomEmail()
	config := func(email string, role string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_user" "terraform" {
  email = "%s"
  account_access = "%s"
}`, email, role)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(emailAddr, "Read"),
			},
			{
				Config: config(emailAddr, "Developer"),
			},
			{
				Config: config(emailAddr, "Admin"),
			},
		},
	})
}

func TestAccBasicUserWithNamespaceAccesses(t *testing.T) {
	type configArgs struct {
		Email         string
		NamespaceName string
		AccountPerm   string
		NamespacePerm string
	}

	emailAddr := createRandomEmail()

	tmpl := template.Must(template.New("config").Parse(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "test" {
  name               = "{{ .NamespaceName }}"
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

  retention_days = 7
}

resource "temporalcloud_user" "terraform" {
  email = "{{ .Email }}"
  account_access = "{{ .AccountPerm }}"
  namespace_accesses = [
    {
      namespace = temporalcloud_namespace.test.id
      permission = "{{ .NamespacePerm }}"
    }
  ]
  
  depends_on = [temporalcloud_namespace.test]
}`))

	config := func(args configArgs) string {
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		if err := tmpl.Execute(writer, args); err != nil {
			t.Errorf("failed to execute template:  %v", err)
			t.FailNow()
		}

		writer.Flush()
		return buf.String()
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(configArgs{
					Email:         emailAddr,
					NamespaceName: randomString(),
					NamespacePerm: "Write",
					AccountPerm:   "Read",
				}),
				Check: func(state *terraform.State) error {
					id := state.RootModule().Resources["temporalcloud_user.terraform"].Primary.Attributes["id"]
					conn := newConnection(t)
					user, err := conn.GetUser(context.Background(), &cloudservicev1.GetUserRequest{
						UserId: id,
					})
					if err != nil {
						return fmt.Errorf("failed to get user: %v", err)
					}
					nsID := state.RootModule().Resources["temporalcloud_namespace.test"].Primary.Attributes["id"]
					ns, err := conn.GetNamespace(context.Background(), &cloudservicev1.GetNamespaceRequest{
						Namespace: nsID,
					})
					if err != nil {
						return fmt.Errorf("failed to get namespace: %v", err)
					}
					spec := user.User.GetSpec()
					if strings.ToLower(spec.GetAccess().GetAccountAccess().GetRole()) != "read" {
						return errors.New("expected account role to be read")
					}
					nsPerm, ok := spec.GetAccess().GetNamespaceAccesses()[ns.Namespace.GetNamespace()]
					if !ok {
						return fmt.Errorf("expected entry in NamespaceAccesses for namespace %s", ns.Namespace.GetNamespace())
					}
					if strings.ToLower(nsPerm.GetPermission()) != "write" {
						return errors.New("expected namespace access permission to be write")
					}
					return nil
				},
			},
		},
	})
}
