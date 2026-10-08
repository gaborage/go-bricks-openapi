package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// byteItemProp is what a bare byte field emits: the items of a [N]byte array.
func byteItemProp() *OpenAPIProperty {
	return &OpenAPIProperty{Type: typeInteger, Format: formatInt32, Minimum: floatPtr(0)}
}

// TestFixedByteArrayFieldIsIntegerArray pins #98's generator rule:
// encoding/json base64-encodes only a byte SLICE, so a [N]byte or [N]uint8
// field — bare, behind a pointer (with no nullable, like *[4]int), or nested in
// a slice, an array or a map — is an array of the integer a bare byte emits.
// A named byte-kind element (type Flag byte, resolved to its byte builtin;
// built here as uint8, which types the same) is the same: [4]Flag is an
// integer array, since a fixed array is never base64 and only a byte slice is.
func TestFixedByteArrayFieldIsIntegerArray(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	byteArr := &OpenAPIProperty{Type: typeArray, Items: byteItemProp()}
	cases := map[string]struct {
		shape models.TypeShape
		want  *OpenAPIProperty
		res   *models.TypeShape
	}{
		"[4]byte":            {arrayOf(prim(goTypeByte)), byteArr, nil},
		"[4]uint8":           {arrayOf(prim(goTypeUint8)), byteArr, nil},
		"*[4]byte":           {ptrOf(arrayOf(prim(goTypeByte))), byteArr, nil},
		"[][4]byte":          {sliceOf(arrayOf(prim(goTypeByte))), &OpenAPIProperty{Type: typeArray, Items: byteArr}, nil},
		"[2][4]byte":         {arrayOf(arrayOf(prim(goTypeByte))), &OpenAPIProperty{Type: typeArray, Items: byteArr}, nil},
		"map[string][4]byte": {mapOf(prim(goTypeString), arrayOf(prim(goTypeByte))), &OpenAPIProperty{Type: typeObject, AdditionalProperties: byteArr}, nil},
		"[2][]byte":          {arrayOf(sliceOf(prim(goTypeByte))), &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{Type: typeString, Format: formatByte}}, nil},
		"[]byte":             {sliceOf(prim(goTypeByte)), &OpenAPIProperty{Type: typeString, Format: formatByte}, nil},
		"[4]Flag":            {arrayOf(named("Flag")), byteArr, shapePtr(arrayOf(prim(goTypeUint8)))},
		"[2][3]Flag":         {arrayOf(arrayOf(named("Flag"))), &OpenAPIProperty{Type: typeArray, Items: byteArr}, shapePtr(arrayOf(arrayOf(prim(goTypeUint8))))},
		"[]Flag":             {sliceOf(named("Flag")), &OpenAPIProperty{Type: typeString, Format: formatByte}, shapePtr(sliceOf(prim(goTypeUint8)))},
	}
	for name, c := range cases {
		f := &models.FieldInfo{Shape: c.shape, JSONName: "v"}
		if c.res != nil {
			f = withResolution(f, *c.res)
		}
		assert.Equal(t, c.want, gen.fieldInfoToProperty(f), name)
	}
}

func shapePtr(s models.TypeShape) *models.TypeShape { return &s }

// TestFixedByteArrayTagsMatchUint16Array pins that a [4]byte field takes the
// validate and example tags exactly as a [4]uint16 field does: cardinality on
// the array and the dive bound on its items (the base64 rule used to drop
// them), and no scalar example on an array.
func TestFixedByteArrayTagsMatchUint16Array(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	field := func(elem string) *models.FieldInfo {
		return &models.FieldInfo{
			Shape: arrayOf(prim(elem)), JSONName: "v", Example: "AQIDBA==",
			Constraints:        map[string]string{"min": "1", "max": "4"},
			ElementConstraints: map[string]string{"max": "200"},
		}
	}
	got := gen.fieldInfoToProperty(field(goTypeByte))
	assert.Equal(t, gen.fieldInfoToProperty(field(goTypeUint16)), got)
	// A named byte-kind array ([4]Flag) keeps cardinality and dive bounds too.
	flag := field(goTypeByte)
	flag.Shape = arrayOf(named("Flag"))
	assert.Equal(t, got, gen.fieldInfoToProperty(withResolution(flag, arrayOf(prim(goTypeUint8)))), "[4]Flag")
	assert.Equal(t, &OpenAPIProperty{
		Type: typeArray, MinItems: intPtr(1), MaxItems: intPtr(4),
		Items: &OpenAPIProperty{Type: typeInteger, Format: formatInt32, Minimum: floatPtr(0), Maximum: floatPtr(200)},
	}, got)
}

// TestFixedArrayFieldMatchesSliceForm pins that every consumer other than the
// two base64 rules treats an array exactly like a slice: a struct element
// ($ref items), a map of struct arrays, a named non-byte scalar element at any
// depth (a byte-kind element is #98's exception, pinned in
// TestFixedByteArrayFieldIsIntegerArray), a pointer to an array (no
// nullable), dive rules on a builtin element, and an unmodeled element.
func TestFixedArrayFieldMatchesSliceForm(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	cents := func(f *models.FieldInfo) *models.FieldInfo { return resolvedTo(f, prim(goTypeInt64)) }
	address := func(f *models.FieldInfo) *models.FieldInfo { return resolvedTo(f, refTo("Address")) }
	cases := map[string]func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo{
		"[2]Address": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return address(&models.FieldInfo{Shape: seq(named("Address")), Constraints: map[string]string{"max": "3"}})
		},
		"*[2]*Address": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return address(&models.FieldInfo{Shape: ptrOf(seq(ptrOf(named("Address"))))})
		},
		"map[string][2]Address": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return address(&models.FieldInfo{Shape: mapOf(prim(goTypeString), seq(named("Address")))})
		},
		"[4]Cents": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return cents(&models.FieldInfo{Shape: seq(named("Cents")), ElementConstraints: map[string]string{"max": "9"}})
		},
		"[2][3]Cents": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return cents(&models.FieldInfo{Shape: seq(seq(named("Cents")))})
		},
		"*[3]int": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return &models.FieldInfo{Shape: ptrOf(seq(prim(goTypeInt)))}
		},
		"[2]string dive": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return &models.FieldInfo{Shape: seq(prim(goTypeString)), ElementConstraints: map[string]string{"min": "2"}}
		},
		"*[2]string dive": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return &models.FieldInfo{Shape: ptrOf(seq(prim(goTypeString))), ElementConstraints: map[string]string{"min": "2"}}
		},
		"[2]chan": func(seq func(models.TypeShape) models.TypeShape) *models.FieldInfo {
			return &models.FieldInfo{Shape: seq(unknownShape())}
		},
	}
	for name, build := range cases {
		slice := gen.fieldInfoToProperty(build(sliceOf))
		require.Equal(t, typeArray, firstArray(slice).Type, "%s: oracle sanity — the slice form is an array", name)
		assert.Equal(t, slice, gen.fieldInfoToProperty(build(arrayOf)), name)
	}
}

// firstArray returns prop, or its additionalProperties for a map schema.
func firstArray(prop *OpenAPIProperty) *OpenAPIProperty {
	if prop.AdditionalProperties != nil {
		return prop.AdditionalProperties
	}
	return prop
}

// TestFixedArrayPayloadSchema pins the payload side of #98: a [4]byte payload
// is an integer array (not base64), a struct-element array $refs its
// component, and a well-known-element array is inline — and only the struct
// element names a component.
func TestFixedArrayPayloadSchema(t *testing.T) {
	byteArr := &OpenAPIProperty{Type: typeArray, Items: byteItemProp()}
	assert.Equal(t, byteArr, responsePayloadSchema(&models.TypeInfo{Shape: payloadArray(prim(goTypeByte))}))
	assert.Equal(t, byteArr, responsePayloadSchema(&models.TypeInfo{Shape: payloadArray(prim(goTypeUint8))}))
	assert.Equal(t, &OpenAPIProperty{Type: typeString, Format: formatByte},
		responsePayloadSchema(&models.TypeInfo{Shape: payloadSlice(prim(goTypeByte))}), "a []byte payload stays base64")

	addr := &models.TypeInfo{Name: "Address", Package: "mod", Shape: payloadArray(named("Address"))}
	assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{Ref: refPath("Address")}}, responsePayloadSchema(addr))
	assert.True(t, payloadNamesComponent(addr))

	times := &models.TypeInfo{Name: "Time", Package: "time", Shape: payloadArray(named(goTypeTimeTime))}
	assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{Type: typeString, Format: formatDateTime}}, responsePayloadSchema(times))
	assert.False(t, payloadNamesComponent(times), "a well-known element is inline, never a component")
}
