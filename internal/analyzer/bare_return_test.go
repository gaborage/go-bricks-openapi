package analyzer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// bareReturnModSrc is a module whose one route, GET /payload, returns RESULT
// with no request. Every payload type the parity table names is declared or
// imported here: a project struct, two local named non-structs, and the
// stdlib/third-party packages of the well-known and unresolvable rows.
const bareReturnModSrc = `package mod

import (
	"encoding/json"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Address struct {
	Street string ` + "`json:\"street\"`" + `
}

type Cents int64

type Tags []string

var (
	_ json.RawMessage
	_ time.Time
	_ uuid.UUID
	_ decimal.Decimal
)

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/payload", m.get)
}

func (m *Module) get(ctx server.HandlerContext) (RESULT, server.IAPIError) {
	return out, nil
}
`

const bareReturnRouteKey = "GET /payload"

// bareReturnPayloads is every payload T a bare handler return is compared
// against server.Result[T] for: builtins (one behind a pointer), well-known
// types, unresolvable names, local named non-structs, slices, the empty
// interface, a map, and the project-struct controls.
var bareReturnPayloads = []string{
	"int64", "*int64", "string", "bool", "float64", "any",
	"time.Time", "*time.Time", "uuid.UUID", "json.RawMessage",
	"decimal.Decimal", "Missing",
	"Cents", "Tags",
	"[]Address", "[]*Address", "[]string", "[]time.Time", "[]decimal.Decimal",
	"interface{}", "map[string]Cents",
	"Address", "*Address",
}

// analyzeBareReturn analyzes bareReturnModSrc with the handler's first result
// set to result and returns the route's response payload and every warning.
func analyzeBareReturn(t *testing.T, result string) (response *models.TypeInfo, warnings []string) {
	t.Helper()
	a, routes := analyzeSingleModule(t, strings.ReplaceAll(bareReturnModSrc, "RESULT", result))
	route := routeForPath(t, routes, bareReturnRouteKey)
	return route.Response, a.Warnings(t.Context())
}

// TestBareReturnMatchesResultPayload pins that a bare (non-wrapper) handler
// return of type T resolves exactly like server.Result[T]: the same payload
// TypeInfo (name, package, fields, shape) and the same analyzer warnings, so
// every downstream consumer — the generator's payload schema, doctor's
// classification, the --strict gate — treats the two alike.
func TestBareReturnMatchesResultPayload(t *testing.T) {
	for _, payload := range bareReturnPayloads {
		t.Run(payload, func(t *testing.T) {
			bare, bareWarnings := analyzeBareReturn(t, payload)
			wrapped, wrappedWarnings := analyzeBareReturn(t, "server.Result["+payload+"]")
			assert.Equal(t, wrapped, bare, "a bare %s return must resolve like server.Result[%s]", payload, payload)
			assert.Equal(t, wrappedWarnings, bareWarnings, "a bare %s return must warn like server.Result[%s]", payload, payload)
		})
	}
}

// TestBareReturnPayloadShapes pins the absolute resolution of the bare rows the
// generator types differently: a builtin names no component, a well-known type
// keeps its name, a slice describes its element, and an unresolvable name is
// cleared with a warning instead of dangling a $ref.
func TestBareReturnPayloadShapes(t *testing.T) {
	t.Run("builtin_names_no_component", func(t *testing.T) {
		ti, warnings := analyzeBareReturn(t, "*int64")
		require.NotNil(t, ti)
		assert.Empty(t, ti.Name, "a builtin names no component — a $ref would dangle")
		require.NotNil(t, ti.Shape)
		assert.Equal(t, models.TypeShape{Kind: models.ShapePrimitive, Name: "int64"}, PayloadBaseShape(*ti.Shape))
		assert.Empty(t, warnings)
	})
	t.Run("well_known_keeps_name_and_shape", func(t *testing.T) {
		ti, warnings := analyzeBareReturn(t, "time.Time")
		require.NotNil(t, ti)
		assert.Equal(t, "Time", ti.Name)
		require.NotNil(t, ti.Shape)
		assert.Equal(t, models.WellKnownTimeTime, ti.Shape.Name)
		assert.Empty(t, warnings)
	})
	t.Run("empty_interface_is_any_json_value", func(t *testing.T) {
		ti, _ := analyzeBareReturn(t, "interface{}")
		require.NotNil(t, ti, "interface{} documents as {} rather than dropping to the untyped route path")
		assert.Empty(t, ti.Name)
		require.NotNil(t, ti.Shape)
		assert.Equal(t, models.ShapePrimitive, ti.Shape.Kind)
	})
	t.Run("slice_describes_its_element", func(t *testing.T) {
		ti, _ := analyzeBareReturn(t, "[]Address")
		require.NotNil(t, ti, "a bare slice return is an array payload, not the untyped fallback")
		assert.Equal(t, "Address", ti.Name)
		require.NotNil(t, ti.Shape)
		assert.Equal(t, models.ShapeSlice, ti.Shape.Kind)
		assert.NotEmpty(t, ti.Fields, "the element struct is registered")
	})
	t.Run("unresolvable_name_is_cleared_with_warning", func(t *testing.T) {
		ti, warnings := analyzeBareReturn(t, "decimal.Decimal")
		require.NotNil(t, ti)
		assert.Empty(t, ti.Name, "an unresolvable payload name is cleared so no $ref dangles")
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "response type decimal.Decimal resolves to no schema component")
	})
	t.Run("map_stays_untyped", func(t *testing.T) {
		ti, warnings := analyzeBareReturn(t, "map[string]Cents")
		assert.Nil(t, ti, "maps are out of scope, as for server.Result[map[string]T]")
		assert.Empty(t, warnings)
	})
}

// TestBareReturnStructAndNoContentUnchanged pins the bare returns that already
// resolved correctly: a project struct (by value or pointer) is the plain $ref
// path — registered, with no Shape — and server.NoContentResult is the bodyless
// 204 marker.
func TestBareReturnStructAndNoContentUnchanged(t *testing.T) {
	for _, payload := range []string{"Address", "*Address"} {
		t.Run(payload, func(t *testing.T) {
			ti, warnings := analyzeBareReturn(t, payload)
			require.NotNil(t, ti)
			assert.Equal(t, "Address", ti.Name)
			assert.Equal(t, "mod", ti.Package)
			assert.Nil(t, ti.Shape, "a registered project struct sheds its Shape so it is $ref'd")
			require.Len(t, ti.Fields, 1)
			assert.Equal(t, "street", ti.Fields[0].JSONName)
			assert.False(t, ti.NoContent)
			assert.Empty(t, warnings)
		})
	}
	t.Run("no_content_result", func(t *testing.T) {
		ti, warnings := analyzeBareReturn(t, "server.NoContentResult")
		assert.Equal(t, &models.TypeInfo{NoContent: true}, ti, "NoContentResult is a bare marker: no name, no shape")
		assert.Empty(t, warnings)
	})
	t.Run("framework_type_is_no_payload", func(t *testing.T) {
		ti, _ := analyzeBareReturn(t, "error")
		assert.Nil(t, ti, "a framework type in first position is not a payload")
	})
}

// TestBareSliceRequestStaysNil pins that request extraction is untouched by the
// bare-return routing: a []T handler parameter still resolves to no request
// type (the request-body path has no array shape to emit), while the same []T
// as a bare return is an array payload.
func TestBareSliceRequestStaysNil(t *testing.T) {
	src := strings.ReplaceAll(bareReturnModSrc,
		"func (m *Module) get(ctx server.HandlerContext) (RESULT, server.IAPIError)",
		"func (m *Module) get(req []Address, ctx server.HandlerContext) ([]Address, server.IAPIError)")
	_, routes := analyzeSingleModule(t, src)
	route := routeForPath(t, routes, bareReturnRouteKey)
	assert.Nil(t, route.Request, "a slice parameter must keep resolving to nil (#102)")
	require.NotNil(t, route.Response)
	require.NotNil(t, route.Response.Shape)
	assert.Equal(t, models.ShapeSlice, route.Response.Shape.Kind)
}
