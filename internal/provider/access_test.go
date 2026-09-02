package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
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

func TestProjectAccessRevocationWarning(t *testing.T) {
	t.Parallel()

	nullSet := types.SetNull(types.ObjectType{AttrTypes: projectAccessAttrs})
	emptySet := types.SetValueMust(types.ObjectType{AttrTypes: projectAccessAttrs}, []attr.Value{})
	unknownSet := types.SetUnknown(types.ObjectType{AttrTypes: projectAccessAttrs})

	cases := []struct {
		name        string
		state       types.Set
		config      types.Set
		wantWarning bool
	}{
		{name: "no roles in state", state: nullSet, config: nullSet},
		{name: "empty state set", state: emptySet, config: nullSet},
		{name: "roles kept in configuration", state: projectAccessSet("project-a"), config: projectAccessSet("project-a")},
		{name: "roles replaced in configuration", state: projectAccessSet("project-a"), config: projectAccessSet("project-b")},
		{name: "configuration is not known until apply", state: projectAccessSet("project-a"), config: unknownSet},
		// An attribute absent from configuration is the case worth warning about: the roles leave
		// the diff without ever having been mentioned.
		{name: "roles absent from configuration", state: projectAccessSet("project-a", "project-b"), config: nullSet, wantWarning: true},
		{name: "configuration empties the set", state: projectAccessSet("project-a"), config: emptySet, wantWarning: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := projectAccessRevocationWarning(tc.state, tc.config)

			if diags.HasError() {
				t.Fatalf("expected no errors, got %+v", diags)
			}
			if got := diags.WarningsCount(); got != boolToCount(tc.wantWarning) {
				t.Fatalf("expected %d warnings, got %d: %+v", boolToCount(tc.wantWarning), got, diags)
			}
		})
	}
}

func boolToCount(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestProjectAccessRevocationOnCreateWarning(t *testing.T) {
	t.Parallel()

	held := func(projectIDs ...string) map[string]*identityv1.ProjectAccess {
		accesses := make(map[string]*identityv1.ProjectAccess, len(projectIDs))
		for _, projectID := range projectIDs {
			accesses[projectID] = &identityv1.ProjectAccess{Role: identityv1.ProjectAccess_PROJECT_ROLE_READ}
		}
		return accesses
	}
	nullSet := types.SetNull(types.ObjectType{AttrTypes: projectAccessAttrs})

	cases := []struct {
		name        string
		current     map[string]*identityv1.ProjectAccess
		config      types.Set
		wantWarning bool
	}{
		{name: "group holds no roles", current: nil, config: nullSet},
		{name: "group holds no roles and configuration adds some", current: nil, config: projectAccessSet("project-a")},
		{name: "configuration keeps managing roles", current: held("project-a"), config: projectAccessSet("project-a")},
		{name: "configuration replaces the roles", current: held("project-a"), config: projectAccessSet("project-b")},
		// A plan can carry an unknown project_id, which still means configuration manages the roles.
		{name: "configuration is not known until apply", current: held("project-a"), config: types.SetUnknown(types.ObjectType{AttrTypes: projectAccessAttrs})},
		{name: "configuration omits the group's roles", current: held("project-a", "project-b"), config: nullSet, wantWarning: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := projectAccessRevocationOnCreateWarning(tc.current, tc.config)

			if diags.HasError() {
				t.Fatalf("expected no errors, got %+v", diags)
			}
			if got := diags.WarningsCount(); got != boolToCount(tc.wantWarning) {
				t.Fatalf("expected %d warnings, got %d: %+v", boolToCount(tc.wantWarning), got, diags)
			}
		})
	}
}

func projectAccessSet(projectIDs ...string) types.Set {
	objects := make([]attr.Value, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		objects = append(objects, types.ObjectValueMust(projectAccessAttrs, map[string]attr.Value{
			"project_id": types.StringValue(projectID),
			"role":       internaltypes.CaseInsensitiveString("read"),
		}))
	}

	return types.SetValueMust(types.ObjectType{AttrTypes: projectAccessAttrs}, objects)
}
