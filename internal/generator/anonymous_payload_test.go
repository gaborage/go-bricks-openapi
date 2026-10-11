package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Issue 118: an anonymous struct payload is documented inline.

// TestAnonymousStructPayloadSchema pins an anonymous struct payload's data
// schema: the inline object a field of its type gets, every pointer shed and
// never nullable, and never annotated as the untyped fallback, even empty.
func TestAnonymousStructPayloadSchema(t *testing.T) {
	data := func(r models.TypeShape) *OpenAPIProperty {
		return successEnvelopeSchema(resolvedPayloadOf(r)).Properties[propNameData]
	}
	x := anonOf(requiredX())
	cases := []struct {
		name string
		r    models.TypeShape
		want *OpenAPIProperty
	}{
		{"direct", x, xObject()},
		{"pointer", ptrOf(ptrOf(x)), xObject()},
		{"empty", anonOf(), &OpenAPIProperty{Type: typeObject}},
		{"empty pointer", ptrOf(anonOf()), &OpenAPIProperty{Type: typeObject}},
		{"slice", sliceOf(x), &OpenAPIProperty{Type: typeArray, Items: xObject()}},
		{"map", mapOf(prim(goTypeString), x), &OpenAPIProperty{Type: typeObject, AdditionalProperties: xObject()}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, data(c.r), c.name)
	}
}

// TestInlineStructPayload pins the annotation guard's predicate: a payload
// whose Resolution, every pointer shed, is an anonymous struct.
func TestInlineStructPayload(t *testing.T) {
	assert.True(t, inlineStructPayload(resolvedPayloadOf(anonOf())))
	assert.True(t, inlineStructPayload(resolvedPayloadOf(ptrOf(anonOf()))))
	assert.False(t, inlineStructPayload(resolvedPayloadOf(sliceOf(anonOf()))))
	assert.False(t, inlineStructPayload(resolvedPayloadOf(prim(goTypeUintptr))))
	s := anonOf()
	assert.False(t, inlineStructPayload(&models.TypeInfo{Shape: &s}), "a payload with no Resolution")
	assert.False(t, inlineStructPayload(nil))
}
