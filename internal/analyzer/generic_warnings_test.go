package analyzer

import (
	"cmp"
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
	warnings := a.Warnings()
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

// genericFieldRows are the body fields of Fields (issue 114): Go name and
// the expected rendered Resolution. A generic struct is a ref to its
// Instantiation's component, a generic named composite is inlined, and the
// leftovers (an out-of-module or undeclared generic type) stay unresolved.
var genericFieldRows = []struct{ name, want string }{
	{"L", "[]string"},
	{"B", "$Box-int"},
	{"P", "$Page-Address"},
	{"PR", "$Pair-string-int"},
	{"AP", "$AddrPage"},
	{"AAP", "$Page-Address"},
	{"ALF", "[]$Address"},
	{"JB", "marshal:JBox[int](unknown)"},
	{"TB", "text:TBox[int]"},
	{"TR", "map[string]cycle:Tree[int]"},
	{"PGP", "$PgPage-Address"},
	{"PGL", "[]$Address"},
	{"MO", "inst:mo.Option[Address]"},
	{"UD", "inst:Undeclared[Address]"},
	{"PB", "*$Box-Address"},
	{"SB", "[]$Box-Address"},
	{"LPF", "[]$Page-Address"},
	{"MB", "map[string]$Box-int"},
	{"NB", "$Box-Page-Address"},
	{"Page", "$Page-Address"},
	{"pg.List", "*[]$Address"},
}

// TestGenericBodyFieldsResolve pins issue 114's body fields: each generic
// field documents its substituted type, and only the leftovers warn: the
// Marshaler JBox[int] (its MarshalJSON is on a generic receiver), the
// recursive Tree[int], and the two generic types the tool cannot read, which
// keep issue 138's warning. Fields is both the request body and the payload,
// and still warns once per field. A json-tagged generic embed is a field
// named by its generic type as written, never "unknown".
func TestGenericBodyFieldsResolve(t *testing.T) {
	run := analyzeGenericRoute(t, "req Fields, ctx server.HandlerContext", "server.Result[Fields]")
	a, route, warnings, src := run.a, run.route, run.warnings, run.src
	require.NotNil(t, route.Request)
	assert.Equal(t, "Fields", route.Request.Name)
	byName := map[string]*models.FieldInfo{}
	for i := range route.Request.Fields {
		byName[route.Request.Fields[i].Name] = &route.Request.Fields[i]
	}
	for _, r := range genericFieldRows {
		if assert.Contains(t, byName, r.name) {
			assert.Equal(t, r.want, renderShape(byName[r.name].ResolvedShape()), r.name)
		}
	}
	assert.Equal(t, []string{
		"field JB at " + genericLoc(t, src, "JB  JBox") + " has type JBox[int]: JBox has its own MarshalJSON method, which " +
			"encoding/json uses instead of its underlying type — emitting an untyped schema ({}); parameters are typed by kind and unaffected",
		"field TR at " + genericLoc(t, src, "TR  Tree") + " has type Tree[int]: Tree[int] contains itself — the recursion is cut to an untyped schema ({})",
		"field MO at " + genericLoc(t, src, "MO  mo.") + genericFieldTail("mo.Option[Address]", "mo.Option[Address]"),
		"field UD at " + genericLoc(t, src, "UD  Undeclared") + genericFieldTail("Undeclared[Address]", "Undeclared[Address]"),
	}, warnings)
	for _, w := range warnings {
		assert.NotContains(t, w, "field unknown at")
	}
	assert.Equal(t, "page", byName["Page"].JSONName)
	assert.Equal(t, "pgEmbed", byName["pg.List"].JSONName)
	for _, name := range []string{"Box-int", "Box-Address", "Page-Address", "Pair-string-int", "PgPage-Address", "Box-Page-Address", "AddrPage"} {
		assert.Contains(t, a.typeRegistry, name)
	}
	assertNoGenericComponent(t, a)
}

// assertNoGenericComponent asserts no generic base was registered as a
// component under its bare name: an Instantiation never reaches a consumer
// as its bare base.
func assertNoGenericComponent(t *testing.T, a *ProjectAnalyzer) {
	t.Helper()
	for _, name := range []string{"Page", "Box", "Pair", "List", "JBox", "TBox", "Tree", "PgPage", "PgList"} {
		assert.NotContains(t, a.typeRegistry, name)
	}
}

// TestGenericParamFieldsResolve pins issue 114's parameter rows: a generic
// path, query or header parameter documents its substituted kind (query
// List[string] is an array of string, path ID[int] a string), with no
// warning, and the binding rule (#121) applies to the substituted type: a
// query Box[int] is a struct go-bricks cannot bind.
func TestGenericParamFieldsResolve(t *testing.T) {
	run := analyzeGenericRoute(t, "req Params, ctx server.HandlerContext", "server.Result[Receipt]")
	route := run.route
	require.NotNil(t, route.Request)
	want := map[string]string{"IDs": "[]string", "ID": goTypeString, "Tags": "[]string", "BQ": "$Box-int"}
	require.Len(t, route.Request.Fields, len(want))
	for i := range route.Request.Fields {
		f := &route.Request.Fields[i]
		assert.NotEmpty(t, f.ParamType, f.Name)
		assert.Equal(t, want[f.Name], renderShape(f.ResolvedShape()), f.Name)
	}
	if assert.Len(t, run.warnings, 1, "%v", run.warnings) {
		assert.Contains(t, run.warnings[0], `field BQ at `)
		assert.Contains(t, run.warnings[0], `is the query parameter "b" of type Box[int], which go-bricks cannot bind (a struct other than time.Time)`)
	}
}

// TestGenericControls pins the shapes around issue 114's: an untagged
// generic struct embed promotes its substituted fields with no warning
// (issue 116's interim dropped-embed warning no longer fires), an untagged
// named slice of an instantiation is a property, while an undeclared bare
// type, a recursive map, anonymous struct and json:"-" fields keep what they
// had, and chan and func fields raise issue 158's func-or-chan warning.
func TestGenericControls(t *testing.T) {
	run := analyzeGenericRoute(t, "req IDReq, ctx server.HandlerContext", "server.Result[Controls]")
	a, warnings := run.a, run.warnings
	assert.Len(t, warnings, 4, "%v", warnings)
	assert.Empty(t, embedWarnings(a))
	controls := renders(t, a.typeRegistry["Controls"])
	assert.Equal(t, "[]$Address", controls["items"])
	assert.Equal(t, goTypeInt, controls["total"])
	assert.Equal(t, "[]$Page-Address", controls["LP"])
	if w := fieldWarnings(a, "X"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "T resolves to no schema", "a type parameter is bound only inside its own declaration")
	}
	if w := fieldWarnings(a, "NT"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "contains itself")
	}
	for name, held := range map[string]string{"Ch": "chan", "Fn": "func"} {
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], "which holds a "+held+", a Go type encoding/json cannot marshal", name)
		}
	}
	for _, name := range []string{"Anon", "Skip", "Note", "LP"} {
		assert.Empty(t, fieldWarnings(a, name), name)
	}
}

// TestGenericEmbedMalformedTagKeepsUnknown pins that a json-tagged generic
// embed with a tag reflect cannot fully read keeps "unknown" in the
// malformed-tag warning, while its field is named by its generic type and
// resolves to its Instantiation's component.
func TestGenericEmbedMalformedTagKeepsUnknown(t *testing.T) {
	src := strings.Replace(genericModSrc, "type Params struct {",
		"type Odd struct {\n\tBox[int] \"json:\\\"box\\\"\\n\\tvalidate:\\\"required\\\"\"\n}\n\ntype Params struct {", 1)
	src = strings.NewReplacer("PARAMS", "req Odd, ctx server.HandlerContext", "RESULT", "server.Result[Receipt]").Replace(src)
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("pg", "pg.go"):       genericPgSrc,
		filepath.FromSlash(genericModFile): src,
	})
	warnings := a.Warnings()
	require.Len(t, warnings, 1, "%v", warnings)
	assert.Len(t, matching(warnings, "struct tag on field unknown at "), 1)
	require.Len(t, routes, 1)
	require.NotNil(t, routes[0].Request)
	box := fieldByJSON(t, a.typeRegistry["Odd"], "box")
	assert.Equal(t, "Box", box.Name)
	assert.Equal(t, "$Box-int", renderShape(box.ResolvedShape()))
}

// genericRequestRows are the request types of table 3 and what each
// documents (issue 114): the component its requestBody $refs, or the one
// warning it raises. A struct Instantiation, directly, behind a pointer,
// through an alias, a generic alias or a generic defined type, is its
// component; a defined type over one (AddrPage) its own. A Marshaler or
// text type declared over a struct Instantiation (MPage, UPage, TPage) is a
// struct Marshaler request (#111). One over a non-struct generic type stays
// F26, and a generic type the tool cannot read, or one cut by a cycle,
// keeps issue 138's generic request warning.
func genericRequestRows() []struct{ typ, name, want string } {
	generic := func(typ, inst string) string { return fmt.Sprintf(genericRequestWarning, typ, inst) }
	f26 := func(typ string) string { return fmt.Sprintf(nonStructRequestWarning, typ) }
	marshaler := func(typ, owner, method string) string {
		return fmt.Sprintf(marshalerRequestWarning, typ, owner, method)
	}
	return []struct{ typ, name, want string }{
		{"Box[Address]", "Box-Address", ""},
		{"*Box[Address]", "Box-Address", ""},
		{"Pair[string, int]", "Pair-string-int", ""},
		{"pg.Page[Address]", "Page-Address", ""},
		{"AddrPage", "AddrPage", ""},
		{"*AddrPage", "AddrPage", ""},
		{"AliasPage", "Page-Address", ""},
		{"pg.AddrPage", "Page-Addr", ""},
		{"GA[Address]", "Box-Address", ""},
		{"GD[Address]", "Box-Address", ""},
		{"GP[Address]", "Page-Address", ""},
		{"GParen[Address]", "Box-Address", ""},
		{"MPage", "", marshaler("MPage", "MPage", "MarshalJSON")},
		{"*MPage", "", marshaler("*MPage", "MPage", "MarshalJSON")},
		{"UPage", "", marshaler("UPage", "UPage", "UnmarshalJSON")},
		{"TPage", "", ""},
		{"AliasM", "", marshaler("AliasM", "MPage", "MarshalJSON")},
		{"pg.MPage", "", marshaler("pg.MPage", "pg.MPage", "MarshalJSON")},
		{"Cyc1[Address]", "", generic("Cyc1[Address]", "Cyc1[Address]")},
		{"mo.Option[Address]", "", generic("mo.Option[Address]", "mo.Option[Address]")},
		{"Undeclared[Address]", "", generic("Undeclared[Address]", "Undeclared[Address]")},
		{"List[Address]", "", f26("List[Address]")},
		{"*List[Address]", "", f26("*List[Address]")},
		{"pg.List[Address]", "", f26("pg.List[Address]")},
		{"GL[Address]", "", f26("GL[Address]")},
		{"GPL[Address]", "", f26("GPL[Address]")},
		{"ID[int]", "", f26("ID[int]")},
		{"Opt[Address]", "", f26("Opt[Address]")},
		{"AL", "", f26("AL")},
		{"ML", "", f26("ML")},
		{"LP", "", f26("LP")},
		{"[]Address", "", f26("[]Address")},
		{"AddrSlice", "", f26("AddrSlice")},
		{"Status", "", f26("Status")},
		{"PP", "", f26("PP")},
		{"PA", "", f26("PA")},
		{"PtrBox[Address]", "", f26("PtrBox[Address]")},
	}
}

// TestGenericRequests pins table 3 in either parameter position: a request
// that resolves to a struct Instantiation adopts its component (named, with
// its substituted fields) and warns nothing; any other raises its one
// warning and documents no requestBody of its own.
func TestGenericRequests(t *testing.T) {
	for _, r := range genericRequestRows() {
		t.Run(r.typ, func(t *testing.T) {
			for _, params := range []string{"req " + r.typ + ", ctx server.HandlerContext", "ctx server.HandlerContext, req " + r.typ} {
				run := analyzeGenericRoute(t, params, "server.Result[Receipt]")
				if r.want == "" {
					assert.Empty(t, run.warnings, params)
				} else {
					assert.Equal(t, []string{r.want}, run.warnings, params)
				}
				if r.name != "" {
					require.NotNil(t, run.route.Request, params)
					assert.Equal(t, r.name, run.route.Request.Name, params)
					assert.NotEmpty(t, run.route.Request.Fields, params)
					assert.Contains(t, run.a.typeRegistry, r.name)
				} else if run.route.Request != nil {
					assert.Empty(t, run.route.Request.Name, params)
				}
				require.NotNil(t, run.route.Response)
				assert.Equal(t, "Receipt", run.route.Response.Name)
				assertNoGenericComponent(t, run.a)
			}
		})
	}
}

// genericPayloadRows are the payloads of table 4, each with the Resolution
// it documents (issue 114: a payload emits what a field of its type emits)
// or, for a leftover, issue 138's warning, naming the payload type as
// written, then the instantiation. A registered name (AddrPage) has no
// Resolution: it is $ref'd as any registered struct.
func genericPayloadRows() []struct{ result, name, res, warn string } {
	const page = "$Page-Address"
	return []struct{ result, name, res, warn string }{
		{"server.Result[Page[Address]]", "", page, ""},
		{"server.ResultWithMeta[Page[Address]]", "", page, ""},
		{"server.Result[[]Page[Address]]", "", "[]" + page, ""},
		{"server.Result[*Page[Address]]", "", "*" + page, ""},
		{"server.Result[map[string]Page[Address]]", "", "map[string]" + page, ""},
		{"server.Result[List[Address]]", "", "[]$Address", ""},
		{"server.Result[Box[Address]]", "", "$Box-Address", ""},
		{"server.Result[Pair[string, int]]", "", "$Pair-string-int", ""},
		{"server.Result[pg.Page[Address]]", "", page, ""},
		{"server.Result[Box[Page[Address]]]", "", "$Box-Page-Address", ""},
		{"Page[Address]", "", page, ""},
		{"*Page[Address]", "", "*" + page, ""},
		{"Pair[string, int]", "", "$Pair-string-int", ""},
		{"List[Address]", "", "[]$Address", ""},
		{"server.Result[AddrPage]", "AddrPage", "", ""},
		{"server.Result[*AddrPage]", "AddrPage", "", ""},
		{"server.Result[AliasPage]", "", page, ""},
		{"server.Result[LP]", "", "[]" + page, ""},
		{"server.Result[[]AddrPage]", "AddrPage", "", ""},
		{"server.Result[map[string]AddrPage]", "", "map[string]$AddrPage", ""},
		{"server.Result[pg.AddrPage]", "", "$Page-Addr", ""},
		{"server.Result[mo.Option[Address]]", "", "", fmt.Sprintf(genericPayloadWarning, "mo.Option[Address]", "mo.Option[Address]")},
		{"mo.Option[Address]", "", "", fmt.Sprintf(genericPayloadWarning, "mo.Option[Address]", "mo.Option[Address]")},
		{"server.Result[[]Undeclared[Address]]", "", "", fmt.Sprintf(genericPayloadWarning, "[]Undeclared[Address]", "Undeclared[Address]")},
	}
}

// TestGenericPayloads pins table 4 on a typed-request route: a payload
// holding an Instantiation the tool resolves is typed, with no warning; a
// leftover keeps issue 138's one warning and resolves to no TypeInfo.
func TestGenericPayloads(t *testing.T) {
	for _, r := range genericPayloadRows() {
		t.Run(r.result, func(t *testing.T) {
			run := analyzeGenericRoute(t, "req IDReq, ctx server.HandlerContext", r.result)
			require.NotNil(t, run.route.Request)
			assert.Equal(t, "IDReq", run.route.Request.Name)
			assertNoGenericComponent(t, run.a)
			if r.warn != "" {
				assert.Equal(t, []string{r.warn}, run.warnings)
				assert.Nil(t, run.route.Response)
				return
			}
			assert.Empty(t, run.warnings)
			require.NotNil(t, run.route.Response)
			assert.True(t, IsTypedPayload(run.route.Response), "doctor counts it typed")
			assert.Equal(t, r.name, run.route.Response.Name)
			assert.Equal(t, cmp.Or(r.res, "unknown"), renderShapePtr(run.route.Response.Resolution))
		})
	}
}

// TestGenericOnlyHandlersAreFound pins that a handler with a generic request
// parameter, whatever its payload, or a request-less handler whose payload
// holds an instantiation, is found: issue 138's status change, kept by issue
// 114. A request-less leftover payload warns; a generic request with a
// generic payload documents both. The generic_status fixture's handlers
// document server.Created and server.NewNotFoundError.
func TestGenericOnlyHandlersAreFound(t *testing.T) {
	run := analyzeGenericRoute(t, "ctx server.HandlerContext", "server.Result[mo.Option[Address]]")
	assert.Equal(t, []string{fmt.Sprintf(genericPayloadWarning, "mo.Option[Address]", "mo.Option[Address]")}, run.warnings)
	assert.Nil(t, run.route.Request)
	assert.Nil(t, run.route.Response)

	run = analyzeGenericRoute(t, "req Box[Address], ctx server.HandlerContext", "server.Result[Page[Address]]")
	assert.Empty(t, run.warnings)
	require.NotNil(t, run.route.Request)
	assert.Equal(t, "Box-Address", run.route.Request.Name)
	require.NotNil(t, run.route.Response)
	assert.Equal(t, "$Page-Address", renderShapePtr(run.route.Response.Resolution))

	run = analyzeGenericRoute(t, "req Box[Address], ctx server.HandlerContext", "server.Result[struct{ A int }]")
	assert.Empty(t, run.warnings)
	if assert.NotNil(t, run.route.Response) && assert.NotNil(t, run.route.Response.Resolution) {
		assert.Equal(t, "struct{A int}", renderShape(*run.route.Response.Resolution))
	}

	a := New(filepath.Join("..", "spectest", "testdata", "generic_status"))
	project, err := a.AnalyzeProject()
	require.NoError(t, err)
	require.Len(t, project.Modules, 1)
	routes := project.Modules[0].Routes
	require.Len(t, routes, 3)
	for _, r := range routes {
		assert.Equal(t, 201, r.SuccessStatus, r.Path)
		assert.Equal(t, []int{404}, r.ErrorStatuses, r.Path)
		assert.NotNil(t, r.Response, r.Path)
	}
	assert.Empty(t, a.Warnings())
}

// TestPointerResultWrapperRequestLess pins that a request-less handler
// returning a pointer to a result wrapper (*server.Result[T],
// *server.ResultWithMeta[T]) is found, since payloadExpr unwraps it as it
// does the value form (issue 143): its route keeps its inferred error
// statuses, a plain or generic payload is typed with no warning (issue 114),
// and a leftover generic payload raises the generic payload warning.
func TestPointerResultWrapperRequestLess(t *testing.T) {
	cases := map[string]string{
		"*server.Result[Receipt]":            "",
		"*server.ResultWithMeta[Receipt]":    "",
		"*server.Result[Page[Address]]":      "",
		"*server.Result[mo.Option[Address]]": fmt.Sprintf(genericPayloadWarning, "mo.Option[Address]", "mo.Option[Address]"),
	}
	for result, warning := range cases {
		src := strings.NewReplacer("PARAMS", "ctx server.HandlerContext", "RESULT", result,
			"return out, nil", `return nil, server.NewNotFoundError("x")`).Replace(genericModSrc)
		a, routes := analyzeProjectRoutes(t, map[string]string{
			"go.mod":                           resolveGoMod,
			filepath.Join("pg", "pg.go"):       genericPgSrc,
			filepath.FromSlash(genericModFile): src,
		})
		require.Len(t, routes, 1)
		assert.Equal(t, []int{404}, routes[0].ErrorStatuses, result)
		if warning == "" {
			assert.Empty(t, a.Warnings(), result)
			require.NotNil(t, routes[0].Response, result)
			assert.True(t, IsTypedPayload(routes[0].Response), result)
			continue
		}
		assert.Equal(t, []string{warning}, a.Warnings(), result)
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
