package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// resolvedPayloadOf returns a nameless payload whose Shape and Resolution are r.
func resolvedPayloadOf(r models.TypeShape) *models.TypeInfo {
	s := r
	return &models.TypeInfo{Shape: &s, Resolution: &r}
}

// TestResponsePayloadSchemaResolution pins that a resolved payload is typed
// from its Resolution exactly as a field is: every pointer shed, never
// nullable, a ref leaf a bare $ref.
func TestResponsePayloadSchemaResolution(t *testing.T) {
	int64Prop := &OpenAPIProperty{Type: typeInteger, Format: formatInt64}
	byteItem := &OpenAPIProperty{Type: typeInteger, Format: formatInt32, Minimum: floatPtr(0)}
	cases := []struct {
		name string
		r    models.TypeShape
		want *OpenAPIProperty
	}{
		{"int64", prim(formatInt64), int64Prop},
		{"**int64", ptrOf(ptrOf(prim(formatInt64))), int64Prop},
		{"[]$User", sliceOf(refTo("User")), &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{Ref: refPath("User")}}},
		{"*$Address", ptrOf(refTo("Address")), &OpenAPIProperty{Ref: refPath("Address")}},
		{"map[string]marshal", mapOf(prim(goTypeString), marshalerOf("Tier", prim(goTypeInt))),
			&OpenAPIProperty{Type: typeObject, AdditionalProperties: &OpenAPIProperty{}}},
		{"kind:integer", kindOnlyLeaf(typeInteger), &OpenAPIProperty{Type: typeInteger}},
		{"[]byte", sliceOf(prim(goTypeByte)), &OpenAPIProperty{Type: typeString, Format: formatByte}},
		{"[N]*byte", arrayOf(ptrOf(prim(goTypeByte))), &OpenAPIProperty{Type: typeArray, Items: byteItem}},
		{"uintptr", prim(goTypeUintptr), &OpenAPIProperty{Type: typeObject}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := responsePayloadSchema(resolvedPayloadOf(c.r))
			assert.Equal(t, c.want, got)
			assert.False(t, got.Nullable)
		})
	}
}

// TestSuccessEnvelopeAnnotatesOnlyBareObject pins that only the bare untyped
// object is annotated: a map or array schema is not the bare-object fallback.
func TestSuccessEnvelopeAnnotatesOnlyBareObject(t *testing.T) {
	data := func(ti *models.TypeInfo) *OpenAPIProperty {
		return successEnvelopeSchema(ti).Properties[propNameData]
	}
	decimal := named("decimal.Decimal")
	assert.Equal(t, responseDataDescription, data(resolvedPayloadOf(prim(goTypeUintptr))).Description)
	assert.Equal(t, responseDataDescription, data(&models.TypeInfo{Shape: &decimal}).Description)
	for name, r := range map[string]models.TypeShape{
		"map":       mapOf(prim(goTypeString), prim(formatInt64)),
		"recursive": mapOf(prim(goTypeString), recursiveLeaf("Tree")),
		"slice":     sliceOf(prim(goTypeString)),
	} {
		assert.Empty(t, data(resolvedPayloadOf(r)).Description, name)
	}
}

// TestReferencedSchemaNamesWalksResponseResolution pins that a response's
// Resolution marks the components its ref leaves name, that a resolved
// well-known payload marks nothing, and that a request is never walked.
func TestReferencedSchemaNamesWalksResponseResolution(t *testing.T) {
	timeShape := named(goTypeTimeTime)
	requestRes := refTo("Hidden")
	routes := []models.Route{
		{Method: "GET", Path: "/addresses", Response: resolvedPayloadOf(mapOf(prim(goTypeString), refTo("Address")))},
		{Method: "GET", Path: "/at", Response: &models.TypeInfo{Name: "Time", Shape: &timeShape, Resolution: &timeShape}},
		{Method: "POST", Path: "/req", Request: &models.TypeInfo{Name: "Req", Resolution: &requestRes}},
	}
	got := referencedSchemaNames(routes, map[string]*models.TypeInfo{})
	assert.Equal(t, map[string]bool{"Address": true}, got)
}

// TestPayloadNamesComponentArms covers each arm of payloadNamesComponent.
func TestPayloadNamesComponentArms(t *testing.T) {
	timeShape := named(goTypeTimeTime)
	items := sliceOf(named("Item"))
	times := sliceOf(ptrOf(named(goTypeTimeTime)))
	bare := models.TypeShape{Kind: models.ShapeSlice}
	cases := []struct {
		name string
		ti   *models.TypeInfo
		want bool
	}{
		{"nameless", &models.TypeInfo{Shape: &items}, false},
		{"jose", &models.TypeInfo{Name: "UUID", JOSE: true, Resolution: &timeShape}, true},
		{"resolved", &models.TypeInfo{Name: "Time", Shape: &timeShape, Resolution: &timeShape}, false},
		{"struct slice", &models.TypeInfo{Name: "Item", Shape: &items}, true},
		{"well-known element", &models.TypeInfo{Name: "Time", Shape: &times}, false},
		{"slice without element", &models.TypeInfo{Name: "Item", Shape: &bare}, true},
		{"registered", &models.TypeInfo{Name: "Item"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, payloadNamesComponent(c.ti))
		})
	}
}

// TestStructSlicePayloadSchema pins a registered struct slice: an array of
// $ref to its element, unless the element is well-known.
func TestStructSlicePayloadSchema(t *testing.T) {
	items := sliceOf(ptrOf(named("Item")))
	got := responsePayloadSchema(&models.TypeInfo{Name: "Item", Shape: &items})
	require.NotNil(t, got.Items)
	assert.Equal(t, refPath("Item"), got.Items.Ref)

	uuids := sliceOf(named(goTypeUUID))
	got = responsePayloadSchema(&models.TypeInfo{Name: "UUID", Shape: &uuids})
	require.NotNil(t, got.Items)
	assert.Empty(t, got.Items.Ref)
	assert.Equal(t, formatUUID, got.Items.Format)
}
