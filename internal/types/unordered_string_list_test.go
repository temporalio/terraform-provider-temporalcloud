package types

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestUnorderedStringListValue_ListSemanticEquals(t *testing.T) {
	testCases := []struct {
		ListA         []string
		ListB         []string
		ExpectedEqual bool
	}{
		{
			ListA:         nil,
			ListB:         nil,
			ExpectedEqual: true,
		},
		{
			ListA:         []string{},
			ListB:         []string{},
			ExpectedEqual: true,
		},
		{
			ListA:         []string{""},
			ListB:         []string{""},
			ExpectedEqual: true,
		},
		{
			ListA:         []string{"test-2"},
			ListB:         []string{"test-1"},
			ExpectedEqual: false,
		},
		{
			ListA:         []string{"test-1"},
			ListB:         []string{"test-2", "test-3", "test-1"},
			ExpectedEqual: false,
		},
		{
			ListA:         []string{"test-1", "test-2", "test-3"},
			ListB:         []string{"test-2"},
			ExpectedEqual: false,
		},
		{
			ListA:         []string{"test-1", "test-2", "test-3"},
			ListB:         []string{"test-2", "test-3", "test-1"},
			ExpectedEqual: true,
		},
		{
			ListA:         []string{"test-1", "test-2", "test-3"},
			ListB:         []string{"test-1", "test-2", "test-3"},
			ExpectedEqual: true,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test-case-%d", i), func(t *testing.T) {
			listA, listDiags := types.ListValueFrom(context.Background(), types.StringType, tc.ListA)
			if listDiags.HasError() {
				t.Fatal(listDiags.Errors())
			}

			listB, listDiags := types.ListValueFrom(context.Background(), types.StringType, tc.ListB)
			if listDiags.HasError() {
				t.Fatal(listDiags.Errors())
			}

			listAUnordered := UnorderedStringListValue{ListValue: listA}
			listBUnordered := UnorderedStringListValue{ListValue: listB}

			equal, diags := listAUnordered.ListSemanticEquals(context.Background(), listBUnordered)
			if diags.HasError() {
				t.Fatal(diags.Errors())
			}

			if equal != tc.ExpectedEqual {
				t.Fatalf("Expected %v, got %v", tc.ExpectedEqual, equal)
			}
		})
	}
}
