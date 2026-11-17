package provider

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func randomString(length int) string {
	r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	const charset = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}

// testAccCheckResourceNotRecreated records the resource's id on first use and fails if a later
// step finds a different one.
func testAccCheckResourceNotRecreated(address string, id *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[address]
		if !ok {
			return fmt.Errorf("%s not found in state", address)
		}
		if *id == "" {
			*id = rs.Primary.ID
			return nil
		}
		if rs.Primary.ID != *id {
			return fmt.Errorf("%s id changed from %s to %s; it should not have been recreated", address, *id, rs.Primary.ID)
		}
		return nil
	}
}
