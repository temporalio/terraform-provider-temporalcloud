package types

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestZeroObject(t *testing.T) {
	type testObject struct {
		boolVal                bool
		stringVal              string
		floatVal               float64
		listVal                []string
		mapVal                 map[string]int
		testObject             *testObject
		testZeroObject         *testObject
		unorderedStringListVal []string
	}

	toObjectFunc := func(testObj *testObject) ZeroObjectValue {
		if testObj != nil {
			listVal := make([]attr.Value, len(testObj.listVal))
			for i, v := range testObj.listVal {
				listVal[i] = types.StringValue(v)
			}
			mapVal := make(map[string]attr.Value, len(testObj.mapVal))
			for k, v := range testObj.mapVal {
				mapVal[k] = types.Int64Value(int64(v))
			}

			var objVal attr.Value
			if testObj.testObject != nil {
				objVal = types.ObjectValueMust(
					map[string]attr.Type{"boolVal": types.BoolType},
					map[string]attr.Value{"boolVal": types.BoolValue(testObj.testObject.boolVal)},
				)
			} else {
				objVal = types.ObjectNull(
					map[string]attr.Type{"boolVal": types.BoolType},
				)
			}

			var zeroObjVal attr.Value
			if testObj.testZeroObject != nil {
				zeroObjVal = ZeroObjectValue{
					ObjectValue: types.ObjectValueMust(
						map[string]attr.Type{"boolVal": types.BoolType},
						map[string]attr.Value{"boolVal": types.BoolValue(testObj.testZeroObject.boolVal)},
					),
				}
			} else {
				zeroObjVal = ZeroObjectValue{
					ObjectValue: types.ObjectNull(
						map[string]attr.Type{"boolVal": types.BoolType},
					),
				}
			}

			var unorderedListVal attr.Value
			if len(testObj.unorderedStringListVal) > 0 {
				list, _ := types.ListValueFrom(context.Background(), types.StringType, testObj.unorderedStringListVal)
				unorderedListVal = UnorderedStringListValue{
					ListValue: list,
				}
			} else {
				unorderedListVal = UnorderedStringListValue{
					ListValue: types.ListNull(types.StringType),
				}
			}

			return ZeroObjectValue{
				ObjectValue: types.ObjectValueMust(
					map[string]attr.Type{
						"boolVal":   types.BoolType,
						"stringVal": types.StringType,
						"floatVal":  types.Float64Type,
						"listVal":   types.ListType{ElemType: types.StringType},
						"mapVal":    types.MapType{ElemType: types.Int64Type},
						"testZeroObject": ZeroObjectType{
							ObjectType: types.ObjectType{
								AttrTypes: map[string]attr.Type{
									"boolVal": types.BoolType,
								},
							},
						},
						"testObject": types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"boolVal": types.BoolType,
							},
						},
						"unorderedStringListVal": UnorderedStringListType{
							ListType: types.ListType{ElemType: types.StringType},
						},
					},
					map[string]attr.Value{
						"boolVal":                types.BoolValue(testObj.boolVal),
						"stringVal":              types.StringValue(testObj.stringVal),
						"floatVal":               types.Float64Value(testObj.floatVal),
						"listVal":                types.ListValueMust(types.StringType, listVal),
						"mapVal":                 types.MapValueMust(types.Int64Type, mapVal),
						"testZeroObject":         zeroObjVal,
						"testObject":             objVal,
						"unorderedStringListVal": unorderedListVal,
					},
				),
			}
		} else {
			return ZeroObjectValue{
				ObjectValue: types.ObjectNull(
					map[string]attr.Type{
						"boolVal":    types.BoolType,
						"stringVal":  types.StringType,
						"floatVal":   types.Float64Type,
						"listVal":    types.ListType{ElemType: types.StringType},
						"mapVal":     types.MapType{ElemType: types.Int64Type},
						"testObject": types.ObjectType{},
					},
				),
			}
		}
	}

	testCases := []struct {
		Name          string
		ObjectA       *testObject
		ObjectB       *testObject
		ExpectedEqual bool
	}{
		{
			Name: "all same",
			ObjectA: &testObject{
				boolVal:   true,
				stringVal: "test",
				floatVal:  1.0,
				listVal:   []string{"a", "b"},
				mapVal:    map[string]int{"key1": 1},
				testObject: &testObject{
					boolVal: true,
				},
				testZeroObject: &testObject{
					boolVal: true,
				},
			},
			ObjectB: &testObject{
				boolVal:   true,
				stringVal: "test",
				floatVal:  1.0,
				listVal:   []string{"a", "b"},
				mapVal:    map[string]int{"key1": 1},
				testObject: &testObject{
					boolVal: true,
				},
				testZeroObject: &testObject{
					boolVal: true,
				},
			},
			ExpectedEqual: true,
		},
		{
			Name: "different bool",
			ObjectA: &testObject{
				boolVal:   true,
				stringVal: "test",
				floatVal:  1.0,
				listVal:   []string{"a", "b"},
				mapVal:    map[string]int{"key1": 1},
				testObject: &testObject{
					boolVal: true,
				},
				testZeroObject: &testObject{
					boolVal: true,
				},
			},
			ObjectB: &testObject{
				boolVal:   false,
				stringVal: "test",
				floatVal:  1.0,
				listVal:   []string{"a", "b"},
				mapVal:    map[string]int{"key1": 1},
				testObject: &testObject{
					boolVal: true,
				},
				testZeroObject: &testObject{
					boolVal: true,
				},
			},
		},
		{
			Name: "zero unordered string list vs nil",
			ObjectA: &testObject{
				unorderedStringListVal: []string{},
			},
			ObjectB: &testObject{
				unorderedStringListVal: nil,
			},
			ExpectedEqual: true,
		},
		{
			Name: "object nil vs zero",
			ObjectA: &testObject{
				testObject: &testObject{},
			},
			ObjectB: &testObject{
				testObject: nil,
			},
		},
		{
			Name: "zero object nil vs zero",
			ObjectA: &testObject{
				testZeroObject: &testObject{},
			},
			ObjectB: &testObject{
				testZeroObject: nil,
			},
			ExpectedEqual: true,
		},
		{
			Name:          "nil vs zero",
			ObjectA:       &testObject{},
			ObjectB:       nil,
			ExpectedEqual: true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("test-case-%s", tc.Name), func(t *testing.T) {
			objectA := toObjectFunc(tc.ObjectA)
			objectB := toObjectFunc(tc.ObjectB)

			equal, diags := objectA.ObjectSemanticEquals(context.Background(), objectB)
			if diags.HasError() {
				t.Fatal(diags.Errors())
			}

			if equal != tc.ExpectedEqual {
				t.Fatalf("Expected %v, got %v", tc.ExpectedEqual, equal)
			}
		})
	}
}
