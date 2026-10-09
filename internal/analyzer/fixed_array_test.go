package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestTypeShapeKeepsArraysApartFromSlices pins #98: encoding/json base64-encodes
// only a byte SLICE, so the decoder must keep a fixed-size array distinct. An
// array is any ArrayType with a Len — a zero length and a constant length (a
// value the AST cannot read) included.
func TestTypeShapeKeepsArraysApartFromSlices(t *testing.T) {
	a := New("")
	cases := map[string]models.TypeShape{
		"[3]int":  {Kind: models.ShapeArray, Elem: &models.TypeShape{Kind: models.ShapePrimitive, Name: "int"}},
		"[0]byte": {Kind: models.ShapeArray, Elem: &models.TypeShape{Kind: models.ShapePrimitive, Name: "byte"}},
		"[N]byte": {Kind: models.ShapeArray, Elem: &models.TypeShape{Kind: models.ShapePrimitive, Name: "byte"}},
		"[]int":   {Kind: models.ShapeSlice, Elem: &models.TypeShape{Kind: models.ShapePrimitive, Name: "int"}},
		"[]byte":  {Kind: models.ShapeSlice, Elem: &models.TypeShape{Kind: models.ShapePrimitive, Name: "byte"}},
	}
	for src, want := range cases {
		assert.Equal(t, want, a.typeShape(mustParse(t, src)), src)
	}
}

// fixedArrayPayloadShapes is the payload Shape each fixed-size array type
// argument must resolve to: the array kind survives the payload path, the
// element pointer is kept (#110), and the element, one pointer level shed,
// keeps describing the component, exactly as for a slice payload. rendered is
// the Shape in renderShape notation; base is payloadBaseShape's element.
var fixedArrayPayloadShapes = map[string]struct {
	name     string
	rendered string
	base     models.TypeShape
}{
	"[4]byte":     {rendered: "[N]byte", base: models.TypeShape{Kind: models.ShapePrimitive, Name: "byte"}},
	"[32]uint8":   {rendered: "[N]uint8", base: models.TypeShape{Kind: models.ShapePrimitive, Name: "uint8"}},
	"[4]*byte":    {rendered: "[N]*byte", base: models.TypeShape{Kind: models.ShapePrimitive, Name: "byte"}},
	"[3]int":      {rendered: "[N]int", base: models.TypeShape{Kind: models.ShapePrimitive, Name: "int"}},
	"[2]Address":  {name: "Address", rendered: "[N]Address", base: models.TypeShape{Kind: models.ShapeNamed, Name: "Address"}},
	"[2]*Address": {name: "Address", rendered: "[N]*Address", base: models.TypeShape{Kind: models.ShapeNamed, Name: "Address"}},
}

// TestFixedArrayPayloadShapes pins the second decode site (#98):
// slicePayloadTypeInfo builds its own Shape, so a server.Result[[4]byte], and
// the same array as a bare return, must keep the array kind there too — and a
// struct-element array keeps that Shape rather than being shed like a
// registered non-slice payload.
func TestFixedArrayPayloadShapes(t *testing.T) {
	for payload, want := range fixedArrayPayloadShapes {
		for _, result := range []string{payload, "server.Result[" + payload + "]", "server.ResultWithMeta[" + payload + "]"} {
			t.Run(result, func(t *testing.T) {
				ti, warnings := analyzeBareReturn(t, result)
				require.NotNil(t, ti)
				require.NotNil(t, ti.Shape, "an array payload keeps its Shape: the array wrapper is built from it")
				assert.Equal(t, models.ShapeArray, ti.Shape.Kind)
				assert.Equal(t, want.rendered, renderShape(*ti.Shape))
				assert.Equal(t, want.base, payloadBaseShape(*ti.Shape), "an array payload is named by its element")
				assert.Equal(t, want.name, ti.Name)
				assert.Empty(t, warnings)
			})
		}
	}
}

// TestPayloadBaseShapeUnwrapsArray pins payloadBaseShape's array arm directly:
// an array payload's base is its element, with one pointer level shed.
func TestPayloadBaseShapeUnwrapsArray(t *testing.T) {
	addr := models.TypeShape{Kind: models.ShapeNamed, Name: "Address"}
	ptr := models.TypeShape{Kind: models.ShapePointer, Elem: &addr}
	assert.Equal(t, addr, payloadBaseShape(models.TypeShape{Kind: models.ShapeArray, Elem: &ptr}))
}
