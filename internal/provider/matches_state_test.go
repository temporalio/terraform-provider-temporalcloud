package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	identityv1 "go.temporal.io/cloud-sdk/api/identity/v1"
	resourcev1 "go.temporal.io/cloud-sdk/api/resource/v1"

	internaltypes "github.com/temporalio/terraform-provider-temporalcloud/internal/types"
)

// buildUserNamespaceAccessSet creates a types.Set for namespace accesses from a map of namespace_id -> permission.
func buildUserNamespaceAccessSet(t *testing.T, ctx context.Context, accesses map[string]string) types.Set {
	t.Helper()
	if len(accesses) == 0 {
		return types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs})
	}
	objects := make([]attr.Value, 0, len(accesses))
	for ns, perm := range accesses {
		obj, diags := types.ObjectValueFrom(ctx, userNamespaceAccessAttrs, userNamespaceAccessModel{
			NamespaceID: types.StringValue(ns),
			Permission:  internaltypes.CaseInsensitiveString(perm),
		})
		if diags.HasError() {
			t.Fatalf("failed to build namespace access object: %v", diags)
		}
		objects = append(objects, obj)
	}
	set, diags := types.SetValue(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}, objects)
	if diags.HasError() {
		t.Fatalf("failed to build namespace access set: %v", diags)
	}
	return set
}

func buildServiceAccountNamespaceAccessSet(t *testing.T, ctx context.Context, accesses map[string]string) types.Set {
	t.Helper()
	if len(accesses) == 0 {
		return types.SetNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs})
	}
	objects := make([]attr.Value, 0, len(accesses))
	for ns, perm := range accesses {
		obj, diags := types.ObjectValueFrom(ctx, serviceAccountNamespaceAccessAttrs, serviceAccountNamespaceAccessModel{
			NamespaceID: types.StringValue(ns),
			Permission:  internaltypes.CaseInsensitiveString(perm),
		})
		if diags.HasError() {
			t.Fatalf("failed to build namespace access object: %v", diags)
		}
		objects = append(objects, obj)
	}
	set, diags := types.SetValue(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}, objects)
	if diags.HasError() {
		t.Fatalf("failed to build namespace access set: %v", diags)
	}
	return set
}

func buildGroupNamespaceAccessSet(t *testing.T, ctx context.Context, accesses map[string]string) types.Set {
	t.Helper()
	if len(accesses) == 0 {
		return types.SetNull(types.ObjectType{AttrTypes: namespaceAccessAttrs})
	}
	objects := make([]attr.Value, 0, len(accesses))
	for ns, perm := range accesses {
		obj, diags := types.ObjectValueFrom(ctx, namespaceAccessAttrs, namespaceAccessModel{
			NamespaceID: types.StringValue(ns),
			Permission:  internaltypes.CaseInsensitiveString(perm),
		})
		if diags.HasError() {
			t.Fatalf("failed to build namespace access object: %v", diags)
		}
		objects = append(objects, obj)
	}
	set, diags := types.SetValue(types.ObjectType{AttrTypes: namespaceAccessAttrs}, objects)
	if diags.HasError() {
		t.Fatalf("failed to build namespace access set: %v", diags)
	}
	return set
}

func makeUser(id, email string, state resourcev1.ResourceState, role identityv1.AccountAccess_Role, nsAccesses map[string]identityv1.NamespaceAccess_Permission) *identityv1.User {
	accesses := make(map[string]*identityv1.NamespaceAccess, len(nsAccesses))
	for ns, perm := range nsAccesses {
		accesses[ns] = &identityv1.NamespaceAccess{Permission: perm}
	}
	return &identityv1.User{
		Id:    id,
		State: state,
		Spec: &identityv1.UserSpec{
			Email: email,
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: role,
				},
				NamespaceAccesses: accesses,
			},
		},
	}
}

func TestUserMatchesState_Identical(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:            types.StringValue("user-123"),
		State:         types.StringValue("active"),
		Email:         types.StringValue("test@example.com"),
		AccountAccess: internaltypes.CaseInsensitiveString("developer"),
		NamespaceAccesses: buildUserNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "write",
			"ns-2": "read",
		}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_DEVELOPER,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-1": identityv1.NamespaceAccess_PERMISSION_WRITE,
			"ns-2": identityv1.NamespaceAccess_PERMISSION_READ,
		})

	if !userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return true for identical state")
	}
}

func TestUserMatchesState_CaseInsensitiveRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("test@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("Developer"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_DEVELOPER,
		nil)

	if !userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return true for case-insensitive role match")
	}
}

func TestUserMatchesState_CaseInsensitivePermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:            types.StringValue("user-123"),
		State:         types.StringValue("active"),
		Email:         types.StringValue("test@example.com"),
		AccountAccess: internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: buildUserNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "Write",
		}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-1": identityv1.NamespaceAccess_PERMISSION_WRITE,
		})

	if !userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return true for case-insensitive permission match")
	}
}

func TestUserMatchesState_DifferentID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("test@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}),
	}
	user := makeUser("user-456", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		nil)

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for different ID")
	}
}

func TestUserMatchesState_DifferentState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("test@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_SUSPENDED,
		identityv1.AccountAccess_ROLE_READ,
		nil)

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for different state")
	}
}

func TestUserMatchesState_DifferentEmail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("old@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}),
	}
	user := makeUser("user-123", "new@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		nil)

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for different email")
	}
}

func TestUserMatchesState_DifferentRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("test@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_ADMIN,
		nil)

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for different role")
	}
}

func TestUserMatchesState_DifferentNamespaceCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:            types.StringValue("user-123"),
		State:         types.StringValue("active"),
		Email:         types.StringValue("test@example.com"),
		AccountAccess: internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: buildUserNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "write",
		}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-1": identityv1.NamespaceAccess_PERMISSION_WRITE,
			"ns-2": identityv1.NamespaceAccess_PERMISSION_READ,
		})

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for different namespace count")
	}
}

func TestUserMatchesState_DifferentNamespacePermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:            types.StringValue("user-123"),
		State:         types.StringValue("active"),
		Email:         types.StringValue("test@example.com"),
		AccountAccess: internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: buildUserNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "read",
		}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-1": identityv1.NamespaceAccess_PERMISSION_WRITE,
		})

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for different namespace permission")
	}
}

func TestUserMatchesState_DifferentNamespaceID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:            types.StringValue("user-123"),
		State:         types.StringValue("active"),
		Email:         types.StringValue("test@example.com"),
		AccountAccess: internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: buildUserNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "write",
		}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-different": identityv1.NamespaceAccess_PERMISSION_WRITE,
		})

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for different namespace ID")
	}
}

func TestUserMatchesState_NullStateWithNoAPIAccesses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("test@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("admin"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_ADMIN,
		nil)

	if !userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return true for null state with no API accesses")
	}
}

func TestUserMatchesState_NullStateWithAPIAccesses(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("test@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: userNamespaceAccessAttrs}),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-1": identityv1.NamespaceAccess_PERMISSION_READ,
		})

	if userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return false for null state with API accesses")
	}
}

func TestUserMatchesState_ManyNamespaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Simulate the real-world scenario with many namespace accesses.
	stateAccesses := make(map[string]string, 70)
	apiAccesses := make(map[string]identityv1.NamespaceAccess_Permission, 70)
	for i := 0; i < 70; i++ {
		ns := fmt.Sprintf("namespace-%d.97055", i)
		stateAccesses[ns] = "write"
		apiAccesses[ns] = identityv1.NamespaceAccess_PERMISSION_WRITE
	}

	state := &userResourceModel{
		ID:                types.StringValue("user-123"),
		State:             types.StringValue("active"),
		Email:             types.StringValue("test@example.com"),
		AccountAccess:     internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: buildUserNamespaceAccessSet(t, ctx, stateAccesses),
	}
	user := makeUser("user-123", "test@example.com",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		apiAccesses)

	if !userMatchesState(ctx, state, user) {
		t.Error("expected userMatchesState to return true for many identical namespace accesses")
	}
}

// --- Service Account tests ---

func makeServiceAccount(id, name, description string, state resourcev1.ResourceState, role identityv1.AccountAccess_Role, nsAccesses map[string]identityv1.NamespaceAccess_Permission) *identityv1.ServiceAccount {
	accesses := make(map[string]*identityv1.NamespaceAccess, len(nsAccesses))
	for ns, perm := range nsAccesses {
		accesses[ns] = &identityv1.NamespaceAccess{Permission: perm}
	}
	return &identityv1.ServiceAccount{
		Id:    id,
		State: state,
		Spec: &identityv1.ServiceAccountSpec{
			Name:        name,
			Description: description,
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: role,
				},
				NamespaceAccesses: accesses,
			},
		},
	}
}

func TestServiceAccountMatchesState_Identical(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &serviceAccountResourceModel{
		ID:            types.StringValue("sa-123"),
		State:         types.StringValue("active"),
		Name:          types.StringValue("my-sa"),
		Description:   types.StringValue("test service account"),
		AccountAccess: internaltypes.CaseInsensitiveString("developer"),
		NamespaceAccesses: buildServiceAccountNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "write",
			"ns-2": "read",
		}),
		NamespaceScopedAccess: types.ObjectNull(serviceAccountNamespaceAccessAttrs),
	}
	sa := makeServiceAccount("sa-123", "my-sa", "test service account",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_DEVELOPER,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-1": identityv1.NamespaceAccess_PERMISSION_WRITE,
			"ns-2": identityv1.NamespaceAccess_PERMISSION_READ,
		})

	if !serviceAccountMatchesState(ctx, state, sa) {
		t.Error("expected serviceAccountMatchesState to return true for identical state")
	}
}

func TestServiceAccountMatchesState_DifferentName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &serviceAccountResourceModel{
		ID:                    types.StringValue("sa-123"),
		State:                 types.StringValue("active"),
		Name:                  types.StringValue("old-name"),
		Description:           types.StringValue(""),
		AccountAccess:         internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses:     types.SetNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}),
		NamespaceScopedAccess: types.ObjectNull(serviceAccountNamespaceAccessAttrs),
	}
	sa := makeServiceAccount("sa-123", "new-name", "",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		nil)

	if serviceAccountMatchesState(ctx, state, sa) {
		t.Error("expected serviceAccountMatchesState to return false for different name")
	}
}

func TestServiceAccountMatchesState_DifferentDescription(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &serviceAccountResourceModel{
		ID:                    types.StringValue("sa-123"),
		State:                 types.StringValue("active"),
		Name:                  types.StringValue("my-sa"),
		Description:           types.StringValue("old description"),
		AccountAccess:         internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses:     types.SetNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}),
		NamespaceScopedAccess: types.ObjectNull(serviceAccountNamespaceAccessAttrs),
	}
	sa := makeServiceAccount("sa-123", "my-sa", "new description",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		nil)

	if serviceAccountMatchesState(ctx, state, sa) {
		t.Error("expected serviceAccountMatchesState to return false for different description")
	}
}

func TestServiceAccountMatchesState_DifferentNamespacePermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &serviceAccountResourceModel{
		ID:            types.StringValue("sa-123"),
		State:         types.StringValue("active"),
		Name:          types.StringValue("my-sa"),
		Description:   types.StringValue(""),
		AccountAccess: internaltypes.CaseInsensitiveString("read"),
		NamespaceAccesses: buildServiceAccountNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "read",
		}),
		NamespaceScopedAccess: types.ObjectNull(serviceAccountNamespaceAccessAttrs),
	}
	sa := makeServiceAccount("sa-123", "my-sa", "",
		resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		identityv1.AccountAccess_ROLE_READ,
		map[string]identityv1.NamespaceAccess_Permission{
			"ns-1": identityv1.NamespaceAccess_PERMISSION_WRITE,
		})

	if serviceAccountMatchesState(ctx, state, sa) {
		t.Error("expected serviceAccountMatchesState to return false for different permission")
	}
}

func TestServiceAccountMatchesState_NamespaceScopedAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	nsObj, diags := types.ObjectValueFrom(ctx, serviceAccountNamespaceAccessAttrs, serviceAccountNamespaceAccessModel{
		NamespaceID: types.StringValue("ns-scoped"),
		Permission:  internaltypes.CaseInsensitiveString("admin"),
	})
	if diags.HasError() {
		t.Fatalf("failed to build namespace scoped access object: %v", diags)
	}

	state := &serviceAccountResourceModel{
		ID:                    types.StringValue("sa-123"),
		State:                 types.StringValue("active"),
		Name:                  types.StringValue("my-sa"),
		Description:           types.StringValue(""),
		AccountAccess:         internaltypes.CaseInsensitiveStringValue{StringValue: types.StringNull()},
		NamespaceAccesses:     types.SetNull(types.ObjectType{AttrTypes: serviceAccountNamespaceAccessAttrs}),
		NamespaceScopedAccess: nsObj,
	}

	sa := &identityv1.ServiceAccount{
		Id:    "sa-123",
		State: resourcev1.ResourceState_RESOURCE_STATE_ACTIVE,
		Spec: &identityv1.ServiceAccountSpec{
			Name:        "my-sa",
			Description: "",
			NamespaceScopedAccess: &identityv1.NamespaceScopedAccess{
				Namespace: "ns-scoped",
				Access: &identityv1.NamespaceAccess{
					Permission: identityv1.NamespaceAccess_PERMISSION_ADMIN,
				},
			},
		},
	}

	if !serviceAccountMatchesState(ctx, state, sa) {
		t.Error("expected serviceAccountMatchesState to return true for matching namespace-scoped access")
	}
}

// --- Group Access tests ---

func TestGroupAccessMatchesState_Identical(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &groupAccessResourceModel{
		ID:            types.StringValue("group-123"),
		AccountAccess: internaltypes.CaseInsensitiveString("none"),
		NamespaceAccesses: buildGroupNamespaceAccessSet(t, ctx, map[string]string{
			"ns-1": "write",
			"ns-2": "read",
		}),
	}

	group := &identityv1.UserGroup{
		Id: "group-123",
		Spec: &identityv1.UserGroupSpec{
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: identityv1.AccountAccess_ROLE_UNSPECIFIED,
				},
				NamespaceAccesses: map[string]*identityv1.NamespaceAccess{
					"ns-1": {Permission: identityv1.NamespaceAccess_PERMISSION_WRITE},
					"ns-2": {Permission: identityv1.NamespaceAccess_PERMISSION_READ},
				},
			},
		},
	}

	if !groupAccessMatchesState(ctx, state, group) {
		t.Error("expected groupAccessMatchesState to return true for identical state")
	}
}

func TestGroupAccessMatchesState_DifferentRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &groupAccessResourceModel{
		ID:                types.StringValue("group-123"),
		AccountAccess:     internaltypes.CaseInsensitiveString("none"),
		NamespaceAccesses: types.SetNull(types.ObjectType{AttrTypes: namespaceAccessAttrs}),
	}

	group := &identityv1.UserGroup{
		Id: "group-123",
		Spec: &identityv1.UserGroupSpec{
			Access: &identityv1.Access{
				AccountAccess: &identityv1.AccountAccess{
					Role: identityv1.AccountAccess_ROLE_DEVELOPER,
				},
			},
		},
	}

	if groupAccessMatchesState(ctx, state, group) {
		t.Error("expected groupAccessMatchesState to return false for different role")
	}
}

func TestGroupAccessMatchesState_NilGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	state := &groupAccessResourceModel{
		ID:            types.StringValue("group-123"),
		AccountAccess: internaltypes.CaseInsensitiveString("none"),
	}

	if groupAccessMatchesState(ctx, state, nil) {
		t.Error("expected groupAccessMatchesState to return false for nil group")
	}
}
