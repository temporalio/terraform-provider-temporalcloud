// The MIT License
//
// Copyright (c) 2024 Temporal Technologies Inc.  All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserNamespaceAccessBasic(t *testing.T) {
	name := fmt.Sprintf("%s-%s", "tf-user-access", randomString())
	config := func(name string) string {
		return fmt.Sprintf(`
provider "temporalcloud" {

}

resource "temporalcloud_namespace" "terraform" {
  name               = "%s"
  regions            = ["aws-us-east-1"]
  retention_days     = 7
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
}

resource "temporalcloud_user" "admin_user" {
  email              = "notauser@example.example"
  account_access     = "developer"
}

resource "temporalcloud_user" "write_user" {
  email              = "notauser.write@example.example"
  account_access     = "developer"
}

resource "temporalcloud_user" "read_user" {
  email              = "notauser.write@example.example"
  account_access     = "developer"
}

resource "temporalcloud_user_namespace_access" "admin_user_access" {
  namespace_id = temporalcloud_namespace.terraform.id
  user_id      = temporalcloud_user.admin_user.id
  permission   = "admin"
}

resource "temporalcloud_user_namespace_access" "write_user_access" {
  namespace_id = temporalcloud_namespace.terraform.id
  user_id      = temporalcloud_user.write_user.id
  permission   = "write"
}

resource "temporalcloud_user_namespace_access" "read_user_access" {
  namespace_id = temporalcloud_namespace.terraform.id
  user_id      = temporalcloud_user.read_user.id
  permission   = "read"
}`, name)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(name),
			},
		},
	})
}
