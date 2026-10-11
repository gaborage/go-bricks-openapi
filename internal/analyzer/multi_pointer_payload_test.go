package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

const (
	mpAddress      = "Address"
	mpDoubleInt64  = "**int64"
	mpResultDouble = "server.Result[**int64]"
)

// multiPointerPayloadRows are the payload rows of #120 beyond payloadRows'
// U3: a written root of two or more pointer levels resolves exactly as its
// *T form does, in every carrier (Result, ResultWithMeta, a bare return),
// while a pointer chain below a container keeps the nameless composite path.
func multiPointerPayloadRows() []payloadRow {
	unresolvable := func(name string) string { return fmt.Sprintf(unresolvablePayloadWarning, name) }
	return []payloadRow{
		// Any depth, any carrier.
		{result: "server.Result[***int64]", res: "***int64"},
		{result: "server.ResultWithMeta[**int64]", res: mpDoubleInt64},
		{result: mpDoubleInt64, res: mpDoubleInt64},
		{result: "server.Result[**string]", res: "**string"},
		// A named root: a struct registers, a named scalar resolves.
		{result: "server.Result[***Address]", name: mpAddress},
		{result: "server.Result[**Cents]", res: mpDoubleInt64},
		{result: "server.Result[**b.Cents]", res: mpDoubleInt64},
		{result: "server.Result[**PC]", res: "***int64"},
		{result: "server.Result[**time.Time]", res: "**time.Time", name: "Time"},
		{result: "server.Result[**Tier]", res: "**marshal:Tier(int)",
			warn: fmt.Sprintf(marshalerPayloadWarning, "**Tier", rowTier, rowMarshalTxt)},
		// A composite pointee.
		{result: "server.Result[**[]Address]", res: "**[]$Address"},
		{result: "server.Result[**map[string]int64]", res: "**map[string]int64"},
		{result: "server.Result[**[2]Cents]", res: "**[N]int64"},
		{result: "server.Result[**interface{}]", res: "**interface{}"},
		// An unresolvable root warns with its bare name, like *Missing (F7).
		{result: "server.Result[**Missing]", warn: unresolvable("Missing")},
		{result: "server.Result[**decimal.Decimal]", warn: unresolvable(rowDecimal)},
		// Unchanged: a pointer chain below a container.
		{result: "server.Result[[]**Address]", res: "[]**$Address", shape: "[]**Address"},
		{result: "server.Result[map[string]**int64]", res: "map[string]**int64"},
		{result: "server.Result[*[]**int64]", res: "*[]**int64"},
	}
}

// TestMultiPointerPayloadRows pins #120's payload rows: the Resolution, name
// and one warning of each payload whose written root has two or more pointer
// levels.
func TestMultiPointerPayloadRows(t *testing.T) {
	rows := multiPointerPayloadRows()
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("b", "b.go"):        "package b\n\ntype Cents int64\n",
		filepath.Join("mod", "types.go"):  payloadRowsTypes,
		filepath.Join("mod", "module.go"): payloadRowsModule(rows),
	})
	var wantWarnings []string
	for i, r := range rows {
		t.Run(r.result, func(t *testing.T) {
			ti := routeByPath(t, routes, fmt.Sprintf("/p%d", i)).Response
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
	assert.ElementsMatch(t, wantWarnings, payloadWarnings(a))
}

// TestMultiPointerStructPayloadRegistersComponent pins that a struct reached
// only through a **T payload still registers its component, and its route
// $refs it (Name set, no Shape left to inline).
func TestMultiPointerStructPayloadRegistersComponent(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): resolveModuleHead + `
type Address struct {
	City string ` + "`json:\"city\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/addr", m.addr)
}

func (m *Module) addr(ctx server.HandlerContext) (server.Result[**Address], server.IAPIError) {
	return server.Result[**Address]{}, nil
}
`,
	})
	assert.Empty(t, a.Warnings(t.Context()))
	assert.Contains(t, a.typeRegistry, mpAddress)
	resp := routeByPath(t, routes, "/addr").Response
	require.NotNil(t, resp)
	assert.Equal(t, mpAddress, resp.Name)
	assert.Nil(t, resp.Shape)
	assert.Nil(t, resp.Resolution)
}

// TestMultiPointerPayloadBesideTypedRequest pins the case #120 found silent:
// beside a typed request, Result[**int64] is now typed, and Result[**T] over
// an unresolvable T warns exactly as Result[*T] does, so --strict sees it.
func TestMultiPointerPayloadBesideTypedRequest(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): `package mod

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type GetReq struct {
	ID string ` + "`param:\"id\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/count/:id", m.count)
	server.GET(hr, r, "/price/:id", m.price)
}

func (m *Module) count(req GetReq, ctx server.HandlerContext) (server.Result[**int64], server.IAPIError) {
	return server.Result[**int64]{}, nil
}

func (m *Module) price(req GetReq, ctx server.HandlerContext) (server.Result[**decimal.Decimal], server.IAPIError) {
	return server.Result[**decimal.Decimal]{}, nil
}
`,
	})
	count := routeByPath(t, routes, "/count/{id}").Response
	require.NotNil(t, count)
	require.NotNil(t, count.Resolution)
	assert.Equal(t, mpDoubleInt64, renderShape(*count.Resolution))
	assert.Empty(t, count.Name)

	price := routeByPath(t, routes, "/price/{id}").Response
	require.NotNil(t, price)
	assert.Empty(t, price.Name)
	assert.Nil(t, price.Resolution)
	assert.Equal(t, []string{fmt.Sprintf(unresolvablePayloadWarning, rowDecimal)}, a.Warnings(t.Context()))
}

// TestMultiPointerRequestlessPayloadKeepsStatuses pins the request-less half
// of #120's output change: a handler with no request parameter whose payload
// is **T now has a response, so it is found as the handler and keeps its
// constructor success status and inferred error statuses, exactly as the *T
// form does (TestRequestlessCompositePayloadKeepsStatuses).
func TestMultiPointerRequestlessPayloadKeepsStatuses(t *testing.T) {
	_, routes := analyzeSingleModule(t, resolveModuleHead+`
type Address struct {
	City string `+"`json:\"city\"`"+`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/addr", m.create)
}

func (m *Module) create(ctx server.HandlerContext) (server.Result[**Address], server.IAPIError) {
	var a *Address
	if a == nil {
		return server.Result[**Address]{}, server.NewNotFoundError("x")
	}
	return server.Created(&a), nil
}
`)
	created := routeByPath(t, routes, "/addr")
	require.NotNil(t, created.Response)
	assert.Equal(t, mpAddress, created.Response.Name)
	assert.Equal(t, 201, created.SuccessStatus)
	assert.Contains(t, created.ErrorStatuses, 404)
}

// TestMultiPointerPayloadTypeInfo pins extraction through payloadTypeInfo: a
// written **T root keeps every pointer level in its Shape and is named as *T
// is; a framework pointee still resolves to nothing.
func TestMultiPointerPayloadTypeInfo(t *testing.T) {
	a := New("")
	extract := func(src string) *models.TypeInfo {
		return a.payloadTypeInfo(mustParse(t, src), "mod", map[string]struct{}{})
	}
	cases := []struct{ src, name, shape string }{
		{mpDoubleInt64, "", mpDoubleInt64},
		{"***Address", mpAddress, "***Address"},
		{"**q.Item", "Item", "**q.Item"},
		{"**[]string", "", "**[]string"},
		{"***map[string]A", "", "***map[string]A"},
		{"**interface{}", "", "**interface{}"},
	}
	for _, c := range cases {
		ti := extract(c.src)
		if assert.NotNil(t, ti, c.src) {
			assert.Equal(t, c.name, ti.Name, c.src)
			require.NotNil(t, ti.Shape, c.src)
			assert.Equal(t, c.shape, renderShape(*ti.Shape), c.src)
		}
	}
	assert.Nil(t, extract("**server.IAPIError"), "a framework pointee is not a payload")
}

// TestSinglePointer pins the peel scalarPayloadTypeInfo names a payload by: a
// written pointer chain is cut to its last level, and a single pointer is
// returned as-is.
func TestSinglePointer(t *testing.T) {
	for src, want := range map[string]string{
		"*int64":      "*int64",
		mpDoubleInt64: "*int64",
		"***Address":  "*Address",
		"**[]string":  "*[]string",
		"**q.Item":    "*q.Item",
	} {
		star, ok := mustParse(t, src).(*ast.StarExpr)
		require.True(t, ok, src)
		assert.Equal(t, want, types.ExprString(singlePointer(star)), src)
	}
	one, ok := mustParse(t, "*Item").(*ast.StarExpr)
	require.True(t, ok)
	assert.Same(t, one, singlePointer(one))
}

// TestPayloadBaseShapeShedsEveryRootPointer pins payloadBaseShape's two arms:
// a non-sequence root sheds every pointer level, so **time.Time is named
// time.Time and **int64 is a builtin; a sequence element sheds exactly one,
// so []**Address stays a pointer element on the nameless composite path.
func TestPayloadBaseShapeShedsEveryRootPointer(t *testing.T) {
	timeT := tsNamed(models.WellKnownTimeTime)
	addr := tsNamed(mpAddress)
	cases := []struct {
		in, want models.TypeShape
	}{
		{tsPtr(tsPtr(timeT)), timeT},
		{tsPtr(tsPtr(tsPtr(tsPrim(goTypeInt64)))), tsPrim(goTypeInt64)},
		{tsPtr(tsSlice(addr)), tsSlice(addr)},
		{tsSlice(tsPtr(tsPtr(addr))), tsPtr(addr)},
		{models.TypeShape{Kind: models.ShapeArray, Elem: ptrTo(tsPtr(tsPtr(addr)))}, tsPtr(addr)},
		{tsPtr(models.TypeShape{Kind: models.ShapePointer}), models.TypeShape{Kind: models.ShapePointer}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, payloadBaseShape(c.in), renderShape(c.in))
	}
}

func ptrTo(s models.TypeShape) *models.TypeShape { return &s }
