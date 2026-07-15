package enums

import "testing"

func TestCustomRoleResourceTypes(t *testing.T) {
	t.Parallel()

	want := []string{
		"accounts",
		"projects",
		"namespaces",
		"nexus-endpoints",
		"connectivity-rules",
		"custom-roles",
	}

	if len(CustomRoleResourceTypes) != len(want) {
		t.Fatalf("expected %d resource types, got %d", len(want), len(CustomRoleResourceTypes))
	}

	for i, value := range want {
		if CustomRoleResourceTypes[i] != value {
			t.Fatalf("expected resource type %q at index %d, got %q", value, i, CustomRoleResourceTypes[i])
		}
	}
}
