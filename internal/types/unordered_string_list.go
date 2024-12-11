package types

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"slices"
)

var _ basetypes.ListTypable = UnorderedStringListType{}
var _ basetypes.ListValuableWithSemanticEquals = UnorderedStringListValue{}

type UnorderedStringListType struct {
	basetypes.ListType
}

func (t UnorderedStringListType) WithElementType(typ attr.Type) attr.TypeWithElementType {
	elemType, ok := t.ListType.WithElementType(typ).(basetypes.ListType)
	if !ok {
		// Shouldn't happen. Here to make linters happy
		panic("unexpected element type")
	}

	return UnorderedStringListType{
		ListType: elemType,
	}
}

func (t UnorderedStringListType) Equal(o attr.Type) bool {
	other, ok := o.(UnorderedStringListType)
	if !ok {
		return false
	}

	return t.ListType.Equal(other.ListType)
}

func (t UnorderedStringListType) String() string {
	return "UnorderedStringListType"
}

func (t UnorderedStringListType) ValueFromList(ctx context.Context, in basetypes.ListValue) (basetypes.ListValuable, diag.Diagnostics) {
	var diags diag.Diagnostics
	if _, ok := in.ElementType(ctx).(basetypes.StringTypable); !ok {
		diags.AddError(
			"Unexpected List Element Type Error",
			"An unexpected value type was received while using custom UnorderedStringListType. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", basetypes.StringType{})+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", in.ElementType(ctx)),
		)
		return nil, diags
	}

	value := UnorderedStringListValue{
		ListValue: in,
	}

	return value, nil
}

func (t UnorderedStringListType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.ListType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	listValue, ok := attrValue.(basetypes.ListValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	listValuable, diags := t.ValueFromList(ctx, listValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting ListValue to ListValuable: %v", diags)
	}

	return listValuable, nil
}

func (t UnorderedStringListType) ValueType(ctx context.Context) attr.Value {
	listValue, ok := t.ListType.ValueType(ctx).(basetypes.ListValue)
	if !ok {
		// Shouldn't happen. Here to make linters happy
		panic("unexpected value type")
	}

	return UnorderedStringListValue{
		ListValue: listValue,
	}
}

type UnorderedStringListValue struct {
	basetypes.ListValue
}

func (v UnorderedStringListValue) Equal(o attr.Value) bool {
	other, ok := o.(UnorderedStringListValue)
	if !ok {
		return false
	}

	return v.ListValue.Equal(other.ListValue)
}

func (v UnorderedStringListValue) Type(ctx context.Context) attr.Type {
	listType, ok := basetypes.ListType{}.WithElementType(v.ElementType(ctx)).(basetypes.ListType)
	if !ok {
		// Shouldn't happen. Here to make linters happy
		panic("unexpected list type")
	}
	return UnorderedStringListType{
		ListType: listType,
	}
}

func (v UnorderedStringListValue) ListSemanticEquals(ctx context.Context, newValuable basetypes.ListValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(UnorderedStringListValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", newValuable),
		)

		return false, diags
	}

	// If they are not the same length then return early
	if len(v.ListValue.Elements()) != len(newValue.ListValue.Elements()) {
		return false, diags
	}

	currentValueArray := make([]string, 0, len(v.ListValue.Elements()))
	diags.Append(v.ElementsAs(ctx, &currentValueArray, false)...)
	if diags.HasError() {
		return false, diags
	}

	newValueArray := make([]string, 0, len(newValue.Elements()))
	diags.Append(newValue.ElementsAs(ctx, &newValueArray, false)...)
	if diags.HasError() {
		return false, diags
	}

	slices.Sort(currentValueArray)
	slices.Sort(newValueArray)

	for i, value := range currentValueArray {
		if newValueArray[i] != value {
			return false, diags
		}
	}

	return true, diags
}
