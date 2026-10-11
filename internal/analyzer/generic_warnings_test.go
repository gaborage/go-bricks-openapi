package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Issue 138: an unresolved generic type instantiation raises one warning
// wherever it is written. The generic_warnings golden pins that the emitted
// spec does not move; these tests carry every warning claim.

// genericPgSrc is the in-module package pg: a generic struct and a generic
// slice, for the pg.-qualified rows.
const genericPgSrc = `package pg

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}

type List[T any] []T

type AddrPage = Page[Addr]

type MPage Page[Addr]

func (p MPage) MarshalJSON() ([]byte, error) { return nil, nil }

type Addr struct {
	City string ` + "`json:\"city\"`" + `
}
`

// genericModSrc is a module whose one route, POST /items/:id, takes the
// handler parameter list PARAMS and returns RESULT. It declares every generic
// base and every struct the generic rows use.
const genericModSrc = `package mod

import (
	"github.com/example/app/pg"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/samber/mo"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

type Receipt struct {
	ID string ` + "`json:\"id\"`" + `
}

type IDReq struct {
	ID string ` + "`param:\"id\"`" + `
}

type List[T any] []T
type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}
type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
	Total int ` + "`json:\"total\"`" + `
}
type Pair[K comparable, V any] struct {
	Key K ` + "`json:\"key\"`" + `
	Val V ` + "`json:\"val\"`" + `
}
type ID[T any] string
type AddrPage Page[Address]
type AliasPage = Page[Address]
type LP []Page[Address]
type AL List[Address]
type AddrSlice []Address
type Status string
type GA[T any] = Box[T]
type GD[T any] Box[T]
type GL[T any] = List[T]
type GP[T any] = pg.Page[T]
type GPL[T any] pg.List[T]
type GParen[T any] (Box[T])
type Opt[T any] interface{ Get() T }
type PP *Page[Address]
type PA *Address
type PtrBox[T any] *Box[T]
type Cyc1[T any] = Cyc2[T]
type Cyc2[T any] = Cyc1[T]
type JBox[T any] struct {
	V T
}

func (b JBox[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type TBox[T any] struct {
	V T
}

func (b TBox[T]) MarshalText() ([]byte, error) { return nil, nil }
func (b *TBox[T]) UnmarshalText([]byte) error  { return nil }

type MPage Page[Address]

func (p MPage) MarshalJSON() ([]byte, error) { return nil, nil }

type UPage Page[Address]

func (p *UPage) UnmarshalJSON([]byte) error { return nil }

type TPage Page[Address]

func (p TPage) MarshalText() ([]byte, error) { return nil, nil }
func (p *TPage) UnmarshalText([]byte) error  { return nil }

type AliasM = MPage
type ML List[Address]

func (l ML) MarshalJSON() ([]byte, error) { return nil, nil }

type Tree[T any] map[string]Tree[T]
type NTree map[string]NTree

type Fields struct {
	L   List[string] ` + "`json:\"l\"`" + `
	B   Box[int] ` + "`json:\"b\"`" + `
	P   Page[Address] ` + "`json:\"p\"`" + `
	PR  Pair[string, int] ` + "`json:\"pr\"`" + `
	AP  AddrPage ` + "`json:\"ap\"`" + `
	AAP AliasPage ` + "`json:\"aap\"`" + `
	ALF AL ` + "`json:\"al\"`" + `
	JB  JBox[int] ` + "`json:\"jb\"`" + `
	TB  TBox[int] ` + "`json:\"tb\"`" + `
	TR  Tree[int] ` + "`json:\"tr\"`" + `
	PGP pg.Page[Address] ` + "`json:\"pgp\"`" + `
	PGL pg.List[Address] ` + "`json:\"pgl\"`" + `
	MO  mo.Option[Address] ` + "`json:\"mo\"`" + `
	UD  Undeclared[Address] ` + "`json:\"ud\"`" + `
	PB  *Box[Address] ` + "`json:\"pb\"`" + `
	SB  []Box[Address] ` + "`json:\"sb\"`" + `
	LPF LP ` + "`json:\"lp\"`" + `
	MB  map[string]Box[int] ` + "`json:\"mb\"`" + `
	NB  Box[Page[Address]] ` + "`json:\"nb\"`" + `
	Page[Address] ` + "`json:\"page\"`" + `
	*pg.List[Address] ` + "`json:\"pgEmbed\"`" + `
}

type Params struct {
	IDs  List[string] ` + "`query:\"ids\"`" + `
	ID   ID[int] ` + "`param:\"id\"`" + `
	Tags List[string] ` + "`header:\"X-Tags\"`" + `
	BQ   Box[int] ` + "`query:\"b\"`" + `
}

type Controls struct {
	Page[Address]
	LP
	Note string ` + "`json:\"note\"`" + `
	X    T ` + "`json:\"x\"`" + `
	NT   NTree ` + "`json:\"nt\"`" + `
	Ch   chan int ` + "`json:\"ch\"`" + `
	Fn   func() ` + "`json:\"fn\"`" + `
	Anon struct{ X int } ` + "`json:\"anon\"`" + `
	Skip Box[int] ` + "`json:\"-\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/items/:id", m.create)
}

func (m *Module) create(PARAMS) (RESULT, server.IAPIError) {
	return out, nil
}
`

const genericModFile = "mod/module.go"

// genericFieldTail is the field warning after its position: the declared
// type and the instantiation.
func genericFieldTail(declared, inst string) string {
	return fmt.Sprintf(genericFieldWarning, "", "", declared, inst)[len("field  at "):]
}

// genericRun is one analysis of genericModSrc: the analyzer, its one route,
// every analyzer warning and the module source.
type genericRun struct {
	a        *ProjectAnalyzer
	route    models.Route
	warnings []string
	src      string
}

// analyzeGenericRoute analyzes genericModSrc with the given handler
// parameter list and first result.
func analyzeGenericRoute(t *testing.T, params, result string) genericRun {
	t.Helper()
	src := strings.NewReplacer("PARAMS", params, "RESULT", result).Replace(genericModSrc)
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("pg", "pg.go"):       genericPgSrc,
		filepath.FromSlash(genericModFile): src,
	})
	require.Len(t, routes, 1)
	warnings := a.Warnings(t.Context())
	return genericRun{a: a, route: routes[0], warnings: warnings, src: src}
}

// genericLoc renders the position of the first line of src containing
// needle, at needle, as the analyzer renders a field's position.
func genericLoc(t *testing.T, src, needle string) string {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if col := strings.Index(line, needle); col >= 0 {
			return fmt.Sprintf("%s:%d:%d", filepath.FromSlash(genericModFile), i+1, col+1)
		}
	}
	require.Failf(t, "needle not found", "%q", needle)
	return ""
}

// genericFieldRows are the body fields of Fields: Go name, declared type and
// the instantiation the warning names.
var genericFieldRows = []struct{ name, declared, inst string }{
	{"L", "List[string]", "List[string]"},
	{"B", "Box[int]", "Box[int]"},
	{"P", "Page[Address]", "Page[Address]"},
	{"PR", "Pair[string, int]", "Pair[string, int]"},
	{"AP", "AddrPage", "Page[Address]"},
	{"AAP", "AliasPage", "Page[Address]"},
	{"ALF", "AL", "List[Address]"},
	{"JB", "JBox[int]", "JBox[int]"},
	{"TB", "TBox[int]", "TBox[int]"},
	{"TR", "Tree[int]", "Tree[int]"},
	{"PGP", "pg.Page[Address]", "pg.Page[Address]"},
	{"PGL", "pg.List[Address]", "pg.List[Address]"},
	{"MO", "mo.Option[Address]", "mo.Option[Address]"},
	{"UD", "Undeclared[Address]", "Undeclared[Address]"},
	{"PB", "*Box[Address]", "Box[Address]"},
	{"SB", "[]Box[Address]", "Box[Address]"},
	{"LPF", "LP", "Page[Address]"},
	{"MB", "map[string]Box[int]", "Box[int]"},
	{"NB", "Box[Page[Address]]", "Box[Page[Address]]"},
	{"Page", "Page[Address]", "Page[Address]"},
	{"pg.List", "*pg.List[Address]", "pg.List[Address]"},
}

// TestGenericBodyFieldsWarnOnce pins table 1: each generic body field raises
// exactly one warning, the generic field text, even though Fields is both the
// request body and the payload. A json-tagged generic embed is a field named
// by its generic type as written, never "unknown".
func TestGenericBodyFieldsWarnOnce(t *testing.T) {
	run := analyzeGenericRoute(t, "req Fields, ctx server.HandlerContext", "server.Result[Fields]")
	a, route, warnings, src := run.a, run.route, run.warnings, run.src
	require.Len(t, warnings, len(genericFieldRows), "one warning per generic field, no other: %v", warnings)
	for _, r := range genericFieldRows {
		w := fieldWarnings(a, r.name)
		if assert.Len(t, w, 1, r.name) {
			assert.True(t, strings.HasSuffix(w[0], genericFieldTail(r.declared, r.inst)), "%s: %s", r.name, w[0])
		}
	}
	loc := genericLoc(t, src, "MB  map")
	assert.Contains(t, warnings, "field MB at "+loc+" has type map[string]Box[int]: Box[int] is a generic type instantiation, "+
		"which is not resolved — emitting an untyped object")
	assert.Equal(t, "field Page at "+genericLoc(t, src, "Page[Address] `json:\"page\"`")+" has type Page[Address]: Page[Address] "+
		"is a generic type instantiation, which is not resolved — emitting an untyped object", fieldWarnings(a, "Page")[0])
	for _, w := range warnings {
		assert.NotContains(t, w, "field unknown at")
	}

	require.NotNil(t, route.Request)
	assert.Equal(t, "Fields", route.Request.Name)
	byJSON := map[string]*models.FieldInfo{}
	for i := range route.Request.Fields {
		byJSON[route.Request.Fields[i].JSONName] = &route.Request.Fields[i]
	}
	require.Contains(t, byJSON, "page")
	require.Contains(t, byJSON, "pgEmbed")
	assert.Equal(t, "Page", byJSON["page"].Name)
	assert.Equal(t, "pg.List", byJSON["pgEmbed"].Name)
	assert.Equal(t, "inst:Box[int]", renderShape(byJSON["b"].ResolvedShape()))
	assert.Equal(t, "*inst:Box[Address]", renderShape(byJSON["pb"].ResolvedShape()))
	assert.Equal(t, "[]inst:Page[Address]", renderShape(byJSON["lp"].ResolvedShape()))
	assertNoGenericComponent(t, a)
}

// assertNoGenericComponent asserts no generic base was registered as a
// component: an Instantiation never reaches a consumer as its bare base.
func assertNoGenericComponent(t *testing.T, a *ProjectAnalyzer) {
	t.Helper()
	for _, name := range []string{"Page", "Box", "Pair", "List", "JBox", "TBox", "Tree", "PgPage", "PgList"} {
		assert.NotContains(t, a.typeRegistry, name)
	}
}

// TestGenericParamFieldsWarnOnce pins table 2: a generic path, query or
// header parameter raises the generic field warning once, and its parameter
// is still an untyped object.
func TestGenericParamFieldsWarnOnce(t *testing.T) {
	run := analyzeGenericRoute(t, "req Params, ctx server.HandlerContext", "server.Result[Receipt]")
	a, route, warnings := run.a, run.route, run.warnings
	rows := []struct{ name, declared string }{
		{"IDs", "List[string]"}, {"ID", "ID[int]"}, {"Tags", "List[string]"}, {"BQ", "Box[int]"},
	}
	require.Len(t, warnings, len(rows), "%v", warnings)
	for _, r := range rows {
		w := fieldWarnings(a, r.name)
		if assert.Len(t, w, 1, r.name) {
			assert.True(t, strings.HasSuffix(w[0], genericFieldTail(r.declared, r.declared)), w[0])
		}
	}
	require.NotNil(t, route.Request)
	require.Len(t, route.Request.Fields, len(rows))
	for i := range route.Request.Fields {
		f := &route.Request.Fields[i]
		assert.NotEmpty(t, f.ParamType, f.Name)
		assert.Equal(t, models.ShapeInstantiation, f.ResolvedShape().Kind, f.Name)
	}
}

// TestGenericControlsUnchanged pins table 5: an untagged generic embed keeps
// issue 116's dropped-embed warning and gets no second one, an untagged named
// slice of an instantiation is a property with the generic field warning,
// and an undeclared bare type, a recursive map, chan, func, anonymous struct
// and json:"-" fields keep what they had.
func TestGenericControlsUnchanged(t *testing.T) {
	run := analyzeGenericRoute(t, "req IDReq, ctx server.HandlerContext", "server.Result[Controls]")
	a, warnings := run.a, run.warnings
	assert.Len(t, warnings, 4, "%v", warnings)
	assert.Len(t, matching(warnings, "embedded field Page at "), 1)
	assert.Len(t, matching(warnings, "has type Page[Address]: Page[Address] is a generic instantiation, which is not expanded"), 1)
	if w := fieldWarnings(a, "LP"); assert.Len(t, w, 1) {
		assert.True(t, strings.HasSuffix(w[0], genericFieldTail("LP", "Page[Address]")), w[0])
	}
	if w := fieldWarnings(a, "X"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "T resolves to no schema")
	}
	if w := fieldWarnings(a, "NT"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "contains itself")
	}
	for _, name := range []string{"Ch", "Fn", "Anon", "Skip", "Note"} {
		assert.Empty(t, fieldWarnings(a, name), name)
	}
}

// TestGenericEmbedMalformedTagKeepsUnknown pins that a json-tagged generic
// embed with a tag reflect cannot fully read keeps "unknown" in the
// malformed-tag warning, while its field warning names it by its generic type.
func TestGenericEmbedMalformedTagKeepsUnknown(t *testing.T) {
	src := strings.Replace(genericModSrc, "type Params struct {",
		"type Odd struct {\n\tBox[int] \"json:\\\"box\\\"\\n\\tvalidate:\\\"required\\\"\"\n}\n\ntype Params struct {", 1)
	src = strings.NewReplacer("PARAMS", "req Odd, ctx server.HandlerContext", "RESULT", "server.Result[Receipt]").Replace(src)
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("pg", "pg.go"):       genericPgSrc,
		filepath.FromSlash(genericModFile): src,
	})
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 2, "%v", warnings)
	assert.Len(t, matching(warnings, "struct tag on field unknown at "), 1)
	if w := fieldWarnings(a, "Box"); assert.Len(t, w, 1) {
		assert.True(t, strings.HasPrefix(w[0], "field Box at "), w[0])
		assert.True(t, strings.HasSuffix(w[0], genericFieldTail("Box[int]", "Box[int]")), w[0])
	}
}

// genericRequestRows are the request types of table 3 and the one warning
// each raises: the generic request text (a struct base, or one the tool
// cannot locate) or F26 (a non-struct base). A type's own (un)marshaling
// methods do not change its kind, so a Marshaler or text type declared over
// a struct instantiation (MPage, UPage, TPage) is a generic request, and one
// over a non-struct (ML) stays F26.
func genericRequestRows() []struct{ typ, want string } {
	generic := func(typ, inst string) string { return fmt.Sprintf(genericRequestWarning, typ, inst) }
	f26 := func(typ string) string { return fmt.Sprintf(nonStructRequestWarning, typ) }
	return []struct{ typ, want string }{
		{"Box[Address]", generic("Box[Address]", "Box[Address]")},
		{"*Box[Address]", generic("*Box[Address]", "Box[Address]")},
		{"Pair[string, int]", generic("Pair[string, int]", "Pair[string, int]")},
		{"pg.Page[Address]", generic("pg.Page[Address]", "pg.Page[Address]")},
		{"AddrPage", generic("AddrPage", "Page[Address]")},
		{"*AddrPage", generic("*AddrPage", "Page[Address]")},
		{"AliasPage", generic("AliasPage", "Page[Address]")},
		{"pg.AddrPage", generic("pg.AddrPage", "Page[Addr]")},
		{"MPage", generic("MPage", "Page[Address]")},
		{"*MPage", generic("*MPage", "Page[Address]")},
		{"UPage", generic("UPage", "Page[Address]")},
		{"TPage", generic("TPage", "Page[Address]")},
		{"AliasM", generic("AliasM", "Page[Address]")},
		{"pg.MPage", generic("pg.MPage", "Page[Addr]")},
		{"GA[Address]", generic("GA[Address]", "GA[Address]")},
		{"GD[Address]", generic("GD[Address]", "GD[Address]")},
		{"GP[Address]", generic("GP[Address]", "GP[Address]")},
		{"GParen[Address]", generic("GParen[Address]", "GParen[Address]")},
		{"Cyc1[Address]", generic("Cyc1[Address]", "Cyc1[Address]")},
		{"mo.Option[Address]", generic("mo.Option[Address]", "mo.Option[Address]")},
		{"Undeclared[Address]", generic("Undeclared[Address]", "Undeclared[Address]")},
		{"List[Address]", f26("List[Address]")},
		{"*List[Address]", f26("*List[Address]")},
		{"pg.List[Address]", f26("pg.List[Address]")},
		{"GL[Address]", f26("GL[Address]")},
		{"GPL[Address]", f26("GPL[Address]")},
		{"ID[int]", f26("ID[int]")},
		{"Opt[Address]", f26("Opt[Address]")},
		{"AL", f26("AL")},
		{"ML", f26("ML")},
		{"LP", f26("LP")},
		{"[]Address", f26("[]Address")},
		{"AddrSlice", f26("AddrSlice")},
		{"Status", f26("Status")},
		{"PP", f26("PP")},
		{"PA", f26("PA")},
		{"PtrBox[Address]", f26("PtrBox[Address]")},
	}
}

// TestGenericRequestsWarnOnce pins table 3 and the generic bases it does not
// name (generic aliases and defined types over a struct, a generic interface,
// a generic named string): one warning per route, no requestBody, and an
// untyped request, in either parameter position.
func TestGenericRequestsWarnOnce(t *testing.T) {
	for _, r := range genericRequestRows() {
		t.Run(r.typ, func(t *testing.T) {
			for _, params := range []string{"req " + r.typ + ", ctx server.HandlerContext", "ctx server.HandlerContext, req " + r.typ} {
				run := analyzeGenericRoute(t, params, "server.Result[Receipt]")
				assert.Equal(t, []string{r.want}, run.warnings, params)
				assertUntypedRequest(t, run.route.Request)
				require.NotNil(t, run.route.Response)
				assert.Equal(t, "Receipt", run.route.Response.Name)
				assertNoGenericComponent(t, run.a)
			}
		})
	}
}

// genericPayloadRows are the payloads of table 4, each with the one warning
// it raises: the full payload type as written, then the instantiation.
func genericPayloadRows() []struct{ result, written, inst string } {
	const page = "Page[Address]"
	return []struct{ result, written, inst string }{
		{"server.Result[Page[Address]]", page, page},
		{"server.ResultWithMeta[Page[Address]]", page, page},
		{"server.Result[[]Page[Address]]", "[]Page[Address]", page},
		{"server.Result[*Page[Address]]", "*Page[Address]", page},
		{"server.Result[map[string]Page[Address]]", "map[string]Page[Address]", page},
		{"server.Result[List[Address]]", "List[Address]", "List[Address]"},
		{"server.Result[Box[Address]]", "Box[Address]", "Box[Address]"},
		{"server.Result[Pair[string, int]]", "Pair[string, int]", "Pair[string, int]"},
		{"server.Result[pg.Page[Address]]", "pg.Page[Address]", "pg.Page[Address]"},
		{"server.Result[mo.Option[Address]]", "mo.Option[Address]", "mo.Option[Address]"},
		{"server.Result[Box[Page[Address]]]", "Box[Page[Address]]", "Box[Page[Address]]"},
		{page, page, page},
		{"*Page[Address]", "*Page[Address]", page},
		{"Pair[string, int]", "Pair[string, int]", "Pair[string, int]"},
		{"List[Address]", "List[Address]", "List[Address]"},
		{"server.Result[AddrPage]", "AddrPage", page},
		{"server.Result[*AddrPage]", "*AddrPage", page},
		{"server.Result[AliasPage]", "AliasPage", page},
		{"server.Result[LP]", "LP", page},
		{"server.Result[[]AddrPage]", "[]AddrPage", page},
		{"server.Result[map[string]AddrPage]", "map[string]AddrPage", page},
		{"server.Result[pg.AddrPage]", "pg.AddrPage", "Page[Addr]"},
	}
}

// TestGenericPayloadsWarnOnce pins table 4: on a typed-request route, each
// generic payload raises exactly one warning naming the full payload type
// (a bare return in full, never its first type argument), never the named
// non-struct one, and its data stays untyped: no Resolution and no name.
func TestGenericPayloadsWarnOnce(t *testing.T) {
	for _, r := range genericPayloadRows() {
		t.Run(r.result, func(t *testing.T) {
			run := analyzeGenericRoute(t, "req IDReq, ctx server.HandlerContext", r.result)
			assert.Equal(t, []string{fmt.Sprintf(genericPayloadWarning, r.written, r.inst)}, run.warnings)
			if run.route.Response != nil {
				assert.Empty(t, run.route.Response.Name)
				assert.Nil(t, run.route.Response.Resolution)
			}
			require.NotNil(t, run.route.Request)
			assert.Equal(t, "IDReq", run.route.Request.Name)
			assertNoGenericComponent(t, run.a)
		})
	}
	run := analyzeGenericRoute(t, "req IDReq, ctx server.HandlerContext", "Pair[string, int]")
	assert.Equal(t, []string{"response type Pair[string, int]: Pair[string, int] is a generic type instantiation, " +
		"which is not resolved — emitting an untyped schema"}, run.warnings)
	run = analyzeGenericRoute(t, "req AddrPage, ctx server.HandlerContext", "server.Result[Receipt]")
	assert.Equal(t, []string{"request type AddrPage: Page[Address] is a generic type instantiation, which is not resolved " +
		"— no requestBody emitted"}, run.warnings)
}

// TestGenericOnlyHandlersAreFound pins the status change issue 138 accepts:
// a handler with a generic request parameter, whatever its payload, or a
// request-less handler whose payload holds an unresolved instantiation, is
// found, so a request-less generic payload warns, a generic request with a
// generic payload raises one warning each, and a generic request with an
// anonymous struct payload raises the request one. The generic_status
// fixture's handlers document server.Created and server.NewNotFoundError.
func TestGenericOnlyHandlersAreFound(t *testing.T) {
	run := analyzeGenericRoute(t, "ctx server.HandlerContext", "server.Result[Page[Address]]")
	assert.Equal(t, []string{fmt.Sprintf(genericPayloadWarning, "Page[Address]", "Page[Address]")}, run.warnings)
	assert.Nil(t, run.route.Request)
	assert.Nil(t, run.route.Response)

	run = analyzeGenericRoute(t, "req Box[Address], ctx server.HandlerContext", "server.Result[Page[Address]]")
	assert.Equal(t, []string{
		fmt.Sprintf(genericRequestWarning, "Box[Address]", "Box[Address]"),
		fmt.Sprintf(genericPayloadWarning, "Page[Address]", "Page[Address]"),
	}, run.warnings)

	run = analyzeGenericRoute(t, "req Box[Address], ctx server.HandlerContext", "server.Result[struct{ A int }]")
	assert.Equal(t, []string{fmt.Sprintf(genericRequestWarning, "Box[Address]", "Box[Address]")}, run.warnings)
	assert.Nil(t, run.route.Response)

	a := New(filepath.Join("..", "spectest", "testdata", "generic_status"))
	project, err := a.AnalyzeProject()
	require.NoError(t, err)
	require.Len(t, project.Modules, 1)
	routes := project.Modules[0].Routes
	require.Len(t, routes, 3)
	for _, r := range routes {
		assert.Equal(t, 201, r.SuccessStatus, r.Path)
		assert.Equal(t, []int{404}, r.ErrorStatuses, r.Path)
		assert.Nil(t, r.Request, r.Path)
		assert.Nil(t, r.Response, r.Path)
	}
}

// TestPointerResultWrapperIsNoInstantiation pins that a pointer to a result
// wrapper (*server.Result[T], *srv.ResultWithMeta[T]), which payloadExpr does
// not unwrap, is not taken for an unresolved instantiation of the project's:
// a request-less handler returning one raises no generic payload warning and
// stays unfound, so its route keeps no inferred error statuses, as before
// issue 138.
func TestPointerResultWrapperIsNoInstantiation(t *testing.T) {
	for _, result := range []string{"*server.Result[Receipt]", "*server.Result[Page[Address]]", "*server.ResultWithMeta[Receipt]"} {
		src := strings.NewReplacer("PARAMS", "ctx server.HandlerContext", "RESULT", result,
			"return out, nil", `return nil, server.NewNotFoundError("x")`).Replace(genericModSrc)
		a, routes := analyzeProjectRoutes(t, map[string]string{
			"go.mod":                           resolveGoMod,
			filepath.Join("pg", "pg.go"):       genericPgSrc,
			filepath.FromSlash(genericModFile): src,
		})
		require.Len(t, routes, 1)
		assert.Empty(t, a.Warnings(t.Context()), result)
		assert.Empty(t, routes[0].ErrorStatuses, result)
		assert.Nil(t, routes[0].Response, result)
	}
}

// TestTypeShapeInstantiation pins the decoder: an instantiation decodes at any
// depth with its generic type as written, its argument Shapes and its text,
// and never as a named type; a base that is no type name stays unknown.
func TestTypeShapeInstantiation(t *testing.T) {
	a := New("")
	cases := map[string]string{
		"G[A]":                "inst:G[A]",
		"G[A, B]":             "inst:G[A, B]",
		"G[A,B]":              "inst:G[A, B]",
		"q.G[A]":              "inst:q.G[A]",
		"*G[A]":               "*inst:G[A]",
		"[]G[A]":              "[]inst:G[A]",
		"map[string]G[int]":   "map[string]inst:G[int]",
		"G[H[A]]":             "inst:G[H[A]]",
		"pkg.sub.G[A]":        "unknown",
		"G[A][B]":             "unknown",
		"string[int]":         "unknown",
		"G[[]int, *q.T, any]": "inst:G[[]int, *q.T, any]",
	}
	for src, want := range cases {
		assert.Equal(t, want, renderShape(a.typeShape(mustParse(t, src))), src)
	}
	s := a.typeShape(mustParse(t, "q.G[H[A], []int]"))
	require.Equal(t, models.ShapeInstantiation, s.Kind)
	assert.Empty(t, s.Name, "an Instantiation never reads as its bare generic type")
	require.NotNil(t, s.Inst)
	assert.Equal(t, "q.G", s.Inst.Base)
	assert.Equal(t, "q.G[H[A], []int]", s.Inst.Written)
	require.Len(t, s.Inst.Args, 2)
	assert.Equal(t, "inst:H[A]", renderShape(s.Inst.Args[0]))
	assert.Equal(t, "H", s.Inst.Args[0].Inst.Base)
	assert.Equal(t, "[]int", renderShape(s.Inst.Args[1]))
	for _, src := range []string{"G[A]", "*G[A]", "[]G[A]", "q.G[A]"} {
		assert.Empty(t, shapeBaseName(a.typeShape(mustParse(t, src))), src)
	}
}

// TestFirstInstantiation pins the walker the payload warning names its
// instantiation by: the outermost one on the element path, never a map key.
func TestFirstInstantiation(t *testing.T) {
	a := New("")
	cases := map[string]string{
		"G[A]": "G[A]", "*[]map[string]G[H[A]]": "G[H[A]]", "map[G[A]]string": "", "[]Address": "", "chan G[A]": "",
	}
	for src, want := range cases {
		s := a.typeShape(mustParse(t, src))
		assert.Equal(t, want, firstInstantiation(&s), src)
	}
}

// TestMergeInstantiationDecls pins that two build-tagged declarations merge
// only when they are one instantiation as written.
func TestMergeInstantiationDecls(t *testing.T) {
	a := New("")
	page := a.typeShape(mustParse(t, "Page[A]"))
	_, ok := mergeShapes(page, a.typeShape(mustParse(t, "Page[A]")))
	assert.True(t, ok)
	_, ok = mergeShapes(page, a.typeShape(mustParse(t, "Box[A]")))
	assert.False(t, ok)
}

// TestPayloadInstantiationNoResults pins that a handler with no results has
// no payload instantiation.
func TestPayloadInstantiationNoResults(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\nfunc h() {}\nfunc g() (server.Result[G[A]], error) { return }\n", 0)
	require.NoError(t, err)
	a := New("")
	assert.Empty(t, a.payloadInstantiation(f.Decls[0].(*ast.FuncDecl).Type.Results, nil))
	assert.Equal(t, "G[A]", a.payloadInstantiation(f.Decls[1].(*ast.FuncDecl).Type.Results, nil))
}
