package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// nonStructRequestModSrc is a module whose one route, POST /items, takes the
// request parameter list PARAMS and returns RESULT. Every request type the
// non-struct table names is declared or imported here: struct controls (a
// body struct, a params-only struct, an empty struct and a JOSE struct), a
// local named scalar, and the stdlib/third-party packages of the well-known
// and third-party rows.
const nonStructRequestModSrc = `package mod

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

type IDReq struct {
	ID string ` + "`param:\"id\"`" + `
}

type Empty struct{}

type Sealed struct {
	_     struct{} ` + "`jose:\"decrypt=our-signing,verify=partner-verify\"`" + `
	Value string   ` + "`json:\"value\"`" + `
}

type Status string

var (
	_ json.RawMessage
	_ time.Time
	_ uuid.UUID
	_ decimal.Decimal
)

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/items", m.create)
}

func (m *Module) create(PARAMS) (RESULT, server.IAPIError) {
	return out, nil
}
`

const nonStructRequestRouteKey = "POST /items"

// nonStructRequestTypes is one request type per non-struct family the analyzer
// must warn about: builtins (one behind a pointer), the empty interface under
// both spellings, well-known types, a third-party and an undeclared name,
// slices, a fixed-size array, a map and a local named scalar.
var nonStructRequestTypes = []string{
	"string", "*string", "int64", "bool", "any", "interface{}",
	"time.Time", "json.RawMessage", "*json.RawMessage", "uuid.UUID",
	"decimal.Decimal", "Missing",
	"[]Address", "[]string", "*[]Address", "[2]Address", "map[string]string",
	"Status", "*Status",
}

// analyzeNonStructRequest analyzes nonStructRequestModSrc with the given
// handler parameter list and first result, returning the route and every
// analyzer warning.
func analyzeNonStructRequest(t *testing.T, params, result string) (route models.Route, warnings []string) {
	t.Helper()
	src := strings.NewReplacer("PARAMS", params, "RESULT", result).Replace(nonStructRequestModSrc)
	a, routes := analyzeSingleModule(t, src)
	return routeForPath(t, routes, nonStructRequestRouteKey), a.Warnings(t.Context())
}

// wantNonStructRequestWarning is the one warning a non-struct request type T raises.
func wantNonStructRequestWarning(typ string) string {
	return "request type " + typ + " is not a struct: go-bricks binds requests by struct fields and panics at request time (F26, gaborage/go-bricks#1811); no requestBody emitted"
}

// assertUntypedRequest asserts a request resolved to nothing the generator or
// doctor can treat as typed: no TypeInfo at all, or one with no name, no
// fields, no shape and no JOSE marker.
func assertUntypedRequest(t *testing.T, req *models.TypeInfo) {
	t.Helper()
	if req == nil {
		return
	}
	assert.Empty(t, req.Name, "a non-struct request names no component")
	assert.Empty(t, req.Fields, "a non-struct request has no fields to bind")
	assert.Nil(t, req.Shape, "a request never carries a payload Shape")
	assert.False(t, req.JOSE)
}

// TestNonStructRequestWarnsOnce pins #102: go-bricks binds every request by
// struct fields and panics at request time on any other type (F26,
// gaborage/go-bricks#1811). Each non-struct request family therefore raises
// exactly one warning per route, whichever position the request parameter
// takes, and resolves to an untyped request so no requestBody is emitted and
// doctor counts it untyped.
func TestNonStructRequestWarnsOnce(t *testing.T) {
	for _, typ := range nonStructRequestTypes {
		for _, order := range []struct{ name, params string }{
			{"request_first", "req " + typ + ", ctx server.HandlerContext"},
			{"context_first", "ctx server.HandlerContext, req " + typ},
		} {
			t.Run(typ+"/"+order.name, func(t *testing.T) {
				route, warnings := analyzeNonStructRequest(t, order.params, "Address")
				assert.Equal(t, []string{wantNonStructRequestWarning(typ)}, warnings)
				assertUntypedRequest(t, route.Request)
				require.NotNil(t, route.Response, "the response is resolved independently of the request")
				assert.Equal(t, "Address", route.Response.Name)
			})
		}
	}
}

// TestStructRequestDoesNotWarn pins the controls: a struct request — by value,
// behind a pointer, params-only, empty, or JOSE-protected — binds fine in
// go-bricks and keeps resolving exactly as before, with no warning.
func TestStructRequestDoesNotWarn(t *testing.T) {
	cases := []struct {
		typ, name string
		fields    int
		jose      bool
	}{
		{typ: "Address", name: "Address", fields: 1},
		{typ: "*Address", name: "Address", fields: 1},
		{typ: "IDReq", name: "IDReq", fields: 1},
		{typ: "Empty", name: "Empty"},
		{typ: "Sealed", name: "Sealed", fields: 1, jose: true},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			route, warnings := analyzeNonStructRequest(t, "req "+c.typ+", ctx server.HandlerContext", "Address")
			assert.Empty(t, warnings)
			require.NotNil(t, route.Request)
			assert.Equal(t, c.name, route.Request.Name)
			assert.Len(t, route.Request.Fields, c.fields)
			assert.Equal(t, c.jose, route.Request.JOSE)
		})
	}
}

// TestNonStructRequestWithUnresolvedResponseStillWarns pins that a handler
// whose request is a type literal ([]T, map, interface{}) — which resolves to
// no request TypeInfo — is still recognised as the handler when its response
// resolves to nothing either, so the warning fires and the handler's success
// and error statuses are still read.
func TestNonStructRequestWithUnresolvedResponseStillWarns(t *testing.T) {
	route, warnings := analyzeNonStructRequest(t, "req []Address, ctx server.HandlerContext", "map[string]chan int")
	assert.Equal(t, []string{wantNonStructRequestWarning("[]Address")}, warnings)
	assert.Nil(t, route.Request)
	assert.Nil(t, route.Response)
}

// TestNonStructRequestLeavesResponseWarningsAlone pins that only the request
// side changed: a local named non-struct RESPONSE resolves like a field of its
// type with no warning of its own (#110), an unresolvable response keeps its
// own warning beside the request's, and a builtin response stays typed with no
// warning.
func TestNonStructRequestLeavesResponseWarningsAlone(t *testing.T) {
	t.Run("named_non_struct_response_resolves", func(t *testing.T) {
		_, warnings := analyzeNonStructRequest(t, "req string, ctx server.HandlerContext", "Status")
		assert.Equal(t, []string{wantNonStructRequestWarning("string")}, warnings)
	})
	t.Run("unresolvable_response", func(t *testing.T) {
		_, warnings := analyzeNonStructRequest(t, "req string, ctx server.HandlerContext", "server.Result[decimal.Decimal]")
		assert.Equal(t, []string{
			wantNonStructRequestWarning("string"),
			fmt.Sprintf(unresolvablePayloadWarning, "decimal.Decimal"),
		}, warnings)
	})
	t.Run("builtin_response", func(t *testing.T) {
		route, warnings := analyzeNonStructRequest(t, "req string, ctx server.HandlerContext", "server.Result[int64]")
		assert.Equal(t, []string{wantNonStructRequestWarning("string")}, warnings)
		require.NotNil(t, route.Response)
		require.NotNil(t, route.Response.Shape)
		assert.Equal(t, models.TypeShape{Kind: models.ShapePrimitive, Name: "int64"}, *route.Response.Shape)
	})
}

// TestNoRequestParameterDoesNotWarn pins that a handler taking only its
// context has no request at all, so nothing is warned about.
func TestNoRequestParameterDoesNotWarn(t *testing.T) {
	route, warnings := analyzeNonStructRequest(t, "ctx server.HandlerContext", "Address")
	assert.Empty(t, warnings)
	assert.Nil(t, route.Request)
}

// TestNonStructRequestInSiblingFileWarnsOnce pins that the request screen also
// runs when the handler is declared in another file of the module's package
// than RegisterRoutes — the package-wide fallback search.
func TestNonStructRequestInSiblingFileWarnsOnce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0600))
	sub := filepath.Join(dir, "mod")
	require.NoError(t, os.MkdirAll(sub, 0750))
	module, handler, ok := strings.Cut(nonStructRequestModSrc, "func (m *Module) create(")
	require.True(t, ok)
	require.NoError(t, os.WriteFile(filepath.Join(sub, "module.go"), []byte(module), 0600))
	handlerSrc := "package mod\n\nimport \"github.com/gaborage/go-bricks/server\"\n\nfunc (m *Module) create(" +
		strings.NewReplacer("PARAMS", "req *string, ctx server.HandlerContext", "RESULT", "Address").Replace(handler)
	require.NoError(t, os.WriteFile(filepath.Join(sub, "handlers.go"), []byte(handlerSrc), 0600))

	a := New(dir)
	project, err := a.AnalyzeProject()
	require.NoError(t, err)
	require.Len(t, project.Modules, 1)
	route := routeForPath(t, project.Modules[0].Routes, nonStructRequestRouteKey)
	assert.Equal(t, []string{wantNonStructRequestWarning("*string")}, a.Warnings(t.Context()))
	assertUntypedRequest(t, route.Request)
	require.NotNil(t, route.Response)
	assert.Equal(t, "Address", route.Response.Name)
}

// TestExtractRequestTypeNilParams pins that a missing parameter list is no
// request: no TypeInfo and no type text, so nothing is screened.
func TestExtractRequestTypeNilParams(t *testing.T) {
	ti, typeText := New(t.TempDir()).extractRequestType(nil, "mod", nil)
	assert.Nil(t, ti)
	assert.Empty(t, typeText)
}
