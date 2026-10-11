package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Issue 118: an anonymous struct field is documented inline, in
// every position, through the ordinary field path.

const anonImports = "\t\"time\"\n\t\"github.com/google/uuid\"\n\t\"github.com/shopspring/decimal\"\n"

// anonDecls declares the named types the anonymous struct rows reach.
const anonDecls = `
var _ decimal.Decimal
var _ time.Time
var _ uuid.UUID

type Home struct{ City string ` + "`json:\"city\"`" + ` }
type Office struct{ Floor int ` + "`json:\"floor\"`" + ` }
type Ack struct{}
type Local struct{ N int }

func (l Local) MarshalJSON() ([]byte, error) { return nil, nil }

type A struct {
	B struct {
		*A
		X int ` + "`json:\"x\"`" + `
	} ` + "`json:\"b\"`" + `
}

type L []struct {
	Next L ` + "`json:\"next\"`" + `
}
`

// anonRow is one Rows field of an anonymous struct type: its JSON name, Go
// type and expected rendered Resolution.
type anonRow struct{ json, goType, want string }

var anonRows = []anonRow{
	{"meta", "struct{ X int `json:\"x\"` }", "struct{x int}"},
	{"ptr", "*struct{ X int `json:\"x\"` }", "*struct{x int}"},
	{"list", "[]struct{ X int `json:\"x\"` }", "[]struct{x int}"},
	{"fixed", "[2]struct{ X int `json:\"x\"` }", "[N]struct{x int}"},
	{"byKey", "map[string]struct{ X int `json:\"x\"` }", "map[string]struct{x int}"},
	{"nested", "struct{ In struct{ Y string `json:\"y\"` } `json:\"in\"` }", "struct{in struct{y string}}"},
	{"promoted", "struct{ Home; Z int `json:\"z\"` }", "struct{z int; city string}"},
	{"office", "struct{ H Office `json:\"h\"`; P *Office `json:\"p\"` }", "struct{h $Office; p *$Office}"},
	{"skips", "struct{ A int `json:\"a,omitempty\"`; B int `json:\"-\"`; c int; D int }", "struct{a int; B int; D int}"},
	{"stamp", "struct{ time.Time; Z int }", "marshal:struct{…}(unknown)"},
	{"id", "struct{ uuid.UUID }", "text:struct{…}"},
	{"local", "struct{ *Local }", "marshal:struct{…}(unknown)"},
	{"dec", "struct{ D decimal.Decimal `json:\"d\"` }", "struct{d decimal.Decimal}"},
	{"addr", "struct{ P uintptr `json:\"p\"` }", "struct{p uintptr}"},
	{"empty", "struct{}", "struct{}"},
	{"a", "A", "$A"},
	{"l", "L", "[]struct{next []cycle:struct{…}}"},
}

// anonRowsBody renders rows as Rows struct fields.
func anonRowsBody(rows []anonRow) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "\t%s %s `json:%q`\n", exportName(r.json), r.goType, r.json)
	}
	return b.String()
}

// anonLoc is the position of the first line of the module source holding
// needle, rendered as a field warning renders it (OS path separators).
func anonLoc(t *testing.T, src, needle string) string {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if col := strings.Index(line, needle); col >= 0 {
			return fmt.Sprintf("%s:%d:%d", filepath.FromSlash("mod/module.go"), i+1, col+1)
		}
	}
	require.Failf(t, "needle not found", "%q", needle)
	return ""
}

// TestTypeShapeAnonymousStruct pins the decoder: a struct literal, at any
// depth, is a ShapeStruct holding its literal and no fields.
func TestTypeShapeAnonymousStruct(t *testing.T) {
	a := New("")
	leaf := func(s models.TypeShape) models.TypeShape {
		for s.Elem != nil {
			s = *s.Elem
		}
		return s
	}
	for _, src := range []string{
		"struct{ A int }", "*struct{ A int }", "[]struct{ A int }", "[2]struct{ A int }",
		"map[string]struct{ A int }", "[]map[string]*struct{ A int }",
	} {
		s := leaf(a.typeShape(mustParse(t, src)))
		require.Equal(t, models.ShapeStruct, s.Kind, src)
		require.NotNil(t, s.Struct, src)
		assert.NotNil(t, s.Struct.Literal, src)
		assert.Nil(t, s.Struct.Fields, src)
	}
	nested := a.typeShape(mustParse(t, "struct{ In struct{ A int } }"))
	assert.Nil(t, nested.Struct.Fields, "a nested literal is not decoded into fields either")
}

// TestAnonymousStructFieldResolution pins every row's Resolution, and the
// warnings the anonymous structs' own fields raise.
func TestAnonymousStructFieldResolution(t *testing.T) {
	src := rowsModule(anonImports, anonDecls, anonRowsBody(anonRows))
	a, _ := analyzeSingleModule(t, src)
	got := renders(t, a.typeRegistry["Rows"])
	for _, r := range anonRows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}

	// Home is promoted, so it is no component; Office and A are.
	assert.NotContains(t, a.typeRegistry, "Home")
	assert.Contains(t, a.typeRegistry, "Office")
	a2 := a.typeRegistry["A"]
	require.NotNil(t, a2)
	assert.Equal(t, map[string]string{"b": "struct{x int; b cycle:struct{…}}"}, renders(t, a2), "cut one level down")

	want := []string{
		fmt.Sprintf(marshalerPromotedFieldWarning, "Stamp", anonLoc(t, src, "Stamp struct"), "struct{time.Time; Z int}",
			anonStructDisplay, methodMarshalJSON, "time.Time"),
		fmt.Sprintf(marshalerPromotedFieldWarning, "Local", anonLoc(t, src, "Local struct{ *"), "struct{*Local}",
			anonStructDisplay, methodMarshalJSON, "Local"),
		fmt.Sprintf(unresolvableFieldWarning, "D", anonLoc(t, src, "D decimal.Decimal"), "decimal.Decimal", "decimal.Decimal"),
		"field P at " + anonLoc(t, src, "P uintptr") + " holds a uintptr, a machine address with no meaningful API contract " +
			"— emitting an untyped schema (use a sized integer type, or exclude the field with json:\"-\")",
		fmt.Sprintf(recursiveFieldWarning, "B", anonLoc(t, src, "B struct {"), "struct{*A; X int}", anonStructDisplay),
		fmt.Sprintf(recursiveFieldWarning, "Next", anonLoc(t, src, "Next L"), "L", anonStructDisplay),
	}
	assert.ElementsMatch(t, want, a.Warnings(t.Context()))
}

// TestAnonymousStructInnerFieldInfo pins that an anonymous struct's fields
// carry what a named struct's carry: tags, validate constraints, and the
// json:"-" and omitempty handling.
func TestAnonymousStructInnerFieldInfo(t *testing.T) {
	a, _ := analyzeRows(t, "", "", "\tMeta struct{ X int `json:\"x\" validate:\"min=1\"`; "+
		"Tag string `json:\"tag,omitempty\" validate:\"required,max=8\"`; Skip int `json:\"-\"` } `json:\"meta\" validate:\"required\"`\n")
	meta := fieldByJSON(t, a.typeRegistry["Rows"], "meta")
	assert.True(t, meta.Required)
	r := meta.ResolvedShape()
	require.Equal(t, models.ShapeStruct, r.Kind)
	inner := &models.TypeInfo{Name: "Meta", Fields: r.Struct.Fields}
	x := fieldByJSON(t, inner, "x")
	assert.Equal(t, map[string]string{"min": "1"}, x.Constraints)
	tag := fieldByJSON(t, inner, "tag")
	assert.True(t, tag.Required)
	assert.Equal(t, "8", tag.Constraints["max"])
	assert.Nil(t, fieldByJSON(t, inner, "-").Resolution, "a json:\"-\" field is not resolved")
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestAnonymousStructUnextracted pins two places a literal stays a bare
// struct{…}: a parameter (bound by kind), and the quiet run of a Marshaler
// type's underlying form, which raises no dropped-embed warning. A struct
// Marshaler request's body field is a third (TestMarshalerRequestLiteralUnextracted).
func TestAnonymousStructUnextracted(t *testing.T) {
	decls := "type Box[T any] struct{ V T }\n\ntype M []struct {\n\tBox[int]\n\tX int\n}\n\n" +
		"func (m M) MarshalJSON() ([]byte, error) { return nil, nil }\n"
	rows := "\tQ struct{ A int `json:\"a\"` } `query:\"q\"`\n\tM M `json:\"m\"`\n"
	src := rowsModule("", decls, rows)
	a, _ := analyzeSingleModule(t, src)
	ti := a.typeRegistry["Rows"]
	got := renders(t, ti)
	q := ti.Fields[0]
	require.Equal(t, "q", q.ParamName)
	assert.Equal(t, "struct{…}", renderShape(q.ResolvedShape()))
	assert.Equal(t, "marshal:M([]struct{…})", got["m"])
	assert.Equal(t, []string{
		fmt.Sprintf(marshalerFieldWarning, "M", anonLoc(t, src, "M M"), "M", "M", methodMarshalJSON),
	}, a.Warnings(t.Context()))
}

// TestAnonymousStructDepthCap pins that a literal shares the registration
// depth budget: extracted one level below its struct, it is cut to {} with
// the latched depth warning past maxTypeRegistrationDepth.
func TestAnonymousStructDepthCap(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "d.go", "package d\n\ntype T struct{ F struct{ X int } }\n")
	lit := typeSpecInFile(file, "T").Type.(*ast.StructType).Fields.List[0].Type
	shape := a.typeShape(lit)

	c := newResolveCtx(file, path, false)
	c.st.inline, c.st.depth = true, maxTypeRegistrationDepth-1
	assert.Equal(t, "struct{X int}", renderShape(a.resolveShape(shape, c)))
	assert.Empty(t, a.warnings)

	c.st.depth = maxTypeRegistrationDepth
	assert.Equal(t, "cycle:struct{…}", renderShape(a.resolveShape(shape, c)))
	require.Len(t, a.warnings, 1)
	assert.Contains(t, a.warnings[0], "registration depth limit")
	assert.Equal(t, fallbackNone, c.st.first.kind, "the depth cut notes nothing")
	assert.Same(t, shape.Struct, a.resolveShape(shape, newResolveCtx(file, path, false)).Struct,
		"outside a field's resolution the Shape's own literal is returned")
}

// TestAnonymousStructBuildTaggedVariants pins how build-tagged declarations
// of one named type over a literal merge: written alike, they merge to the
// first's extraction, unless one qualifier in them names two packages;
// written differently, or reaching two structs, they disagree, with a
// warning.
func TestAnonymousStructBuildTaggedVariants(t *testing.T) {
	tagged := func(tag, body string) string { return "//go:build " + tag + "\n\npackage mod\n\n" + body }
	same := "type Same []struct{ A int `json:\"a\"` }\n"
	imp := func(pkg, name string) string {
		return "import items \"github.com/example/app/" + pkg + "\"\n\ntype " + name + " []struct{ I items.Items `json:\"i\"` }\n"
	}
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":        resolveGoMod,
		"lx/items.go":   "package lx\n\ntype Items struct{ Linux int `json:\"linux\"` }\n",
		"wn/items.go":   "package wn\n\ntype Items struct{ Win string `json:\"win\"` }\n",
		"mod/same_a.go": tagged("linux", same),
		"mod/same_b.go": tagged("!linux", same),
		"mod/diff_a.go": tagged("linux", "type Diff []struct{ A int `json:\"a\"` }\n"),
		"mod/diff_b.go": tagged("!linux", "type Diff []struct{ A int `json:\"b\"` }\n"),
		"mod/one_a.go":  tagged("linux", imp("lx", "One")),
		"mod/one_b.go":  tagged("!linux", imp("lx", "One")),
		"mod/two_a.go":  tagged("linux", imp("lx", "Two")),
		"mod/two_b.go":  tagged("!linux", imp("wn", "Two")),
		"mod/module.go": rowsModule("", "", "\tSame Same `json:\"same\"`\n\tDiff Diff `json:\"diff\"`\n\tOne One `json:\"one\"`\n\tTwo Two `json:\"two\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "[]struct{a int}", got["same"])
	assert.Equal(t, "Diff", got["diff"])
	assert.Equal(t, "[]struct{i $Items}", got["one"], "one struct reached from two files is no clash")
	assert.Equal(t, "Two", got["two"])
	for _, name := range []string{"Diff", "Two"} {
		if w := fieldWarnings(a, name); assert.Len(t, w, 1) {
			assert.Contains(t, w[0], name+"'s build-tagged declarations disagree on shape")
		}
	}
	assert.Len(t, a.Warnings(t.Context()), 2)
}

// TestAnonymousStructBuildTaggedCycle pins that build-tagged declarations,
// written alike, of a named slice or map type over an anonymous struct that
// contains itself document what one declaration does. Inside the first
// declaration's literal, the type's own declarations resolve to that literal
// cut (it is open) and the second's literal extracted (it is not): the first,
// the cut, is kept, so no disagreement is raised, only the recursion warning.
func TestAnonymousStructBuildTaggedCycle(t *testing.T) {
	decls := func(l, m string) string {
		return "type " + l + " []struct {\n\tNext " + l + " `json:\"next\"`\n\tN int `json:\"n\"`\n}\n\n" +
			"type " + m + " map[string]struct {\n\tSub " + m + " `json:\"sub\"`\n\tK int `json:\"k\"`\n}\n"
	}
	tagged := func(tag string) string { return "//go:build " + tag + "\n\npackage mod\n\n" + decls("L", "M") }
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":        resolveGoMod,
		"mod/cyc_a.go":  tagged("linux"),
		"mod/cyc_b.go":  tagged("!linux"),
		"mod/one.go":    "package mod\n\n" + decls("L1", "M1"),
		"mod/module.go": rowsModule("", "", "\tL L `json:\"l\"`\n\tM M `json:\"m\"`\n\tL1 L1 `json:\"l1\"`\n\tM1 M1 `json:\"m1\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "[]struct{next []cycle:struct{…}; n int}", got["l1"], "one declaration")
	assert.Equal(t, got["l1"], got["l"], "two declarations written alike")
	assert.Equal(t, "map[string]struct{sub map[string]cycle:struct{…}; k int}", got["m1"], "one declaration")
	assert.Equal(t, got["m1"], got["m"], "two declarations written alike")
	for _, name := range []string{"Next", "Sub"} {
		if w := fieldWarnings(a, name); assert.Len(t, w, 2, name) {
			for _, msg := range w {
				assert.Contains(t, msg, anonStructDisplay+" contains itself", name)
			}
		}
	}
	assert.Len(t, a.Warnings(t.Context()), 4, "no disagreement")
}

// TestAnonymousStructBuildTaggedLimit pins the other asymmetric cut: inside an
// anonymous struct, the first declaration's literal uses up the
// maxLiteralExtractions limit, so the second's is cut. The first's extraction
// is kept, so the field documents what one declaration does, with the limit's
// warning and no disagreement.
func TestAnonymousStructBuildTaggedLimit(t *testing.T) {
	const n = 11 // 2047 literals
	var decls strings.Builder
	for i := 1; i < n; i++ {
		fmt.Fprintf(&decls, "type Q%d []struct{ A Q%d `json:\"a\"`; B Q%d `json:\"b\"` }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&decls, "type Q%d []struct{ X int `json:\"x\"` }\n", n)
	tagged := func(tag string) string {
		return "//go:build " + tag + "\n\npackage mod\n\ntype T []struct{ A Q1 `json:\"a\"` }\n"
	}
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":        resolveGoMod,
		"mod/t_a.go":    tagged("linux"),
		"mod/t_b.go":    tagged("!linux"),
		"mod/one.go":    "package mod\n\n" + decls.String() + "type T1 []struct{ A Q1 `json:\"a\"` }\n",
		"mod/module.go": rowsModule("", "", "\tW struct{ T T `json:\"t\"` } `json:\"w\"`\n\tW1 struct{ T T1 `json:\"t\"` } `json:\"w1\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, maxLiteralExtractions, extractedLiterals(map[string]string{"w1": got["w1"]}), "one declaration")
	assert.Equal(t, got["w1"], got["w"], "two declarations")
	assert.Equal(t, []string{fmt.Sprintf(literalLimitWarning, maxLiteralExtractions)}, a.Warnings(t.Context()))
}

// TestAnonymousStructUnextractedBuildTagged pins that build-tagged
// declarations of a named type over a literal, written differently, still
// merge where the literal is not extracted: a query parameter keeps its
// array type with no warning, and a payload keeps its untyped fallback and
// its named non-struct warning.
func TestAnonymousStructUnextractedBuildTagged(t *testing.T) {
	tagged := func(tag, name string) string {
		return "//go:build " + tag + "\n\npackage mod\n\ntype BD []struct{ A int `json:\"" + name + "\"` }\n"
	}
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":      resolveGoMod,
		"mod/bd_a.go": tagged("linux", "a"),
		"mod/bd_b.go": tagged("!linux", "a,omitempty"),
		"mod/module.go": resolveModuleHead + `
type ListReq struct {
	Modes BD ` + "`query:\"modes\"`" + `
}

func (m *Module) list(req ListReq, ctx server.HandlerContext) (server.Result[[]BD], server.IAPIError) {
	return server.Result[[]BD]{}, nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/list", m.list)
}
`,
	})
	req := a.typeRegistry["ListReq"]
	require.NotNil(t, req)
	require.Len(t, req.Fields, 1)
	assert.Equal(t, "modes", req.Fields[0].ParamName)
	assert.Equal(t, "[]struct{…}", renderShape(req.Fields[0].ResolvedShape()))
	resp := routeByPath(t, routes, "/list").Response
	require.NotNil(t, resp)
	assert.Nil(t, resp.Resolution)
	assert.Equal(t, []string{namedNonStructWarning("BD")}, a.Warnings(t.Context()))
}

// TestSameLiteral pins literalText's sense of "written alike": names, types
// and raw tags, nested literals' tags included, and the nil guard.
func TestSameLiteral(t *testing.T) {
	lit := func(src string) *models.AnonymousStruct {
		e, err := parser.ParseExpr(src)
		require.NoError(t, err)
		return &models.AnonymousStruct{Literal: e.(*ast.StructType)}
	}
	assert.True(t, sameLiteral(lit("struct{ A int `json:\"a\"` }"), lit("struct{A int `json:\"a\"`}")))
	assert.False(t, sameLiteral(lit("struct{ A int `json:\"a\"` }"), lit("struct{ A int `json:\"b\"` }")))
	assert.False(t, sameLiteral(lit("struct{ In struct{ A int `json:\"a\"` } }"), lit("struct{ In struct{ A int `json:\"b\"` } }")),
		"a nested literal's tags count")
	assert.False(t, sameLiteral(lit("struct{ A int `x`; B int }"), lit("struct{ A int; B int `x` }")), "a tag stays with its field")
	assert.False(t, sameLiteral(nil, lit("struct{}")))
	assert.False(t, sameLiteral(lit("struct{}"), &models.AnonymousStruct{}))
}

// TestAnonymousStructRegistersInnerRefs pins that a named struct reachable
// only through an anonymous struct registers, with its collision-qualified
// name stamped into the inline field's leaf, and that an empty one does too.
func TestAnonymousStructRegistersInnerRefs(t *testing.T) {
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":          resolveGoMod,
		"other/office.go": "package other\n\ntype Office struct{ Desk int `json:\"desk\"` }\n",
		"mod/module.go": rowsModule("\t\"github.com/example/app/other\"\n",
			"type Office struct{ Floor int `json:\"floor\"` }\ntype Ack struct{}\ntype Deep struct{ V int `json:\"v\"` }\n",
			"\tO Office `json:\"o\"`\n\tIn struct{ H other.Office `json:\"h\"`; A Ack `json:\"a\"`; N []struct{ D map[string]*Deep `json:\"d\"` } `json:\"n\"` } `json:\"in\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "struct{h $OtherOffice; a $Ack; n []struct{d map[string]*$Deep}}", got["in"])
	for _, name := range []string{"Office", "OtherOffice", "Ack", "Deep"} {
		assert.Contains(t, a.typeRegistry, name)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestAnonymousStructWarnsSurvivorsOnly pins that an anonymous struct is
// extracted only when the field holding it survives its struct's dominance
// rule (dropDominatedFields): a promoted one a shallower field of its key
// outranks never reaches the document, so neither its fields' warnings (an
// unresolvable type) nor its extraction's (a malformed tag, a dropped
// generic embed) fire, as for a dominated field of a named struct type.
func TestAnonymousStructWarnsSurvivorsOnly(t *testing.T) {
	decls := "var _ decimal.Decimal\n\ntype Gen[T any] struct{ V T }\n\ntype Inner struct {\n\tMeta struct {\n" +
		"\t\tD decimal.Decimal `json:\"d\"`\n\t\tBad int `bad json:\"b\"`\n\t\tGen[int]\n\t} `json:\"meta\"`\n}\n"
	a, _ := analyzeRows(t, "\t\"github.com/shopspring/decimal\"\n", decls, "\tInner\n\tMeta string `json:\"meta\"`\n")
	assert.Equal(t, map[string]string{"meta": "string"}, renders(t, a.typeRegistry["Rows"]))
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestAnonymousStructDominatedCycle pins that a field the dominance rule
// drops puts no anonymous struct on a cycle. TY's element promotes R, whose
// x is the open X literal, but the element's own x outranks it, so the
// element never contains itself: both y and z expand it, with no warning.
func TestAnonymousStructDominatedCycle(t *testing.T) {
	decls := "type TY []struct {\n\t*R\n\tX int `json:\"x\"`\n\tV int `json:\"v\"`\n}\n\n" +
		"type Q struct {\n\tY TY `json:\"y\"`\n\tZ TY `json:\"z\"`\n}\n\n" +
		"type R struct {\n\tX struct {\n\t\t*Q\n\t\tW int `json:\"w\"`\n\t} `json:\"x\"`\n}\n"
	a, _ := analyzeRows(t, "", decls, "\tR R `json:\"r\"`\n")
	elem := "[]struct{x int; v int}"
	assert.Equal(t, map[string]string{"x": "struct{w int; y " + elem + "; z " + elem + "}"}, renders(t, a.typeRegistry["R"]))
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestAnonymousStructInnerGenericField pins that an Instantiation inside an
// anonymous struct warns on the inner field, not on the field holding it.
func TestAnonymousStructInnerGenericField(t *testing.T) {
	src := rowsModule("", "type Box[T any] struct{ V T }\n", "\tW struct{ P Box[int] `json:\"p\"` } `json:\"w\"`\n")
	a, _ := analyzeSingleModule(t, src)
	assert.Equal(t, []string{
		fmt.Sprintf(genericFieldWarning, "P", anonLoc(t, src, "P Box[int]"), "Box[int]", "Box[int]"),
	}, a.Warnings(t.Context()))
}

// payloadOverLiteralModule is a module whose routes return and take named
// types over anonymous structs, which stay untyped in this part of issue 118.
const payloadOverLiteralModule = `package mod

import (
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

var _ time.Time

type Rows []struct{ A int }
type MRows []struct {
	time.Time
	Z int
}
type BadRows []struct {
	A int ` + "`json:\"a\"\tquery:\"q\"`" + `
}
type ReqRows []struct{ A int }

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) { return server.Result[Rows]{}, nil }
func (m *Module) mrows(ctx server.HandlerContext) (server.Result[MRows], server.IAPIError) {
	return server.Result[MRows]{}, nil
}
func (m *Module) bad(ctx server.HandlerContext) (server.Result[BadRows], server.IAPIError) {
	return server.Result[BadRows]{}, nil
}
func (m *Module) req(req ReqRows, ctx server.HandlerContext) (server.Result[int], server.IAPIError) {
	return server.Result[int]{}, nil
}
func (m *Module) list(ctx server.HandlerContext) (server.Result[[]struct{ A int }], server.IAPIError) {
	return server.Result[[]struct{ A int }]{}, nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
	server.GET(hr, r, "/mrows", m.mrows)
	server.GET(hr, r, "/bad", m.bad)
	server.POST(hr, r, "/req", m.req)
	server.GET(hr, r, "/list", m.list)
}
`

// TestPayloadOverLiteralStaysUntyped pins the seam of this part of issue
// 118: a payload or request rooted outside a field never extracts the
// anonymous struct it holds, so it keeps today's untyped fallback and
// warnings exactly — a Marshaler literal included, which would otherwise
// become a typed Marshaler leaf.
func TestPayloadOverLiteralStaysUntyped(t *testing.T) {
	a, routes := analyzeSingleModule(t, payloadOverLiteralModule)
	for _, path := range []string{"/rows", "/mrows", "/bad"} {
		r := routeByPath(t, routes, path)
		require.NotNil(t, r.Response, path)
		assert.Nil(t, r.Response.Resolution, path)
		assert.Empty(t, r.Response.Name, path)
	}
	assert.Nil(t, routeByPath(t, routes, "/list").Response, "a nameless composite over a literal stays nil")
	req := routeByPath(t, routes, "/req").Request
	require.NotNil(t, req)
	assert.Empty(t, req.Name)
	named := "request/response type %s is a named non-struct type — emitting an untyped schema (annotate or restructure it as a struct for a typed spec)"
	assert.ElementsMatch(t, []string{
		fmt.Sprintf(named, "Rows"), fmt.Sprintf(named, "MRows"), fmt.Sprintf(named, "BadRows"),
		fmt.Sprintf(nonStructRequestWarning, "ReqRows"),
	}, a.Warnings(t.Context()))
	assert.Empty(t, a.openLiterals)
}

// marshalerRequestLiteralModule is a module whose one route takes a struct
// Marshaler request (a string both ways) with an anonymous struct body field
// whose tag reflect cannot read.
const marshalerRequestLiteralModule = `package mod

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type TextReq struct {
	ID   int64 ` + "`param:\"id\"`" + `
	Body struct {
		Bad int ` + "`json:\"bad\" validate:\"min=1`" + `
	} ` + "`json:\"body\"`" + `
}

func (TextReq) UnmarshalText([]byte) error   { return nil }
func (TextReq) MarshalText() ([]byte, error) { return nil, nil }

func (m *Module) post(req TextReq, ctx server.HandlerContext) (server.Result[int], server.IAPIError) {
	return server.Result[int]{}, nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/t/:id", m.post)
}
`

// TestMarshalerRequestLiteralUnextracted pins that requestParamFields, which
// reads a struct Marshaler request only for its parameters, extracts no
// anonymous struct from a body field it discards: the body is the
// Marshaler's schema, so the literal's malformed tag raises no warning, as a
// named struct's in that position does not.
func TestMarshalerRequestLiteralUnextracted(t *testing.T) {
	a, routes := analyzeSingleModule(t, marshalerRequestLiteralModule)
	req := routeByPath(t, routes, "/t/{id}").Request
	require.NotNil(t, req)
	require.Len(t, req.Fields, 1)
	assert.Equal(t, "id", req.Fields[0].ParamName)
	assert.Empty(t, a.Warnings(t.Context()))
}

// siblingLiteralRows renders k Rows fields Fi, each an anonymous struct that
// embeds *Rows and adds its own Vi.
func siblingLiteralRows(k int) string {
	var b strings.Builder
	for i := 1; i <= k; i++ {
		fmt.Fprintf(&b, "\tF%d struct{ *Rows; V%d int `json:\"v%d\"` } `json:\"f%d\"`\n", i, i, i, i)
	}
	return b.String()
}

// extractedLiterals counts the anonymous structs a rendered Resolution
// expands: every struct{ that is not a cut.
func extractedLiterals(rendered map[string]string) int {
	n := 0
	for _, r := range rendered {
		n += strings.Count(r, "struct{") - strings.Count(r, "cycle:struct{")
	}
	return n
}

// TestAnonymousStructSiblingCycles pins closesLiteralCycle on k sibling
// literals that each embed their enclosing struct, which reach one another in
// every order: once a field's extraction finds two of them on a cycle, either
// met again inside the other is cut, so each field expands each literal once
// (k² in all, not k!), with one recursion warning per field.
func TestAnonymousStructSiblingCycles(t *testing.T) {
	_, got := analyzeRows(t, "", "", siblingLiteralRows(3))
	cut := "cycle:struct{…}"
	f3 := "struct{v3 int; f1 " + cut + "; f2 " + cut + "; f3 " + cut + "}"
	f2 := "struct{v2 int; f1 " + cut + "; f2 " + cut + "; f3 " + f3 + "}"
	assert.Equal(t, "struct{v1 int; f1 "+cut+"; f2 "+f2+"; f3 "+cut+"}", got["f1"],
		"f3 is cut in f1 once f2's extraction found it on f1's cycle")

	const k = 6
	a, got := analyzeRows(t, "", "", siblingLiteralRows(k))
	assert.Equal(t, k*k, extractedLiterals(got))
	assert.Len(t, a.Warnings(t.Context()), k)
	for _, w := range a.Warnings(t.Context()) {
		assert.Contains(t, w, "struct{…} contains itself")
	}
	assert.Empty(t, a.openLiterals)
	assert.Empty(t, a.literalCycles, "the outermost extraction's cycles are forgotten when it closes")
}

// TestAnonymousStructExpansionLimit pins maxLiteralExtractions: named slices
// over anonymous structs that each hold two fields of the next unfold
// 2^n - 1 literals with no cycle, so one field's extraction stops at the
// limit, the rest cut to {}, with one latched warning.
func TestAnonymousStructExpansionLimit(t *testing.T) {
	const n = 11 // 2047 literals
	var decls strings.Builder
	for i := 1; i < n; i++ {
		fmt.Fprintf(&decls, "type Q%d []struct{ A Q%d `json:\"a\"`; B Q%d `json:\"b\"` }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&decls, "type Q%d []struct{ X int `json:\"x\"` }\n", n)
	a, got := analyzeRows(t, "", decls.String(), "\tF Q1 `json:\"f\"`\n\tG Q1 `json:\"g\"`\n")
	assert.Equal(t, 2*maxLiteralExtractions, extractedLiterals(got), "each field gets the whole limit")
	assert.Equal(t, []string{fmt.Sprintf(literalLimitWarning, maxLiteralExtractions)}, a.Warnings(t.Context()))

	_, small := analyzeRows(t, "", "type P1 []struct{ A P2 `json:\"a\"`; B P2 `json:\"b\"` }\ntype P2 []struct{ X int `json:\"x\"` }\n",
		"\tF P1 `json:\"f\"`\n")
	assert.Equal(t, "[]struct{a []struct{x int}; b []struct{x int}}", small["f"], "below the limit a repeat is no cycle")
}

// TestAnonymousStructInlineRefDepth pins that an anonymous struct's fields
// register their refs one level below the field holding it: at
// maxTypeRegistrationDepth the inner ref is past the cap, so it is not
// registered and the latched depth warning fires; one level up it registers.
func TestAnonymousStructInlineRefDepth(t *testing.T) {
	register := func(depth int) *ProjectAnalyzer {
		dir := t.TempDir()
		a := New(dir)
		file, path := parseInDir(t, a, dir, "d.go", "package d\n\ntype Deep struct{ V int }\n\ntype T struct{ F struct{ D Deep } }\n")
		fields := a.extractStructFields(typeSpecInFile(file, "T").Type.(*ast.StructType), file, path, map[string]struct{}{}, 0)
		require.Len(t, fields, 1)
		fi := fields[0]
		require.Equal(t, "struct{D $Deep}", renderShape(fi.ResolvedShape()))
		a.registerFieldRefAt(&fi, file, path, depth)
		return a
	}
	a := register(maxTypeRegistrationDepth - 1)
	assert.Contains(t, a.typeRegistry, "Deep")
	assert.Empty(t, a.warnings)

	a = register(maxTypeRegistrationDepth)
	assert.NotContains(t, a.typeRegistry, "Deep")
	require.Len(t, a.warnings, 1)
	assert.Contains(t, a.warnings[0], "registration depth limit")
}
