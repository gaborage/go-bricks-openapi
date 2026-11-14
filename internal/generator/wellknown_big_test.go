package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// pointerTo is a pointer shape to s.
func pointerTo(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePointer, Elem: &s}
}

// TestBigTypesTypedOnlyBehindPointer pins math/big's entries: *big.Int is an
// integer and *big.Float and *big.Rat strings, each with no format, as a
// pointer field (nullable), a slice item and a map value; a value one that
// reaches the generator is {} (ptrOnly), as is a slice of them.
func TestBigTypesTypedOnlyBehindPointer(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	for name, typ := range map[string]string{goTypeBigInt: typeInteger, goTypeBigFloat: typeString, goTypeBigRat: typeString} {
		wk, ok := wellKnownShape(named(name))
		require.True(t, ok, name)
		assert.Equal(t, wellKnownType{typ: typ, ptrOnly: true}, wk, name)

		leaf := &OpenAPIProperty{Type: typ}
		ptr := pointerTo(named(name))
		assert.Equal(t, &OpenAPIProperty{Type: typ, Nullable: true},
			gen.fieldInfoToProperty(&models.FieldInfo{Shape: ptr, JSONName: "v"}), name)
		assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: leaf},
			gen.fieldInfoToProperty(&models.FieldInfo{Shape: sliceOf(ptr), JSONName: "v"}), name)
		assert.Equal(t, &OpenAPIProperty{Type: typeObject, AdditionalProperties: leaf},
			gen.fieldInfoToProperty(&models.FieldInfo{Shape: mapOf(prim(goTypeString), ptr), JSONName: "v"}), name)
		assert.Equal(t, leaf, payloadShapeSchema(ptr), name)

		assert.Equal(t, &OpenAPIProperty{}, gen.fieldInfoToProperty(&models.FieldInfo{Shape: named(name), JSONName: "v"}), name)
		assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{}},
			gen.fieldInfoToProperty(&models.FieldInfo{Shape: sliceOf(named(name)), JSONName: "v"}), name)
		assert.Equal(t, &OpenAPIProperty{}, payloadShapeSchema(named(name)), name)
	}
}

// TestOnlyBigTypesArePointerOnly pins that ptrOnly marks math/big's three
// entries and no other: every other well-known type is typed by value too.
func TestOnlyBigTypesArePointerOnly(t *testing.T) {
	for name, wk := range wellKnownFormats {
		want := name == goTypeBigInt || name == goTypeBigFloat || name == goTypeBigRat
		assert.Equal(t, want, wk.ptrOnly, name)
	}
}

// TestBigTypesIgnoreStringOption pins that a ",string" json option leaves the
// three as typed: they are structs, which encoding/json never quotes, so
// *big.Int,string is still a JSON number and *big.Float,string a string,
// while a plain *int64,string is still quoted.
func TestBigTypesIgnoreStringOption(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	for name, typ := range map[string]string{goTypeBigInt: typeInteger, goTypeBigFloat: typeString, goTypeBigRat: typeString} {
		f := &models.FieldInfo{Shape: pointerTo(named(name)), JSONName: "v", JSONString: true}
		assert.Equal(t, &OpenAPIProperty{Type: typ, Nullable: true}, gen.fieldInfoToProperty(f), name)
		assert.False(t, isQuotedScalar(f, &OpenAPIProperty{Type: typ}), name)
	}
	plain := &models.FieldInfo{Shape: pointerTo(prim(goTypeInt64)), JSONName: "v", JSONString: true}
	assert.True(t, isQuotedScalar(plain, &OpenAPIProperty{Type: typeInteger}))
	assert.Equal(t, typeString, gen.fieldInfoToProperty(plain).Type)
}
