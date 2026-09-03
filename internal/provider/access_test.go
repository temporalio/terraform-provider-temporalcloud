package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
)

func TestGetCustomRolesRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	input := []string{"role-alpha", "role-beta"}

	accountAccessCustomRolesSet, diags := getCustomRolesSet(ctx, input)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating custom role set: %+v", diags)
	}

	got, diags := getCustomRolesFromSet(ctx, accountAccessCustomRolesSet)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics reading custom role set: %+v", diags)
	}

	if len(got) != len(input) {
		t.Fatalf("expected %d custom roles, got %d", len(input), len(got))
	}

	gotSet := make(map[string]struct{}, len(got))
	for _, role := range got {
		gotSet[role] = struct{}{}
	}

	for _, expected := range input {
		if _, ok := gotSet[expected]; !ok {
			t.Fatalf("expected custom role %q to round-trip", expected)
		}
	}
}

func TestGetCustomRolesSetEmpty(t *testing.T) {
	t.Parallel()

	accountAccessCustomRolesSet, diags := getCustomRolesSet(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	if !accountAccessCustomRolesSet.IsNull() {
		t.Fatalf("expected empty custom roles to produce a null set, got %#v", accountAccessCustomRolesSet)
	}
}

func TestGetAccountAccessFromModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	accountAccessCustomRolesSet, diags := types.SetValueFrom(ctx, types.StringType, []string{"role-alpha"})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating input set: %+v", diags)
	}

	testCases := []struct {
		name                     string
		accountAccess            string
		accountAccessCustomRoles types.Set
		wantNil                  bool
		wantRole                 identityv1.AccountAccess_Role
		wantCustomRoleCount      int
	}{
		{
			name:                     "built-in role only",
			accountAccess:            "read",
			accountAccessCustomRoles: types.SetNull(types.StringType),
			wantRole:                 identityv1.AccountAccess_ROLE_READ,
			wantCustomRoleCount:      0,
		},
		{
			name:                     "none with custom roles remains present",
			accountAccess:            "none",
			accountAccessCustomRoles: accountAccessCustomRolesSet,
			wantRole:                 identityv1.AccountAccess_ROLE_UNSPECIFIED,
			wantCustomRoleCount:      1,
		},
		{
			name:                     "none without custom roles becomes nil",
			accountAccess:            "none",
			accountAccessCustomRoles: types.SetNull(types.StringType),
			wantNil:                  true,
			wantRole:                 identityv1.AccountAccess_ROLE_UNSPECIFIED,
			wantCustomRoleCount:      0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			accountAccess, diags := getAccountAccessFromModel(ctx, tc.accountAccess, tc.accountAccessCustomRoles)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %+v", diags)
			}

			if tc.wantNil {
				if accountAccess != nil {
					t.Fatalf("expected nil account access, got %#v", accountAccess)
				}
				return
			}

			if accountAccess == nil {
				t.Fatal("expected non-nil account access")
			}

			if accountAccess.GetRole() != tc.wantRole {
				t.Fatalf("expected role %v, got %v", tc.wantRole, accountAccess.GetRole())
			}

			if len(accountAccess.GetCustomRoles()) != tc.wantCustomRoleCount {
				t.Fatalf("expected %d custom roles, got %d", tc.wantCustomRoleCount, len(accountAccess.GetCustomRoles()))
			}
		})
	}
}

func TestPreserveProjectAccesses(t *testing.T) {
	t.Parallel()

	grants := map[string]*identityv1.ProjectAccess{
		"project-abc": {Role: identityv1.ProjectAccess_PROJECT_ROLE_ADMIN},
	}

	t.Run("carries grants onto a freshly built access", func(t *testing.T) {
		t.Parallel()

		built := &identityv1.Access{AccountAccess: &identityv1.AccountAccess{Role: identityv1.AccountAccess_ROLE_READ}}
		current := &identityv1.Access{ProjectAccesses: grants}

		preserveProjectAccesses(built, current)

		if len(built.GetProjectAccesses()) != 1 {
			t.Fatalf("expected 1 preserved grant, got %d", len(built.GetProjectAccesses()))
		}
		if got := built.GetProjectAccesses()["project-abc"].GetRole(); got != identityv1.ProjectAccess_PROJECT_ROLE_ADMIN {
			t.Errorf("expected admin role, got %v", got)
		}
		// Preserving must not disturb the access the caller built from configuration.
		if built.GetAccountAccess().GetRole() != identityv1.AccountAccess_ROLE_READ {
			t.Error("account access was modified")
		}
	})

	t.Run("no grants server-side leaves the built access alone", func(t *testing.T) {
		t.Parallel()

		built := &identityv1.Access{}
		preserveProjectAccesses(built, &identityv1.Access{})

		if built.GetProjectAccesses() != nil {
			t.Errorf("expected nil, got %v", built.GetProjectAccesses())
		}
	})

	t.Run("nil current access is tolerated", func(t *testing.T) {
		t.Parallel()

		built := &identityv1.Access{}
		preserveProjectAccesses(built, nil)

		if built.GetProjectAccesses() != nil {
			t.Errorf("expected nil, got %v", built.GetProjectAccesses())
		}
	})

	// A namespace-scoped service account has no Access message at all.
	t.Run("nil built access does not panic", func(t *testing.T) {
		t.Parallel()

		preserveProjectAccesses(nil, &identityv1.Access{ProjectAccesses: grants})
	})
}
