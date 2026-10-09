package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestHandlerInSiblingFileResolvesInItsOwnFile pins seam B of the file
// context fix: a handler found in another file of the package than the one
// holding RegisterRoutes is populated with ITS OWN file's path, so a named
// type declared in the walked file (Tags, in module.go) resolves. Before, the
// handler's AST was paired with the walked file's path, and the resolver,
// which skips the file whose path it is given, never saw module.go.
func TestHandlerInSiblingFileResolvesInItsOwnFile(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): resolveModuleHead + `
type Tags []string

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/resp", m.get)
}
`,
		filepath.Join("mod", "handlers.go"): `package mod

import "github.com/gaborage/go-bricks/server"

type Resp struct {
	T Tags ` + "`json:\"t\"`" + `
}

func (m *Module) get(ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) {
	return server.Result[Resp]{}, nil
}
`,
	})
	assert.Empty(t, a.Warnings(t.Context()))
	assert.Equal(t, "[]string", renderShape(fieldByJSON(t, a.typeRegistry["Resp"], "t").ResolvedShape()))
}

// seamAModule is a package whose Module struct (and the named types Tags and
// Cents) live in module.go while RegisterRoutes and its handlers live in
// routes.go.
func seamAModule() map[string]string {
	return map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): resolveModuleHead + `
type Tags []string

type Cents int64
`,
		filepath.Join("mod", "routes.go"): `package mod

import "github.com/gaborage/go-bricks/server"

type Resp struct {
	T Tags ` + "`json:\"t\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/resp", m.get)
	server.GET(hr, r, "/cents", m.cents)
}

func (m *Module) get(ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) {
	return server.Result[Resp]{}, nil
}

func (m *Module) cents(ctx server.HandlerContext) (server.Result[Cents], server.IAPIError) {
	return server.Result[Cents]{}, nil
}
`,
	}
}

// TestHandlerInRouteFileResolvesModuleFileTypes pins seam A of the file
// context fix: a RegisterRoutes kept outside the Module-struct file is walked
// with its own file's path, so the handlers beside it resolve named types
// declared in the Module-struct file, both as a field (Resp.T) and as a
// payload (Result[Cents]).
func TestHandlerInRouteFileResolvesModuleFileTypes(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, seamAModule())
	assert.Empty(t, a.Warnings(t.Context()))
	assert.Equal(t, "[]string", renderShape(fieldByJSON(t, a.typeRegistry["Resp"], "t").ResolvedShape()))
	cents := routeByPath(t, routes, "/cents").Response
	require.NotNil(t, cents)
	require.NotNil(t, cents.Resolution)
	assert.Equal(t, goTypeInt64, renderShape(*cents.Resolution))
	assert.Empty(t, cents.Name)
}

// analyzeProjectRoutes writes files (project-relative path -> source) into a
// fresh project root, runs one analysis and returns the analyzer and the
// flattened routes.
func analyzeProjectRoutes(t *testing.T, files map[string]string) (*ProjectAnalyzer, []models.Route) {
	t.Helper()
	dir := t.TempDir()
	for rel, src := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o600))
	}
	a := New(dir)
	project, err := a.AnalyzeProject()
	require.NoError(t, err)
	var routes []models.Route
	for i := range project.Modules {
		routes = append(routes, project.Modules[i].Routes...)
	}
	return a, routes
}

// routeByPath returns the route registered at path.
func routeByPath(t *testing.T, routes []models.Route, path string) *models.Route {
	t.Helper()
	for i := range routes {
		if routes[i].Path == path {
			return &routes[i]
		}
	}
	require.Failf(t, "route not found", "no route at %s", path)
	return nil
}

// payloadRowsTypes declares every named type the payload rows use: the
// resolved rows (P), the fallback rows (F) and the unmodelled ones (U).
const payloadRowsTypes = `package mod

import (
	"encoding/json"
	"time"
)

type Cents int64
type Chain Cents
type Alias = int64
type Dur time.Duration
type Flag byte
type Status string
type Ratio float64
type Enabled bool
type Blob []byte
type Raw json.RawMessage
type Tags []string
type UserList []User
type Stamp = time.Time
type RawA = json.RawMessage
type AnyD any
type PC *int64
type Pair [2]int

type User struct {
	ID int64 ` + "`json:\"id\"`" + `
}

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

type Tier int

func (t Tier) MarshalText() ([]byte, error) { return nil, nil }

type PTier int

func (t *PTier) MarshalText() ([]byte, error) { return nil, nil }

type FlagV byte

func (f FlagV) MarshalJSON() ([]byte, error) { return nil, nil }

type Tree map[string]Tree
type Addr uintptr
type StampD time.Time
type Cx complex128
type Fn func()
type Ch chan int

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}
`

// payloadRow is one handler's first result and what its response must carry.
// res is the Resolution in renderShape notation, "" for a nil Resolution;
// shape, when set, is the Shape in that notation; warn is the payload's one
// warning, "" for none; nilResp expects no response at all.
type payloadRow struct {
	result  string
	res     string
	shape   string
	name    string
	warn    string
	nilResp bool
}

const (
	rowUsers      = "[]$User"
	rowMapInt64   = "map[string]int64"
	rowMarshalTxt = "MarshalText"
	rowTier       = "Tier"
	rowDecimal    = "decimal.Decimal"
)

// namedNonStructWarning is the (A) text a payload whose root resolves to
// nothing keeps (screenUnresolvedPayload).
func namedNonStructWarning(name string) string {
	return "request/response type " + name + " is a named non-struct type — emitting an untyped schema " +
		"(annotate or restructure it as a struct for a typed spec)"
}

func payloadRows() []payloadRow {
	marshal := func(written, name, method string) string {
		return fmt.Sprintf(marshalerPayloadWarning, written, name, method)
	}
	unresolvable := fmt.Sprintf(unresolvablePayloadWarning, rowDecimal)
	return []payloadRow{
		// P1–P10: named non-struct payloads, local and qualified.
		{result: "server.Result[Cents]", res: goTypeInt64},
		{result: "server.Result[*Cents]", res: "*int64"},
		{result: "server.Result[Chain]", res: goTypeInt64},
		{result: "server.Result[Alias]", res: goTypeInt64},
		{result: "server.Result[Dur]", res: goTypeInt64},
		{result: "server.Result[PC]", res: "*int64"},
		{result: "server.Result[*PC]", res: "**int64"},
		{result: "server.ResultWithMeta[Cents]", res: goTypeInt64},
		{result: "Cents", res: goTypeInt64},
		{result: "server.Result[Flag]", res: goTypeByte},
		{result: "server.Result[Status]", res: goTypeString},
		{result: "server.Result[Ratio]", res: "float64"},
		{result: "server.Result[Enabled]", res: "bool"},
		{result: "server.Result[[]Cents]", res: "[]int64"},
		{result: "server.Result[[]*Cents]", res: "[]*int64"},
		{result: "server.Result[[]Enabled]", res: "[]bool"},
		{result: "server.Result[[2]Cents]", res: "[N]int64"},
		{result: "server.Result[Blob]", res: "[]byte"},
		{result: "server.Result[[]Flag]", res: "[]byte"},
		{result: "server.Result[Raw]", res: "[]byte"},
		{result: "server.Result[*[]byte]", res: "*[]byte"},
		{result: "server.Result[Tags]", res: "[]string"},
		{result: "server.Result[*Tags]", res: "*[]string"},
		{result: "server.Result[b.Tags]", res: "[]string"},
		{result: "server.Result[[]Tags]", res: "[][]string"},
		{result: "server.Result[[][]string]", res: "[][]string"},
		{result: "server.Result[UserList]", res: rowUsers},
		{result: "server.Result[*UserList]", res: "*" + rowUsers},
		{result: "server.ResultWithMeta[UserList]", res: rowUsers},
		{result: "UserList", res: rowUsers},
		{result: "server.Result[Stamp]", res: "time.Time"},
		{result: "server.Result[RawA]", res: "json.RawMessage"},
		{result: "server.Result[AnyD]", res: "any"},
		{result: "server.Result[Pair]", res: "[N]int"},
		{result: "server.Result[b.Cents]", res: goTypeInt64},
		// P11–P12: unnamed composites.
		{result: "server.Result[map[string]int64]", res: rowMapInt64},
		{result: "server.ResultWithMeta[map[string]int64]", res: rowMapInt64},
		{result: "map[string]int64", res: rowMapInt64},
		{result: "server.Result[map[string]Cents]", res: rowMapInt64},
		{result: "server.Result[map[string][]Cents]", res: "map[string][]int64"},
		{result: "server.Result[[]map[string]int64]", res: "[]map[string]int64"},
		{result: "server.Result[[][]Cents]", res: "[][]int64"},
		{result: "server.Result[*[]string]", res: "*[]string"},
		{result: "server.Result[*interface{}]", res: "*interface{}"},
		{result: "server.Result[map[string]Address]", res: "map[string]$Address"},
		// P14: pointer and array elements of a byte kind are never base64.
		{result: "server.Result[[]*byte]", res: "[]*byte"},
		{result: "server.Result[[]*Flag]", res: "[]*byte"},
		{result: "server.Result[[4]Flag]", res: "[N]byte"},
		{result: "server.Result[*[4]byte]", res: "*[N]byte"},
		// F1–F3: Marshaler types and recursion.
		{result: "server.Result[Tier]", res: "marshal:Tier(int)", warn: marshal(rowTier, rowTier, rowMarshalTxt)},
		{result: "server.Result[*Tier]", res: "*marshal:Tier(int)", warn: marshal("*Tier", rowTier, rowMarshalTxt)},
		{result: "server.Result[PTier]", res: "marshal:PTier(int)", warn: marshal("PTier", "PTier", rowMarshalTxt)},
		{result: "server.Result[b.Tier]", res: "marshal:Tier(int)", warn: marshal("b.Tier", "b.Tier", rowMarshalTxt)},
		{result: "server.Result[[]Tier]", res: "[]marshal:Tier(int)", warn: marshal("[]Tier", rowTier, rowMarshalTxt)},
		{result: "server.Result[[]FlagV]", res: "[]marshal:FlagV(byte)", warn: marshal("[]FlagV", "FlagV", "MarshalJSON")},
		{result: "server.Result[map[string]Tier]", res: "map[string]marshal:Tier(int)", warn: marshal("map[string]Tier", rowTier, rowMarshalTxt)},
		{result: "server.Result[Tree]", res: "map[string]cycle:Tree", warn: fmt.Sprintf(recursivePayloadWarning, "Tree", "Tree")},
		// F4–F5: uintptr.
		{result: "server.Result[Addr]", res: goTypeUintptr, warn: fmt.Sprintf(uintptrPayloadWarning, "Addr")},
		{result: "server.Result[uintptr]", res: goTypeUintptr, warn: fmt.Sprintf(uintptrPayloadWarning, goTypeUintptr)},
		{result: "server.Result[map[string]uintptr]", res: "map[string]uintptr", warn: fmt.Sprintf(uintptrPayloadWarning, "map[string]uintptr")},
		{result: "server.Result[[]uintptr]", res: "[]uintptr", warn: fmt.Sprintf(uintptrPayloadWarning, "[]uintptr")},
		// F6–F7: unresolvable names; only a root falls to the screens.
		{result: "server.Result[decimal.Decimal]", warn: unresolvable},
		{result: "server.Result[t.Time]", warn: fmt.Sprintf(unresolvablePayloadWarning, "t.Time")},
		{result: "server.Result[*Missing]", warn: fmt.Sprintf(unresolvablePayloadWarning, "Missing")},
		{result: "server.Result[[]decimal.Decimal]", res: "[]decimal.Decimal", warn: unresolvable},
		{result: "server.Result[map[string]decimal.Decimal]", res: "map[string]decimal.Decimal", warn: unresolvable},
		// F8–F10: a defined type over a well-known struct, untyped builtins.
		{result: "server.Result[StampD]", warn: namedNonStructWarning("StampD")},
		{result: "server.Result[[]StampD]", res: "[]StampD",
			warn: fmt.Sprintf(definedOverQualifiedPayloadWarning, "[]StampD", "StampD", "time.Time")},
		{result: "server.Result[Cx]", res: "complex128", warn: fmt.Sprintf(untypedBuiltinPayloadWarning, "Cx", "complex128")},
		{result: "server.Result[complex128]", res: "complex128",
			warn: fmt.Sprintf(untypedBuiltinPayloadWarning, "complex128", "complex128")},
		{result: "server.Result[[]error]", res: "[]error", warn: fmt.Sprintf(untypedBuiltinPayloadWarning, "[]error", "error")},
		// U1: a named func or chan root keeps (A).
		{result: "server.Result[Fn]", warn: namedNonStructWarning("Fn")},
		{result: "server.Result[[]Fn]", warn: namedNonStructWarning("Fn")},
		{result: "server.Result[[]Ch]", warn: namedNonStructWarning("Ch")},
		// U2: a literal unmodelled leaf keeps a nil response.
		{result: "server.Result[struct{ A int }]", nilResp: true},
		{result: "server.Result[[]struct{ A int }]", nilResp: true},
		{result: "server.Result[map[string]func()]", nilResp: true},
		{result: "server.Result[map[string]chan int]", nilResp: true},
		{result: "server.Result[Page[User]]", nilResp: true},
		// U3: a written **T root stays nil (#120).
		{result: "server.Result[**int64]", nilResp: true},
		{result: "server.Result[**Address]", nilResp: true},
		{result: "server.Result[**[]string]", nilResp: true},
		// U4: a nameless composite over a named func or chan type is typed
		// from its Shape, silently, like the field of its type.
		{result: "server.Result[map[string]Fn]", shape: "map[string]Fn"},
		{result: "server.Result[map[string]Ch]", shape: "map[string]Ch"},
		{result: "server.Result[*[]Fn]", shape: "*[]Fn"},
		{result: "server.Result[[][]Fn]", shape: "[][]Fn"},
		// C1: struct and well-known payloads are unchanged.
		{result: "server.Result[time.Time]", res: "time.Time", name: "Time"},
		{result: "server.Result[[]*time.Time]", res: "[]*time.Time", name: "Time"},
	}
}

// payloadRowsModule renders one route /p<i> per row, each handler returning
// the row's result.
func payloadRowsModule(rows []payloadRow) string {
	var reg, handlers strings.Builder
	for i, r := range rows {
		fmt.Fprintf(&reg, "\tserver.GET(hr, r, \"/p%d\", m.h%d)\n", i, i)
		fmt.Fprintf(&handlers, "func (m *Module) h%d(ctx server.HandlerContext) (%s, server.IAPIError) {\n\tvar v %s\n\treturn v, nil\n}\n\n",
			i, r.result, r.result)
	}
	return `package mod

import (
	"time"
	t "time"

	"github.com/example/app/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

var _ = time.Now
var _ t.Time

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
` + reg.String() + "}\n\n" + handlers.String()
}

// payloadWarnings returns the warnings a payload raises.
func payloadWarnings(a *ProjectAnalyzer) []string {
	var out []string
	for _, w := range a.warnings {
		if strings.HasPrefix(w, "response type ") || strings.HasPrefix(w, "request/response type ") {
			out = append(out, w)
		}
	}
	return out
}

// TestPayloadResolutionRows pins every payload row of #110: the Resolution a
// payload carries, the name it keeps, and the one warning it raises, if any.
func TestPayloadResolutionRows(t *testing.T) {
	rows := payloadRows()
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("b", "b.go"):         "package b\n\ntype Cents int64\n\ntype Tags []string\n\ntype Tier int\n\nfunc (t Tier) MarshalText() ([]byte, error) { return nil, nil }\n",
		filepath.Join("mod", "types.go"):   payloadRowsTypes,
		filepath.Join("mod", "width_a.go"): "//go:build linux\n\npackage mod\n\ntype Width int32\n",
		filepath.Join("mod", "width_b.go"): "//go:build !linux\n\npackage mod\n\ntype Width int64\n",
		filepath.Join("mod", "module.go"):  payloadRowsModule(append(rows, payloadRow{result: "server.Result[Width]"})),
	})
	var wantWarnings []string
	for i, r := range rows {
		t.Run(r.result, func(t *testing.T) {
			ti := routeByPath(t, routes, fmt.Sprintf("/p%d", i)).Response
			if r.nilResp {
				assert.Nil(t, ti)
				return
			}
			require.NotNil(t, ti)
			assert.Equal(t, r.name, ti.Name)
			if r.res == "" {
				assert.Nil(t, ti.Resolution)
			} else if assert.NotNil(t, ti.Resolution) {
				assert.Equal(t, r.res, renderShape(*ti.Resolution))
			}
			if r.shape != "" && assert.NotNil(t, ti.Shape) {
				assert.Equal(t, r.shape, renderShape(*ti.Shape))
			}
		})
		if r.warn != "" {
			wantWarnings = append(wantWarnings, r.warn)
		}
	}
	width := routeByPath(t, routes, fmt.Sprintf("/p%d", len(rows))).Response
	require.NotNil(t, width)
	require.NotNil(t, width.Resolution)
	assert.Equal(t, "kind:"+kindInteger, renderShape(*width.Resolution))

	assert.ElementsMatch(t, wantWarnings, payloadWarnings(a))
	assert.Contains(t, a.typeRegistry, "User")
	assert.Contains(t, a.typeRegistry, "Address")
	for _, name := range []string{"UserList", "Tags", "Cents"} {
		assert.NotContains(t, a.typeRegistry, name)
	}
}

// Hand-built shape builders for the payload unit tests (the generator's
// builders live in another package).
func tsPrim(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePrimitive, Name: name}
}

func tsNamed(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
}

func tsPtr(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePointer, Elem: &s}
}

func tsSlice(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeSlice, Elem: &s}
}

func tsMap(v models.TypeShape) models.TypeShape {
	k := tsPrim(goTypeString)
	return models.TypeShape{Kind: models.ShapeMap, Key: &k, Elem: &v}
}

// TestPayloadResolvesRejects pins payloadResolves on hand-built resolutions:
// only a non-well-known named ROOT (every pointer shed) or an unmodelled leaf
// rejects; a non-well-known named leaf below a container still resolves.
func TestPayloadResolvesRejects(t *testing.T) {
	cases := []struct {
		s    models.TypeShape
		want bool
	}{
		{tsNamed("Cents"), false},
		{tsPtr(tsNamed("Cents")), false},
		{tsPtr(tsNamed(models.WellKnownTimeTime)), true},
		{tsSlice(models.TypeShape{Kind: models.ShapeUnknown}), false},
		{tsSlice(models.TypeShape{}), false},
		{models.TypeShape{Kind: models.ShapePointer}, true},
		{tsSlice(tsNamed(rowDecimal)), true},
		{tsPtr(tsPtr(tsPrim(goTypeInt64))), true},
		{models.TypeShape{Kind: models.ShapeRef, Name: "User"}, true},
	}
	for _, c := range cases {
		t.Run(renderShape(c.s), func(t *testing.T) {
			s := c.s
			assert.Equal(t, c.want, payloadResolves(&s))
		})
	}
}

// TestRequestlessCompositePayloadKeepsStatuses pins Impact 12: a handler with
// no request parameter whose payload is newly carried (a map, wrapped or
// bare, or a U4 map over a named func type) is found as the handler, so its
// success status and inferred error statuses are kept; a literal unmodelled
// payload (struct{...}) still is not, and loses them.
func TestRequestlessCompositePayloadKeepsStatuses(t *testing.T) {
	_, routes := analyzeSingleModule(t, resolveModuleHead+`
type Fn func()

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/map-created", m.create)
	server.GET(hr, r, "/bare-map", m.bareMap)
	server.GET(hr, r, "/fn-map", m.fnMap)
	server.POST(hr, r, "/anon", m.anon)
}

func (m *Module) create(ctx server.HandlerContext) (server.Result[map[string]int64], server.IAPIError) {
	if ctx.Echo == nil {
		return server.Result[map[string]int64]{}, server.NewNotFoundError("x")
	}
	return server.Created(map[string]int64{}), nil
}

func (m *Module) bareMap(ctx server.HandlerContext) (map[string]int64, server.IAPIError) {
	return nil, server.NewConflictError("x")
}

func (m *Module) fnMap(ctx server.HandlerContext) (server.Result[map[string]Fn], server.IAPIError) {
	return server.Result[map[string]Fn]{}, server.NewNotFoundError("x")
}

func (m *Module) anon(ctx server.HandlerContext) (server.Result[struct{ A int }], server.IAPIError) {
	if ctx.Echo == nil {
		return server.Result[struct{ A int }]{}, server.NewNotFoundError("x")
	}
	return server.Created(struct{ A int }{}), nil
}
`)
	created := routeByPath(t, routes, "/map-created")
	assert.Equal(t, 201, created.SuccessStatus)
	assert.Contains(t, created.ErrorStatuses, 404)

	assert.Contains(t, routeByPath(t, routes, "/bare-map").ErrorStatuses, 409)

	fnMap := routeByPath(t, routes, "/fn-map")
	assert.NotNil(t, fnMap.Response)
	assert.Contains(t, fnMap.ErrorStatuses, 404)

	anon := routeByPath(t, routes, "/anon")
	assert.Nil(t, anon.Response)
	assert.Zero(t, anon.SuccessStatus)
	assert.Empty(t, anon.ErrorStatuses)
}

// TestCompositePayloadTypeInfo pins extraction of composite payloads through
// payloadTypeInfo: maps, nested slices and pointers to a container are carried
// nameless with their Shape; a literal unmodelled leaf and a written **T root
// are not; a pointer element is kept.
func TestCompositePayloadTypeInfo(t *testing.T) {
	a := New("")
	extract := func(src string) *models.TypeInfo {
		return a.payloadTypeInfo(mustParse(t, src), "mod", map[string]struct{}{})
	}
	for _, src := range []string{"map[string]func()", "**int64", "**Address", "[]struct{A int}", "[]server.IAPIError"} {
		assert.Nil(t, extract(src), src)
	}
	for src, want := range map[string]string{
		"map[string]Fn": "map[string]Fn",
		"*[]string":     "*[]string",
		"*interface{}":  "*interface{}",
		"*[4]byte":      "*[N]byte",
		"*map[string]A": "*map[string]A",
		"[]*[]string":   "[]*[]string",
		"[]**int64":     "[]**int64",
	} {
		ti := extract(src)
		if assert.NotNil(t, ti, src) {
			assert.Empty(t, ti.Name, src)
			require.NotNil(t, ti.Shape, src)
			assert.Equal(t, want, renderShape(*ti.Shape), src)
		}
	}
	item := extract("[]*Item")
	require.NotNil(t, item)
	assert.Equal(t, "Item", item.Name)
	require.NotNil(t, item.Shape)
	assert.Equal(t, "[]*Item", renderShape(*item.Shape))

	assert.False(t, isCompositePointee(mustParse(t, "*int64")))
}

// TestIsTypedPayload covers every arm of IsTypedPayload and isFallbackLeaf.
func TestIsTypedPayload(t *testing.T) {
	resolved := func(r models.TypeShape) *models.TypeInfo {
		s := r
		return &models.TypeInfo{Shape: &s, Resolution: &r}
	}
	statusSlice := tsSlice(tsNamed("Status"))
	int64Shape := tsPrim(goTypeInt64)
	timeShape := tsNamed(models.WellKnownTimeTime)
	cases := []struct {
		name string
		ti   *models.TypeInfo
		want bool
	}{
		{"nil", nil, false},
		{"registered", &models.TypeInfo{Name: "Item"}, true},
		{"well-known name", &models.TypeInfo{Name: "Time", Shape: &timeShape}, true},
		{"ref", resolved(tsSlice(models.TypeShape{Kind: models.ShapeRef, Name: "User"})), true},
		{"kind-only", resolved(models.TypeShape{Kind: models.ShapeKindOnly, Name: kindInteger}), true},
		{"marshaler", resolved(models.TypeShape{Kind: models.ShapeMarshaler, Name: rowTier}), false},
		{"text", resolved(tsSlice(models.TypeShape{Kind: models.ShapeText, Name: rowTier})), true},
		{"recursive", resolved(tsMap(models.TypeShape{Kind: models.ShapeRecursive, Name: "Tree"})), false},
		{"uintptr", resolved(tsPrim(goTypeUintptr)), false},
		{"complex128", resolved(tsPrim(goTypeComplex128)), false},
		{"non-well-known named", resolved(tsMap(tsNamed(rowDecimal))), false},
		{"unknown", resolved(tsSlice(models.TypeShape{Kind: models.ShapeUnknown})), false},
		{"well-known leaf", resolved(tsSlice(tsNamed(models.WellKnownTimeTime))), true},
		{"named-scalar slice element", &models.TypeInfo{Shape: &statusSlice}, false},
		{"nameless builtin", &models.TypeInfo{Shape: &int64Shape}, true},
		{"neither", &models.TypeInfo{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsTypedPayload(c.ti))
		})
	}
}

// TestWarnPayloadEveryKind drives warnPayload with each fallback kind, and a
// uintptr resolution that outranks a noted Marshaler fallback.
func TestWarnPayloadEveryKind(t *testing.T) {
	const written = "map[string]X"
	plain := tsMap(tsPrim(goTypeInt64))
	cases := []struct {
		fb   fieldFallback
		want string
	}{
		{fieldFallback{kind: fallbackMarshaler, typeName: "X", detail: rowMarshalTxt}, fmt.Sprintf(marshalerPayloadWarning, written, "X", rowMarshalTxt)},
		{fieldFallback{kind: fallbackRecursive, typeName: "X"}, fmt.Sprintf(recursivePayloadWarning, written, "X")},
		{fieldFallback{kind: fallbackDepthCap, typeName: "X"}, fmt.Sprintf(depthCapPayloadWarning, written, maxNamedResolutionDepth, "X")},
		{fieldFallback{kind: fallbackUnresolvable, typeName: rowDecimal}, fmt.Sprintf(unresolvablePayloadWarning, rowDecimal)},
		{fieldFallback{kind: fallbackDefinedOverQualified, typeName: "X", detail: "q.T"}, fmt.Sprintf(definedOverQualifiedPayloadWarning, written, "X", "q.T")},
		{fieldFallback{kind: fallbackDeclsDisagree, typeName: "X"}, fmt.Sprintf(declsDisagreePayloadWarning, written, "X")},
		{fieldFallback{kind: fallbackUntypedBuiltin, typeName: goTypeComplex128, detail: goTypeComplex128},
			fmt.Sprintf(untypedBuiltinPayloadWarning, written, goTypeComplex128)},
		{fieldFallback{kind: fallbackMarshalerPromoted, typeName: "X", detail: rowMarshalTxt, via: "Y"},
			fmt.Sprintf(marshalerPromotedPayloadWarning, written, "X", rowMarshalTxt, "Y")},
	}
	for _, c := range cases {
		a := New("")
		assert.True(t, a.warnPayload(written, &plain, c.fb))
		assert.Equal(t, []string{c.want}, a.warnings)
	}

	a := New("")
	assert.False(t, a.warnPayload(written, &plain, fieldFallback{}))
	assert.Empty(t, a.warnings)

	ptr := tsMap(tsPrim(goTypeUintptr))
	assert.True(t, a.warnPayload(written, &ptr, fieldFallback{kind: fallbackMarshaler, typeName: "X", detail: rowMarshalTxt}))
	assert.Equal(t, []string{fmt.Sprintf(uintptrPayloadWarning, written)}, a.warnings)
}

// TestPayloadDepthCapAndDeclsDisagree drives the two fallback kinds that need
// a composite root to reach warnPayload end to end.
func TestPayloadDepthCapAndDeclsDisagree(t *testing.T) {
	var chain strings.Builder
	for i := range 10 {
		fmt.Fprintf(&chain, "type D%d []D%d\n", i, i+1)
	}
	chain.WriteString("type D10 []int\n")
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("mod", "chain.go"):  "package mod\n\n" + chain.String(),
		filepath.Join("mod", "w_a.go"):    "package mod\n\ntype W []string\n",
		filepath.Join("mod", "w_b.go"):    "package mod\n\ntype W []int\n",
		filepath.Join("mod", "module.go"): payloadRowsModule([]payloadRow{{result: "server.Result[[]D0]"}, {result: "server.Result[[]W]"}}),
	})
	require.NotNil(t, routeByPath(t, routes, "/p0").Response)
	assert.ElementsMatch(t, []string{
		fmt.Sprintf(depthCapPayloadWarning, "[]D0", maxNamedResolutionDepth, "D9"),
		fmt.Sprintf(declsDisagreePayloadWarning, "[]W", "W"),
	}, payloadWarnings(a))
}

// TestRegisterPayloadRefsDemotes pins that a ref leaf that cannot register is
// demoted to a named leaf and warned about once, unless the payload already
// warned.
func TestRegisterPayloadRefsDemotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mod.go")
	require.NoError(t, os.WriteFile(path, []byte("package mod\n"), 0o600))
	a := New(dir)
	files, err := a.parsePackageDir(dir)
	require.NoError(t, err)
	site := pkgFile{file: files[path], path: path}

	r := tsMap(models.TypeShape{Kind: models.ShapeRef, Name: "Nope"})
	a.registerPayloadRefs(&r, map[string]pkgFile{}, site, "map[string]Nope", false)
	assert.Equal(t, models.ShapeNamed, r.Elem.Kind)
	assert.Equal(t, []string{fmt.Sprintf(unregisteredRefPayloadWarning, "map[string]Nope", "Nope")}, a.warnings)

	b := New(dir)
	r2 := tsSlice(models.TypeShape{Kind: models.ShapeRef, Name: "Nope"})
	b.registerPayloadRefs(&r2, map[string]pkgFile{"Nope": site}, site, "[]Nope", true)
	assert.Equal(t, models.ShapeNamed, r2.Elem.Kind)
	assert.Empty(t, b.warnings)
}

// TestExtractResponseTypeWritten pins the payload type as written that
// extractResponseType hands to the payload warnings.
func TestExtractResponseTypeWritten(t *testing.T) {
	src := `package mod

import srv "github.com/gaborage/go-bricks/server"

func a() (server.Result[map[string]Tier], error)     { return }
func b() ([]uintptr, error)                          { return }
func c() (server.NoContentResult, error)             { return }
func d() (srv.ResultWithMeta[*Cents], error)         { return }
func e() (Pair[A, B], error)                         { return }
`
	f, err := parser.ParseFile(token.NewFileSet(), "mod.go", src, 0)
	require.NoError(t, err)
	aliases := map[string]struct{}{"srv": {}, frameworkPkgServer: {}}
	want := []string{"map[string]Tier", "[]uintptr", "", "*Cents", "A"}
	an := New("")
	for i, decl := range f.Decls[1:] {
		fn := decl.(*ast.FuncDecl)
		_, written := an.extractResponseType(fn.Type.Results, "mod", aliases)
		assert.Equal(t, want[i], written, fn.Name.Name)
	}
	_, written := an.extractResponseType(nil, "mod", aliases)
	assert.Empty(t, written)
	assert.Empty(t, firstTypeArg(&ast.IndexListExpr{}))
}
