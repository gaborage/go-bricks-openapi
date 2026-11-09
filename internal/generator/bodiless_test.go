package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestReferencedSchemaNamesSkipsBodilessResponse pins issue 144's guard: a
// bodiless (NoContent) response marks nothing, neither its Name nor the refs
// in its Resolution, so a params-only type its Name also names stays
// unemitted, while a response with a body still marks its component.
func TestReferencedSchemaNamesSkipsBodilessResponse(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	res := refTo("Inner")
	params := &models.TypeInfo{Name: "Params", Fields: []models.FieldInfo{{Name: "ID", ParamType: "path", ParamName: "id", Shape: prim(goTypeString)}}}
	types := map[string]*models.TypeInfo{"Params": params}
	routes := []models.Route{
		{Method: "PUT", Path: "/a", SuccessStatus: 204, Response: &models.TypeInfo{Name: "Params", NoContent: true}},
		{Method: "PUT", Path: "/b", SuccessStatus: 204, Response: &models.TypeInfo{NoContent: true, Resolution: &res}},
		{Method: "GET", Path: "/c", Response: &models.TypeInfo{Name: "Body"}},
	}
	got := referencedSchemaNames(routes, types)
	assert.Equal(t, map[string]bool{"Body": true}, got)
	assert.NotContains(t, gen.generateSchemasFromTypes(types, got), "Params")
}

// TestRouteTypeRegistrySkipsBodilessResponse pins that the route-derived
// fallback registry does not collect a bodiless response's payload, which
// would otherwise be emitted as an orphan component.
func TestRouteTypeRegistrySkipsBodilessResponse(t *testing.T) {
	project := &models.Project{Modules: []models.Module{{Routes: []models.Route{
		{Method: "PUT", Path: "/a", Response: &models.TypeInfo{Name: "Gone", NoContent: true}},
		{Method: "GET", Path: "/b", Response: &models.TypeInfo{Name: "Kept"}},
	}}}}
	got := routeTypeRegistry(project)
	assert.NotContains(t, got, "Gone")
	assert.Contains(t, got, "Kept")
}
