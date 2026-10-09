package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

const (
	mtLevel  = "Level"
	mtFilter = "Filter"
	mtEmpty  = "Empty"
)

// TestTextLeafTakesNoKeywords pins a text Marshaler leaf (#111): a string
// with no validate keyword, nullable behind a pointer, its example kept as a
// string, and an enclosing slice's cardinality kept.
func TestTextLeafTakesNoKeywords(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	level := textLeaf(mtLevel)
	str := &OpenAPIProperty{Type: typeString}

	f := resolved(named(mtLevel), level)
	f.Constraints = map[string]string{"min": "1", "max": "3", "email": "", "oneof": "1 2 3"}
	assert.Equal(t, str, gen.fieldInfoToProperty(f))

	assert.Equal(t, &OpenAPIProperty{Type: typeString, Nullable: true},
		gen.fieldInfoToProperty(resolved(ptrOf(named(mtLevel)), ptrOf(level))))

	f = resolved(sliceOf(named(mtLevel)), sliceOf(level))
	f.Constraints = map[string]string{"min": "1"}
	f.ElementConstraints = map[string]string{"max": "3"}
	assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: str, MinItems: intPtr(1)}, gen.fieldInfoToProperty(f))

	for _, ex := range []string{"2", "high"} {
		f = resolved(named(mtLevel), level)
		f.Example = ex
		assert.Equal(t, &OpenAPIProperty{Type: typeString, Example: ex}, gen.fieldInfoToProperty(f))
	}
}

// TestIsMarshalerLeaf pins which leaves take no validate keyword.
func TestIsMarshalerLeaf(t *testing.T) {
	assert.True(t, isMarshalerLeaf(textLeaf(mtLevel)))
	assert.True(t, isMarshalerLeaf(ptrOf(textLeaf(mtLevel))))
	assert.True(t, isMarshalerLeaf(marshalerOf("M", prim("int"))))
	assert.False(t, isMarshalerLeaf(prim(goTypeString)))
}

// TestMarshalerRequestBody pins a struct Marshaler request: its body is its
// Resolution's schema, and its parameters stay.
func TestMarshalerRequestBody(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	marshaler := models.TypeShape{Kind: models.ShapeMarshaler, Name: "CustomBody"}
	id := models.FieldInfo{Name: "ID", ParamType: "path", ParamName: "id", Shape: prim(goTypeString)}
	route := &models.Route{Method: "POST", Path: "/custom/{id}", HandlerName: "custom",
		Request: &models.TypeInfo{Resolution: &marshaler, Fields: []models.FieldInfo{id}}}
	op := gen.buildOperation(route, map[string]string{})
	require.NotNil(t, op.RequestBody)
	assert.Equal(t, &OpenAPIProperty{}, op.RequestBody.Content[mediaJSON].Schema)
	require.Len(t, op.Parameters, 1)
	assert.Equal(t, "id", op.Parameters[0].Name)

	text := textLeaf("MoneyT")
	op = gen.buildOperation(&models.Route{Method: "POST", Path: "/t", HandlerName: "t",
		Request: &models.TypeInfo{Resolution: &text}}, map[string]string{})
	require.NotNil(t, op.RequestBody)
	assert.Equal(t, &OpenAPIProperty{Type: typeString}, op.RequestBody.Content[mediaJSON].Schema)

	assert.Equal(t, typeObject, gen.buildRequestBody(nil).Content[mediaJSON].Schema.Type)
}

// TestReferencedSchemaNamesMarshalerRequestParams pins addRequestFieldRefs:
// a struct Marshaler request's struct parameters are marked (or their $refs
// dangle), and a request's own name never is.
func TestReferencedSchemaNamesMarshalerRequestParams(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	marshaler := models.TypeShape{Kind: models.ShapeMarshaler, Name: "CmdBody"}
	f := withResolution(&models.FieldInfo{Name: "F", ParamType: "query", ParamName: "f", Shape: named(mtFilter)}, refTo(mtFilter))
	e := withResolution(&models.FieldInfo{Name: "E", ParamType: "query", ParamName: "e", Shape: named(mtEmpty)}, refTo(mtEmpty))
	types := map[string]*models.TypeInfo{
		mtFilter: {Name: mtFilter, Fields: []models.FieldInfo{{Name: "A", ParamType: "query", ParamName: "a", Shape: prim(goTypeString)}}},
		mtEmpty:  {Name: mtEmpty},
	}
	routes := []models.Route{{Method: "POST", Path: "/cmd",
		Request: &models.TypeInfo{Resolution: &marshaler, Fields: []models.FieldInfo{*f, *e}}}}
	got := referencedSchemaNames(routes, types)
	assert.Equal(t, map[string]bool{mtFilter: true, mtEmpty: true}, got)
	schemas := gen.generateSchemasFromTypes(types, got)
	for _, name := range []string{mtFilter, mtEmpty} {
		require.Contains(t, schemas, name)
		assert.Equal(t, typeObject, schemas[name].Type)
	}

	out := map[string]bool{}
	addRequestFieldRefs(out, nil)
	assert.Empty(t, out)

	user := withResolution(&models.FieldInfo{Name: "U", JSONName: "u", Shape: named("User")}, refTo("User"))
	req := &models.TypeInfo{Name: "Req", Fields: []models.FieldInfo{*user}}
	addRequestFieldRefs(out, req)
	viaRegistry := map[string]bool{}
	addFieldSchemaRefs(viaRegistry, map[string]*models.TypeInfo{"Req": req})
	assert.Equal(t, viaRegistry, out)
	assert.NotContains(t, out, "Req")
}

// TestJOSEMarshalerRequestBody pins a jose-tagged struct Marshaler payload:
// it stays application/jose, naming no plaintext component on either side,
// not even SuccessResponse, which is then not emitted.
func TestJOSEMarshalerRequestBody(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	marshaler := models.TypeShape{Kind: models.ShapeMarshaler, Name: "JoseReq"}
	req := &models.TypeInfo{JOSE: true, Resolution: &marshaler}
	rb := gen.buildRequestBody(req)
	assert.Equal(t, joseUndocumentedDescription, rb.Description)
	assert.NotContains(t, rb.Description, refComponentPrefix)
	assert.Equal(t, joseTokenSchema(), rb.Content[mediaJOSE].Schema)

	resp := &models.TypeInfo{JOSE: true, Resolution: &marshaler}
	route := &models.Route{Method: "POST", Path: "/jose", HandlerName: "joseRoute", Request: req, Response: resp}
	op := gen.buildOperation(route, map[string]string{})
	require.NotNil(t, op.RequestBody)
	assert.Contains(t, op.RequestBody.Content, mediaJOSE)
	assert.Equal(t, joseUndocumentedDescription, op.Responses["200"].Description)
	assert.Contains(t, op.Responses["200"].Content, mediaJOSE)
	std := gen.createStandardSchemas([]models.Route{*route})
	assert.NotContains(t, std, schemaSuccessResponse)
	assert.Contains(t, std, schemaJOSEErrorEnvelope)

	untyped := &models.Route{Method: "GET", Path: "/u", HandlerName: "u", Response: &models.TypeInfo{JOSE: true}}
	assert.Contains(t, gen.buildOperation(untyped, map[string]string{}).Responses["200"].Description, refPath(schemaSuccessResponse))
	assert.Contains(t, gen.createStandardSchemas([]models.Route{*untyped}), schemaSuccessResponse)

	assert.Contains(t, joseDescription("X"), "conforms to the X schema")
	assert.Contains(t, joseDescription("X"), refPath("X"))
}
