package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Issue 118: an anonymous struct payload is documented inline, as a
// field of its type is, and an anonymous struct request warns.

// anonPayloadDecls declares the named types the anonymous payload rows reach.
const anonPayloadDecls = `
var _ decimal.Decimal
var _ time.Time
var _ uuid.UUID

type Office struct{ Floor int ` + "`json:\"floor\"`" + ` }
type Rows []struct{ A int ` + "`json:\"a\"`" + ` }
type L []struct {
	Next L ` + "`json:\"next\"`" + `
}
`

// anonPayloadRow is one payload: the handler's first result, the rendered
// Resolution its route's response carries, and the payload warning it
// raises, if any.
type anonPayloadRow struct{ result, res, warn string }

func anonPayloadRows() []anonPayloadRow {
	return []anonPayloadRow{
		{result: "server.Result[struct{ A int `json:\"a\"` }]", res: "struct{a int}"},
		{result: "server.ResultWithMeta[struct{ A int }]", res: "struct{A int}"},
		{result: "struct{ A int }", res: "struct{A int}"},
		{result: "server.Result[*struct{ A int }]", res: "*struct{A int}"},
		{result: "server.Result[**struct{ A int }]", res: "**struct{A int}"},
		{result: "server.Result[[]struct{ A int }]", res: "[]struct{A int}"},
		{result: "[]struct{ A int }", res: "[]struct{A int}"},
		{result: "server.Result[[]*struct{ A int }]", res: "[]*struct{A int}"},
		{result: "server.Result[[2]struct{ A int }]", res: "[N]struct{A int}"},
		{result: "server.Result[map[string]struct{ A int }]", res: "map[string]struct{A int}"},
		{result: "server.Result[*map[string]struct{ A int }]", res: "*map[string]struct{A int}"},
		{result: "server.Result[struct{}]", res: "struct{}"},
		{result: "server.Result[struct{ In struct{ B string `json:\"b\"` } `json:\"in\"` }]", res: "struct{in struct{b string}}"},
		{result: "server.Result[struct{ O Office `json:\"o\"`; P []*Office `json:\"p\"` }]", res: "struct{o $Office; p []*$Office}"},
		{result: "server.Result[Rows]", res: "[]struct{a int}"},
		{result: "server.Result[L]", res: "[]struct{next []cycle:struct{…}}"},
		{result: "server.Result[struct{ uuid.UUID }]", res: "text:struct{…}"},
		{
			result: "server.Result[struct{ time.Time; Z int }]", res: "marshal:struct{…}(unknown)",
			warn: fmt.Sprintf(marshalerPromotedPayloadWarning, "struct{time.Time; Z int}", anonStructDisplay, methodMarshalJSON, "time.Time"),
		},
		{
			result: "server.Result[[]struct{ time.Time; Z int }]", res: "[]marshal:struct{…}(unknown)",
			warn: fmt.Sprintf(marshalerPromotedPayloadWarning, "[]struct{time.Time; Z int}", anonStructDisplay, methodMarshalJSON, "time.Time"),
		},
		{result: "server.Result[struct{ D decimal.Decimal `json:\"d\"` }]", res: "struct{d decimal.Decimal}"},
		{result: "server.Result[struct{ P uintptr `json:\"p\"` }]", res: "struct{p uintptr}"},
	}
}

// anonPayloadModule renders one route /p<i> per row, each handler returning
// the row's result.
func anonPayloadModule(rows []anonPayloadRow) string {
	var reg, handlers strings.Builder
	for i, r := range rows {
		fmt.Fprintf(&reg, "\tserver.GET(hr, r, \"/p%d\", m.h%d)\n", i, i)
		fmt.Fprintf(&handlers, "func (m *Module) h%d(ctx server.HandlerContext) (%s, server.IAPIError) {\n\tvar v %s\n\treturn v, nil\n}\n\n",
			i, r.result, r.result)
	}
	return `package mod

import (
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
` + anonPayloadDecls + `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
` + reg.String() + "}\n\n" + handlers.String()
}

// TestAnonymousStructPayloadRows pins every anonymous struct payload: wrapped
// in Result or ResultWithMeta or bare, behind pointers, as slice, array or map
// elements, nested, through a named slice, recursive, and embedding a text or
// other Marshaler type. Each names no component and is typed, from its
// Resolution, exactly as a field of its type is; the payload raises at most
// one warning, and the anonymous struct's own fields warn as fields.
func TestAnonymousStructPayloadRows(t *testing.T) {
	rows := anonPayloadRows()
	src := anonPayloadModule(rows)
	a, routes := analyzeSingleModule(t, src)
	want := []string{
		fmt.Sprintf(recursiveFieldWarning, "Next", anonLoc(t, src, "Next L"), "L", anonStructDisplay),
		fmt.Sprintf(unresolvableFieldWarning, "D", anonLoc(t, src, "D decimal.Decimal"), "decimal.Decimal", "decimal.Decimal"),
		"field P at " + anonLoc(t, src, "P uintptr") + " holds a uintptr, a machine address with no meaningful API contract " +
			"— emitting an untyped schema (use a sized integer type, or exclude the field with json:\"-\")",
	}
	for i, r := range rows {
		ti := routeByPath(t, routes, fmt.Sprintf("/p%d", i)).Response
		if !assert.NotNil(t, ti, r.result) {
			continue
		}
		assert.Empty(t, ti.Name, r.result)
		assert.Empty(t, ti.Fields, r.result)
		if assert.NotNil(t, ti.Resolution, r.result) {
			assert.Equal(t, r.res, renderShape(*ti.Resolution), r.result)
		}
		if r.warn != "" {
			want = append(want, r.warn)
		}
	}
	assert.ElementsMatch(t, want, a.Warnings(t.Context()))
	assert.Contains(t, a.typeRegistry, "Office", "a struct reachable only through an anonymous payload registers")
	assert.NotContains(t, a.typeRegistry, "Rows")
	assert.Empty(t, a.openLiterals)
}

// TestAnonymousStructPayloadTyping pins doctor's verdict on anonymous struct
// payloads: an inline object is typed, like a registered struct, whatever its
// fields hold; one that embeds a Marshaler type other than a text one is not.
func TestAnonymousStructPayloadTyping(t *testing.T) {
	rows := []anonPayloadRow{
		{result: "server.Result[struct{ A int }]"},
		{result: "server.Result[[]*struct{ A int }]"},
		{result: "server.Result[struct{}]"},
		{result: "server.Result[struct{ D decimal.Decimal }]"},
		{result: "server.Result[struct{ uuid.UUID }]"},
		{result: "server.Result[struct{ time.Time; Z int }]"},
	}
	_, routes := analyzeSingleModule(t, anonPayloadModule(rows))
	typed := make([]bool, len(rows))
	for i := range rows {
		typed[i] = IsTypedPayload(routeByPath(t, routes, fmt.Sprintf("/p%d", i)).Response)
	}
	assert.Equal(t, []bool{true, true, true, true, true, false}, typed)

	empty := models.TypeShape{Kind: models.ShapeStruct, Struct: &models.AnonymousStruct{Fields: []models.FieldInfo{}}}
	assert.False(t, isFallbackLeaf(&empty))
}

// TestAnonymousStructPayloadRefSites pins that a named struct reached only
// through an anonymous struct registers against the file its name was written
// in: the declaring file of a named slice over the literal, which imports the
// struct's package where the handler's file does not, and the handler's file
// for a literal written there, under its collision-qualified name.
func TestAnonymousStructPayloadRefSites(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":          resolveGoMod,
		"other/office.go": "package other\n\ntype Office struct{ Desk int `json:\"desk\"` }\n",
		"mod/types.go": "package mod\n\nimport \"github.com/example/app/other\"\n\n" +
			"type Office struct{ Floor int `json:\"floor\"` }\n\ntype Desks []struct{ O other.Office `json:\"o\"` }\n",
		"mod/module.go": resolveModuleHead + `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/desks", m.desks)
	server.GET(hr, r, "/offices", m.offices)
}

func (m *Module) desks(ctx server.HandlerContext) (server.Result[Desks], server.IAPIError) {
	return server.Result[Desks]{}, nil
}

func (m *Module) offices(ctx server.HandlerContext) (server.Result[struct{ O Office ` + "`json:\"o\"`" + ` }], server.IAPIError) {
	return server.Result[struct{ O Office ` + "`json:\"o\"`" + ` }]{}, nil
}
`,
	})
	desks := routeByPath(t, routes, "/desks").Response
	require.NotNil(t, desks)
	require.NotNil(t, desks.Resolution)
	offices := routeByPath(t, routes, "/offices").Response
	require.NotNil(t, offices)
	require.NotNil(t, offices.Resolution)
	pair := []string{renderShape(*desks.Resolution), renderShape(*offices.Resolution)}
	assert.Contains(t, [][]string{
		{"[]struct{o $OtherOffice}", "struct{o $Office}"},
		{"[]struct{o $Office}", "struct{o $ModOffice}"},
	}, pair, "both register, one collision-qualified")
	assert.Len(t, a.typeRegistry, 2)
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestCrossPackageNamedAnonymousPayload pins that a named slice over an
// anonymous struct declared in another package resolves inline, its struct
// field registering against that package, with no payload warning: it raised
// the no-schema-component warning before, behind a typed request too.
func TestCrossPackageNamedAnonymousPayload(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		"q/rows.go": "package q\n\ntype Office struct{ Floor int `json:\"floor\"` }\n\n" +
			"type Rows []struct {\n\tO Office `json:\"o\"`\n\tN int `json:\"n\"`\n}\n",
		"mod/module.go": strings.Replace(resolveModuleHead, "import (", "import (\n\t\"github.com/example/app/q\"", 1) + `
type Req struct{ ID int64 ` + "`param:\"id\"`" + ` }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows/:id", m.rows)
}

func (m *Module) rows(req Req, ctx server.HandlerContext) (server.Result[q.Rows], server.IAPIError) {
	return server.Result[q.Rows]{}, nil
}
`,
	})
	rows := routeByPath(t, routes, "/rows/{id}").Response
	require.NotNil(t, rows)
	require.NotNil(t, rows.Resolution)
	assert.Equal(t, "[]struct{o $Office; n int}", renderShape(*rows.Resolution))
	assert.Contains(t, a.typeRegistry, "Office")
	assert.NotContains(t, a.typeRegistry, "Rows")
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestRequestlessAnonymousPayloadIsFound pins that a request-less handler
// returning an anonymous struct payload is found, so its constructor-derived
// success status and inferred error statuses are documented.
func TestRequestlessAnonymousPayloadIsFound(t *testing.T) {
	_, routes := analyzeSingleModule(t, resolveModuleHead+`
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/anon", m.anon)
}

func (m *Module) anon(ctx server.HandlerContext) (*struct{ A int }, server.IAPIError) {
	if ctx.Echo == nil {
		return nil, server.NewConflictError("x")
	}
	return nil, nil
}
`)
	anon := routeByPath(t, routes, "/anon")
	require.NotNil(t, anon.Response)
	assert.Equal(t, []int{409}, anon.ErrorStatuses)
}

// TestJOSEAnonymousPayloadKeepsBase pins that a payload whose root, every
// written pointer shed, is a jose-tagged anonymous struct is left as it was
// before anonymous struct payloads were documented (issue 171): it resolves
// to no response type, so the route keeps the untyped fallback, a struct only
// it reaches does not register, a malformed tag in it raises nothing, and a
// request-less handler returning one is not found. One under a slice is an
// ordinary anonymous struct payload, its jose tag ignored.
func TestJOSEAnonymousPayloadKeepsBase(t *testing.T) {
	tagged := "struct {\n\t_ struct{} `jose:\"encrypt=k3,sign=k4\"`\n\tO Only `json:\"o\"`\n\tP int `bad json:\"p\"`\n}"
	listed := "[]struct {\n\t_ struct{} `jose:\"encrypt=k3,sign=k4\"`\n\tB int `json:\"b\"`\n}"
	a, routes := analyzeSingleModule(t, resolveModuleHead+`
type JReq struct {
	_ struct{} `+"`jose:\"decrypt=k1,verify=k2\"`"+`
	A int `+"`json:\"a\"`"+`
}

type Only struct{ Q int }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/bare", m.bare)
	server.POST(hr, r, "/wrapped", m.wrapped)
	server.POST(hr, r, "/deep", m.deep)
	server.POST(hr, r, "/none", m.none)
	server.POST(hr, r, "/list", m.list)
}

func (m *Module) bare(req JReq, ctx server.HandlerContext) (`+tagged+`, server.IAPIError) {
	return `+tagged+`{}, nil
}

func (m *Module) wrapped(req JReq, ctx server.HandlerContext) (server.Result[*`+tagged+`], server.IAPIError) {
	return server.Result[*`+tagged+`]{}, nil
}

func (m *Module) deep(req JReq, ctx server.HandlerContext) (server.Result[**`+tagged+`], server.IAPIError) {
	return server.Result[**`+tagged+`]{}, nil
}

func (m *Module) none(ctx server.HandlerContext) (*`+tagged+`, server.IAPIError) {
	if ctx.Echo == nil {
		return nil, server.NewConflictError("x")
	}
	return nil, nil
}

func (m *Module) list(req JReq, ctx server.HandlerContext) (server.Result[`+listed+`], server.IAPIError) {
	return server.Result[`+listed+`]{}, nil
}
`)
	for _, path := range []string{"/bare", "/wrapped", "/deep", "/none"} {
		assert.Nil(t, routeByPath(t, routes, path).Response, path)
	}
	assert.Empty(t, routeByPath(t, routes, "/none").ErrorStatuses, "a request-less handler returning one is not found")
	list := routeByPath(t, routes, "/list").Response
	require.NotNil(t, list)
	assert.False(t, list.JOSE)
	require.NotNil(t, list.Resolution)
	assert.Equal(t, "[]struct{b int}", renderShape(*list.Resolution))
	assert.NotContains(t, a.typeRegistry, "Only")
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestAnonymousStructRequestWarns pins that a request parameter of anonymous
// struct type, in either position, behind one pointer or parenthesized, is
// the request: go-bricks binds it, but it is not documented yet, so it raises
// the anonymous request warning once and stays untyped, and its handler is
// found. Behind two pointers it is a non-struct request (F26). Only a
// field-less one is silent (TestEmptyAnonymousStructRequestIsSilent): one
// whose fields are all blank or unexported still warns.
func TestAnonymousStructRequestWarns(t *testing.T) {
	cases := []struct{ params, written string }{
		{"req struct{ ID int64 `param:\"id\"`; Q string `query:\"q\"` }, ctx server.HandlerContext", "struct{ID int64; Q string}"},
		{"ctx server.HandlerContext, req *struct{ A int `json:\"a\" validate:\"required\"` }", "*struct{A int}"},
		{"req (struct{ A int }), ctx server.HandlerContext", "(struct{A int})"},
		{"req *(struct{ A int }), ctx server.HandlerContext", "*(struct{A int})"},
		{"req struct{ _ int }, ctx server.HandlerContext", "struct{_ int}"},
	}
	for _, c := range cases {
		t.Run(c.written, func(t *testing.T) {
			route, warnings := analyzeNonStructRequest(t, c.params, "Address")
			assert.Equal(t, []string{fmt.Sprintf(anonymousRequestWarning, c.written)}, warnings)
			assert.Nil(t, route.Request)
			require.NotNil(t, route.Response, "the response is resolved independently of the request")
			assert.Equal(t, "Address", route.Response.Name)
		})
	}

	route, warnings := analyzeNonStructRequest(t, "req **struct{ A int }, ctx server.HandlerContext", "Address")
	assert.Equal(t, []string{wantNonStructRequestWarning("**struct{A int}")}, warnings)
	assertUntypedRequest(t, route.Request)
}

// TestEmptyAnonymousStructRequestIsSilent pins that a field-less anonymous
// struct request, go-bricks' request type for a route that takes no input,
// raises no warning: there is nothing to document, and a named empty struct
// documents nothing either. Behind two pointers it is still F26.
func TestEmptyAnonymousStructRequestIsSilent(t *testing.T) {
	for _, params := range []string{
		"_ struct{}, ctx server.HandlerContext",
		"ctx server.HandlerContext, req *struct{}",
		"req (struct{}), ctx server.HandlerContext",
		"req *(struct{}), ctx server.HandlerContext",
	} {
		t.Run(params, func(t *testing.T) {
			route, warnings := analyzeNonStructRequest(t, params, "Address")
			assert.Empty(t, warnings)
			assert.Nil(t, route.Request)
			require.NotNil(t, route.Response)
			assert.Equal(t, "Address", route.Response.Name)
		})
	}

	route, warnings := analyzeNonStructRequest(t, "req **struct{}, ctx server.HandlerContext", "Address")
	assert.Equal(t, []string{wantNonStructRequestWarning("**struct{}")}, warnings)
	assertUntypedRequest(t, route.Request)
}

// TestEmptyAnonymousStructRequestHandlerIsFound pins that a field-less
// anonymous struct request is still the handler's request, so a handler whose
// payload resolves to nothing is found and keeps its statuses.
func TestEmptyAnonymousStructRequestHandlerIsFound(t *testing.T) {
	src := strings.Replace(nonStructRequestModSrc, "return out, nil",
		"if ctx.Echo == nil {\n\t\treturn out, server.NewNotFoundError(\"x\")\n\t}\n\treturn server.Created(out), nil", 1)
	src = strings.NewReplacer("PARAMS", "_ struct{}, ctx server.HandlerContext",
		"RESULT", "server.Result[map[string]func()]").Replace(src)
	a, routes := analyzeSingleModule(t, src)
	route := routeForPath(t, routes, nonStructRequestRouteKey)
	assert.Nil(t, route.Request)
	assert.Nil(t, route.Response)
	assert.Equal(t, 201, route.SuccessStatus)
	assert.Equal(t, []int{404}, route.ErrorStatuses)
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestAnonymousStructRequestHandlerIsFound pins that a handler whose only
// carried type is an anonymous struct request is found, so its statuses are
// kept: its payload (an unmodelled leaf) resolves to nothing.
func TestAnonymousStructRequestHandlerIsFound(t *testing.T) {
	src := strings.Replace(nonStructRequestModSrc, "return out, nil",
		"if ctx.Echo == nil {\n\t\treturn out, server.NewNotFoundError(\"x\")\n\t}\n\treturn server.Created(out), nil", 1)
	src = strings.NewReplacer("PARAMS", "req struct{ A int }, ctx server.HandlerContext",
		"RESULT", "server.Result[map[string]func()]").Replace(src)
	a, routes := analyzeSingleModule(t, src)
	route := routeForPath(t, routes, nonStructRequestRouteKey)
	assert.Nil(t, route.Request)
	assert.Nil(t, route.Response)
	assert.Equal(t, 201, route.SuccessStatus)
	assert.Equal(t, []int{404}, route.ErrorStatuses)
	assert.Equal(t, []string{fmt.Sprintf(anonymousRequestWarning, "struct{A int}")}, a.Warnings(t.Context()))
}

// TestIsAnonymousStruct pins the request screen: a struct literal, one
// pointer level and any parentheses shed.
func TestIsAnonymousStruct(t *testing.T) {
	for src, want := range map[string]bool{
		"struct{ A int }":    true,
		"*struct{ A int }":   true,
		"(struct{ A int })":  true,
		"*(struct{ A int })": true,
		"(*struct{ A int })": true,
		"**struct{ A int }":  false,
		"Address":            false,
		"*Address":           false,
		"[]struct{ A int }":  false,
	} {
		assert.Equal(t, want, isAnonymousStruct(mustParse(t, src)), src)
	}
	assert.False(t, isAnonymousStruct(nil))
}

// TestAnonymousStructPayloadExpansionLimit pins that a payload's anonymous
// structs share the field's expansion limit: twelve named slices over
// anonymous structs that each hold two fields of the next unfold past
// maxLiteralExtractions from the payload's root, with no field holding them.
// The one latched warning fits that trigger too: it names the payload's
// outermost anonymous struct, Q0's, not a field, nor the small one an
// earlier route extracts.
func TestAnonymousStructPayloadExpansionLimit(t *testing.T) {
	const n = 11
	var decls strings.Builder
	for i := range n {
		fmt.Fprintf(&decls, "type Q%d []struct{ A Q%d `json:\"a\"`; B Q%d `json:\"b\"` }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&decls, "type Q%d []struct{ Z int `json:\"z\"` }\n", n)
	src := resolveModuleHead + decls.String() + `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/small", m.small)
	server.GET(hr, r, "/q", m.q)
}

func (m *Module) small(ctx server.HandlerContext) (server.Result[struct{ X int }], server.IAPIError) {
	return server.Result[struct{ X int }]{}, nil
}

func (m *Module) q(ctx server.HandlerContext) (server.Result[Q0], server.IAPIError) {
	return server.Result[Q0]{}, nil
}
`
	a, routes := analyzeSingleModule(t, src)
	q := routeByPath(t, routes, "/q")
	require.NotNil(t, q.Response)
	require.NotNil(t, q.Response.Resolution, "the payload still resolves, truncated past the limit")
	root := anonLoc(t, src, "struct{ A Q1 ")
	assert.Equal(t, []string{fmt.Sprintf(literalLimitWarning, root, maxLiteralExtractions)}, a.Warnings(t.Context()))
	assert.Empty(t, a.openLiterals)
}
