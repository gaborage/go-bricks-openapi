package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestRouteMetaSchema pins the success envelope's meta (issue 69): the
// framework block alone without a meta type, and allOf the unchanged block
// and the meta type's $ref with one.
func TestRouteMetaSchema(t *testing.T) {
	assert.Equal(t, metaEnvelopeSchema(), routeMetaSchema(nil))
	assert.Equal(t, metaEnvelopeSchema(), successEnvelopeSchema(nil, nil).Properties[propNameMeta])

	meta := successEnvelopeSchema(nil, &models.TypeInfo{Name: "PageMeta"}).Properties[propNameMeta]
	require.Len(t, meta.AllOf, 2)
	assert.Empty(t, meta.Type, "allOf carries no sibling keywords")
	assert.Nil(t, meta.Properties)
	assert.Equal(t, metaEnvelopeSchema(), meta.AllOf[0])
	assert.Equal(t, &OpenAPIProperty{Ref: refPath("PageMeta")}, meta.AllOf[1])
}

// TestReferencedSchemaNamesMarksRouteMeta verifies an applied meta type is
// referenced, so a field-less one is still emitted as an object, and that a
// route-derived registry (no project.Types) collects it.
func TestReferencedSchemaNamesMarksRouteMeta(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	empty := &models.TypeInfo{Name: "EmptyMeta"}
	routes := []models.Route{{Method: "GET", Path: "/a", Meta: empty}}
	got := referencedSchemaNames(routes, map[string]*models.TypeInfo{"EmptyMeta": empty})
	assert.Equal(t, map[string]bool{"EmptyMeta": true}, got)
	assert.Equal(t, &OpenAPISchema{Type: typeObject}, gen.generateSchemasFromTypes(map[string]*models.TypeInfo{"EmptyMeta": empty}, got)["EmptyMeta"])

	project := &models.Project{Modules: []models.Module{{Routes: routes}}}
	assert.Contains(t, routeTypeRegistry(project), "EmptyMeta")
}

// TestBuildResponsesRouteMeta verifies only the inline JSON envelope carries
// the route's meta type.
func TestBuildResponsesRouteMeta(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	route := &models.Route{Method: "GET", Path: "/a", Response: &models.TypeInfo{Name: "Item"}, Meta: &models.TypeInfo{Name: "PageMeta"}}
	schema := gen.buildResponses(route)["200"].Content[mediaJSON].Schema
	require.Len(t, schema.Properties[propNameMeta].AllOf, 2)
}
