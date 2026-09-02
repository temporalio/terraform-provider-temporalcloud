package enums

import (
	"errors"
	"testing"

	"go.temporal.io/cloud-sdk/api/identity/v1"
)

func TestProjectAccessRoleRoundTrip(t *testing.T) {
	t.Parallel()

	for _, role := range AllowedProjectAccessRoles() {
		t.Run(role, func(t *testing.T) {
			t.Parallel()

			enum, err := ToProjectAccessRole(role)
			if err != nil {
				t.Fatalf("ToProjectAccessRole(%q): %v", role, err)
			}
			if enum == identity.ProjectAccess_PROJECT_ROLE_UNSPECIFIED {
				t.Fatalf("ToProjectAccessRole(%q) produced UNSPECIFIED", role)
			}

			got, err := FromProjectAccessRole(enum)
			if err != nil {
				t.Fatalf("FromProjectAccessRole(%v): %v", enum, err)
			}
			if got != role {
				t.Errorf("round-trip changed %q into %q", role, got)
			}
		})
	}
}

func TestToProjectAccessRoleIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"ADMIN", "Admin", "aDmIn"} {
		got, err := ToProjectAccessRole(input)
		if err != nil {
			t.Fatalf("ToProjectAccessRole(%q): %v", input, err)
		}
		if got != identity.ProjectAccess_PROJECT_ROLE_ADMIN {
			t.Errorf("ToProjectAccessRole(%q) = %v, want ADMIN", input, got)
		}
	}
}

// "developer" is inherited from the account_access developer role and cannot be assigned, so it
// must be rejected on the way in while still decoding on the way out.
func TestProjectAccessRoleDeveloperIsNotAssignable(t *testing.T) {
	t.Parallel()

	for _, role := range AllowedProjectAccessRoles() {
		if role == "developer" {
			t.Fatal("developer must not appear in AllowedProjectAccessRoles")
		}
	}

	if _, err := ToProjectAccessRole("developer"); !errors.Is(err, ErrInvalidProjectAccessRole) {
		t.Errorf("ToProjectAccessRole(\"developer\") error = %v, want ErrInvalidProjectAccessRole", err)
	}

	got, err := FromProjectAccessRole(identity.ProjectAccess_PROJECT_ROLE_DEVELOPER)
	if err != nil {
		t.Fatalf("FromProjectAccessRole(DEVELOPER): %v", err)
	}
	if got != "developer" {
		t.Errorf("FromProjectAccessRole(DEVELOPER) = %q, want \"developer\"", got)
	}
}

func TestProjectAccessRoleRejectsUnknown(t *testing.T) {
	t.Parallel()

	if _, err := ToProjectAccessRole("nonsense"); !errors.Is(err, ErrInvalidProjectAccessRole) {
		t.Errorf("ToProjectAccessRole(\"nonsense\") error = %v, want ErrInvalidProjectAccessRole", err)
	}

	if _, err := FromProjectAccessRole(identity.ProjectAccess_PROJECT_ROLE_UNSPECIFIED); !errors.Is(err, ErrInvalidProjectAccessRole) {
		t.Errorf("FromProjectAccessRole(UNSPECIFIED) error = %v, want ErrInvalidProjectAccessRole", err)
	}
}
