package types

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ basetypes.ObjectTypable                    = ZeroObjectType{}
	_ basetypes.ObjectValuableWithSemanticEquals = ZeroObjectValue{}
)

type ZeroObjectType struct {
	basetypes.ObjectType
}

func (t ZeroObjectType) WithAttributeTypes(typs map[string]attr.Type) attr.TypeWithAttributeTypes {
	elemType, ok := t.ObjectType.WithAttributeTypes(typs).(basetypes.ObjectType)
	if !ok {
		// Shouldn't happen. Here to make linters happy
		panic("unexpected element type")
	}

	return ZeroObjectType{
		ObjectType: elemType,
	}
}

func (t ZeroObjectType) Equal(o attr.Type) bool {
	other, ok := o.(ZeroObjectType)
	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t ZeroObjectType) String() string {
	return "ZeroObjectType"
}

func (t ZeroObjectType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	value := ZeroObjectValue{
		ObjectValue: in,
	}
	return value, nil
}

func (t ZeroObjectType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.ObjectType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	objectValue, ok := attrValue.(basetypes.ObjectValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	objectValuable, diags := t.ValueFromObject(ctx, objectValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting ObjectValue to ObjectValuable: %v", diags)
	}

	return objectValuable, nil
}

func (t ZeroObjectType) ValueType(ctx context.Context) attr.Value {
	objectValue, ok := t.ObjectType.ValueType(ctx).(basetypes.ObjectValue)
	if !ok {
		// Shouldn't happen. Here to make linters happy
		panic("unexpected value type")
	}

	return ZeroObjectValue{
		ObjectValue: objectValue,
	}
}

type ZeroObjectValue struct {
	basetypes.ObjectValue
}

func (v ZeroObjectValue) Equal(o attr.Value) bool {
	other, ok := o.(ZeroObjectValue)
	if !ok {
		return false
	}

	return v.ObjectValue.Equal(other.ObjectValue)
}

func (v ZeroObjectValue) Type(ctx context.Context) attr.Type {
	objectType, ok := basetypes.ObjectType{}.WithAttributeTypes(v.AttributeTypes(ctx)).(basetypes.ObjectType)
	if !ok {
		// Shouldn't happen. Here to make linters happy
		panic("unexpected object type")
	}
	return ZeroObjectType{
		ObjectType: objectType,
	}
}

func (v ZeroObjectValue) ObjectSemanticEquals(ctx context.Context, newValuable basetypes.ObjectValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(ZeroObjectValue)
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

	diags.AddWarning(fmt.Sprintf("comparing: %v and %v", v.String(), newValue.String()), "comparing")
	if v.IsZero(ctx) && newValue.IsZero(ctx) {
		// Both values are zero, so they are semantically equal.
		return true, diags
	}

	// If both values are not zero, we compare them for equality.
	return v.Equal(newValue), diags
}

func (v ZeroObjectValue) IsZero(ctx context.Context) bool {
	for _, attrValue := range v.Attributes() {
		switch value := attrValue.(type) {
		case basetypes.StringValue:
			if value.ValueString() != "" {
				return false
			}
		case basetypes.BoolValue:
			if value.ValueBool() {
				return false
			}
		case basetypes.NumberValue:
			if value.ValueBigFloat().Cmp(new(big.Float)) != 0 {
				return false
			}
		case basetypes.Float64Value:
			if value.ValueFloat64() != 0.0 {
				return false
			}
		case basetypes.ListValue:
			if len(value.Elements()) > 0 {
				return false
			}
		case basetypes.MapValue:
			if len(value.Elements()) > 0 {
				return false
			}
		case ZeroObjectValue:
			if !value.IsZero(ctx) {
				return false
			}
		case basetypes.ObjectValue:
			if !value.IsNull() {
				return false
			}
		}
	}
	return true
}
