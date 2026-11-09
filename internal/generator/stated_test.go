package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// statedLeaf is a Marshaler leaf of kind with a Stated schema (#117).
func statedLeaf(kind models.ShapeKind, typ, format string) models.TypeShape {
	return models.TypeShape{Kind: kind, Name: "Money", Stated: &models.StatedSchema{Type: typ, Format: format}}
}

// TestSetTypeAndFormatStatedLeaf pins that a stated leaf emits exactly its
// stated type and format (no implied format, no unsigned floor) in place of
// {} or a bare string, at any depth, and that an unstated one is unchanged.
func TestSetTypeAndFormatStatedLeaf(t *testing.T) {
	cases := []struct {
		name  string
		shape models.TypeShape
		want  OpenAPIProperty
	}{
		{"marshaler string+format", statedLeaf(models.ShapeMarshaler, typeString, "decimal"), OpenAPIProperty{Type: typeString, Format: "decimal"}},
		{"marshaler integer, no format", statedLeaf(models.ShapeMarshaler, typeInteger, ""), OpenAPIProperty{Type: typeInteger}},
		{"text with format", statedLeaf(models.ShapeText, typeString, "date"), OpenAPIProperty{Type: typeString, Format: "date"}},
		{"pointer", ptrOf(statedLeaf(models.ShapeMarshaler, typeBoolean, "")), OpenAPIProperty{Type: typeBoolean}},
		{"unstated marshaler", marshalerOf("Money", models.TypeShape{}), OpenAPIProperty{}},
		{"unstated text", textLeaf("Day"), OpenAPIProperty{Type: typeString}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var prop OpenAPIProperty
			setTypeAndFormat(&prop, c.shape)
			assert.Equal(t, c.want, prop)
		})
	}

	var arr OpenAPIProperty
	setTypeAndFormat(&arr, sliceOf(statedLeaf(models.ShapeMarshaler, typeNumber, "double")))
	assert.Equal(t, &OpenAPIProperty{Type: typeNumber, Format: "double"}, arr.Items)
}

// TestStatedFieldProperty pins a stated field's property: nullable on a
// pointer, no validate keyword, and its example coerced to the stated type.
func TestStatedFieldProperty(t *testing.T) {
	resolution := ptrOf(statedLeaf(models.ShapeMarshaler, typeInteger, ""))
	f := &models.FieldInfo{
		Name: "Count", JSONName: "count", Shape: ptrOf(named("Count")), Resolution: &resolution,
		Example: "7", Constraints: map[string]string{"min": "1", "max": "3"},
	}
	prop := fieldProperty(f)
	assert.Equal(t, typeInteger, prop.Type)
	assert.True(t, prop.Nullable)
	assert.Nil(t, prop.Minimum)
	assert.Nil(t, prop.Maximum)
	assert.Equal(t, int64(7), prop.Example)

	f.Example = "seven"
	assert.Nil(t, fieldProperty(f).Example, "an example with no integer form is dropped")
}

// TestStatedLeafIsNeverAQuotedScalar pins that json ",string" never quotes a
// Marshaler leaf, even one stated as a number: encoding/json writes it
// through its own method.
func TestStatedLeafIsNeverAQuotedScalar(t *testing.T) {
	leaf := statedLeaf(models.ShapeMarshaler, typeInteger, "")
	f := &models.FieldInfo{Name: "C", JSONName: "c", JSONString: true, Shape: named("Count"), Resolution: &leaf}
	prop := &OpenAPIProperty{Type: typeInteger}
	assert.False(t, isQuotedScalar(f, prop))
}
