package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// A parenthesized type is the same type: (T) is T and *(T) is *T (issues
// 160, 167 and 177). These tests parse source strings, never formatted
// fixtures, because gofmt strips the top-level parentheses of a parameter
// or result type ((*T), (T), (string)).

// parenModSrc is a module whose one route, POST /items, takes the parameter
// list PARAMS and returns RESULT.
const parenModSrc = `package mod

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type CreateReq struct {
	Name string ` + "`json:\"name\"`" + `
}

type Named struct {
	A int ` + "`json:\"a\"`" + `
}

type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/items", m.create)
}

func (m *Module) create(PARAMS) (RESULT, server.IAPIError) {
	return out, nil
}
`

// analyzeParen analyzes parenModSrc with the given parameter list and first
// result, returning POST /items and every analyzer warning.
func analyzeParen(t *testing.T, params, result string) (route models.Route, warnings []string) {
	t.Helper()
	src := strings.NewReplacer("PARAMS", params, "RESULT", result).Replace(parenModSrc)
	a, routes := analyzeSingleModule(t, src)
	return routeForPath(t, routes, "POST /items"), a.Warnings(t.Context())
}

// TestParenStructRequest pins that a struct request written in parentheses,
// at the top or under its pointer, is resolved exactly as its unparenthesized
// form: a CreateReq body and no warning, in either parameter position.
func TestParenStructRequest(t *testing.T) {
	for _, typ := range []string{"(*CreateReq)", "(CreateReq)", "*(CreateReq)", "((*(CreateReq)))"} {
		for _, params := range []string{"req " + typ + ", ctx server.HandlerContext", "ctx (server.HandlerContext), req " + typ} {
			t.Run(params, func(t *testing.T) {
				route, warnings := analyzeParen(t, params, "server.Result[Named]")
				assert.Empty(t, warnings)
				require.NotNil(t, route.Request, "a parenthesized request is not skipped")
				assert.Equal(t, "CreateReq", route.Request.Name)
			})
		}
	}
}

// TestParenNonStructRequestWarnsOnce pins that a non-struct request written
// in parentheses takes the non-struct path of its unparenthesized form (F26),
// once, naming the type as written.
func TestParenNonStructRequestWarnsOnce(t *testing.T) {
	for _, typ := range []string{"(string)", "*(string)", "*([]CreateReq)", "(map[string]int)"} {
		t.Run(typ, func(t *testing.T) {
			route, warnings := analyzeParen(t, "req "+typ+", ctx server.HandlerContext", "server.Result[Named]")
			assert.Equal(t, []string{wantNonStructRequestWarning(typ)}, warnings)
			assertUntypedRequest(t, route.Request)
		})
	}
}

// TestParenGenericRequestWarns pins that a generic request written in
// parentheses, around or under its pointer, raises the generic request
// warning of Box[CreateReq], not F26.
func TestParenGenericRequestWarns(t *testing.T) {
	for _, typ := range []string{"*(Box[CreateReq])", "(*Box[CreateReq])", "(Box[CreateReq])"} {
		t.Run(typ, func(t *testing.T) {
			route, warnings := analyzeParen(t, "req "+typ+", ctx server.HandlerContext", "server.Result[Named]")
			want := "request type " + typ + ": Box[CreateReq] is a generic type instantiation, which is not resolved — no requestBody emitted"
			assert.Equal(t, []string{want}, warnings)
			assert.Nil(t, route.Request)
		})
	}
}

// TestParenPayloads pins that a payload written in parentheses, in the
// result wrapper's type argument or around the wrapper itself, resolves
// exactly as its unparenthesized form.
func TestParenPayloads(t *testing.T) {
	cases := []struct{ result, plain string }{
		{"(server.Result[Named])", "server.Result[Named]"},
		{"((server.ResultWithMeta[(Named)]))", "server.ResultWithMeta[Named]"},
		{"server.Result[(Named)]", "server.Result[Named]"},
		{"server.Result[*(*Named)]", "server.Result[**Named]"},
		{"server.Result[(*Named)]", "server.Result[*Named]"},
		{"server.Result[*([]Named)]", "server.Result[*[]Named]"},
		{"server.Result[[](Named)]", "server.Result[[]Named]"},
		{"server.Result[(map[string]int)]", "server.Result[map[string]int]"},
		{"server.Result[(interface{})]", "server.Result[interface{}]"},
		{"server.Result[(string)]", "server.Result[string]"},
		{"*(Named)", "*Named"},
		{"server.Result[(struct{ A int })]", "server.Result[struct{ A int }]"},
		{"server.Result[*(struct{ A int })]", "server.Result[*struct{ A int }]"},
	}
	for _, c := range cases {
		t.Run(c.result, func(t *testing.T) {
			route, warnings := analyzeParen(t, "ctx server.HandlerContext", c.result)
			plain, plainWarnings := analyzeParen(t, "ctx server.HandlerContext", c.plain)
			assert.Empty(t, warnings)
			assert.Empty(t, plainWarnings)
			require.NotNil(t, route.Response, "a parenthesized payload is typed")
			require.NotNil(t, plain.Response)
			assert.Equal(t, plain.Response.Name, route.Response.Name)
			assert.Equal(t, renderShapePtr(plain.Response.Shape), renderShapePtr(route.Response.Shape))
		})
	}
}

// TestParenNoContentResult pins that a parenthesized NoContentResult result
// keeps its bodiless-204 meaning.
func TestParenNoContentResult(t *testing.T) {
	route, warnings := analyzeParen(t, "ctx server.HandlerContext", "(server.NoContentResult)")
	assert.Empty(t, warnings)
	require.NotNil(t, route.Response)
	assert.True(t, route.Response.NoContent)
}

// TestParenGenericPayloadWarns pins that a parenthesized generic payload
// raises the generic payload warning of its unparenthesized form, naming the
// type as written, and that a pointer to a result wrapper, parenthesized
// around or under the pointer, is unwrapped like its value form (issue 143):
// typed, with no warning.
func TestParenGenericPayloadWarns(t *testing.T) {
	for _, typ := range []string{"(Box[Named])", "Box[Named]"} {
		t.Run(typ, func(t *testing.T) {
			route, warnings := analyzeParen(t, "ctx server.HandlerContext", "server.Result["+typ+"]")
			want := "response type " + typ + ": Box[Named] is a generic type instantiation, which is not resolved — emitting an untyped schema"
			assert.Equal(t, []string{want}, warnings)
			assert.Nil(t, route.Response)
		})
	}
	for _, typ := range []string{"*server.Result[Named]", "*(server.Result[Named])", "(*server.Result[Named])"} {
		t.Run(typ, func(t *testing.T) {
			route, warnings := analyzeParen(t, "ctx server.HandlerContext", typ)
			assert.Empty(t, warnings)
			require.NotNil(t, route.Response)
			assert.Equal(t, "Named", route.Response.Name)
		})
	}
}

// TestParenJOSELiteralPayloadUntyped pins that a jose-tagged anonymous
// struct payload stays untyped behind parentheses, like its
// unparenthesized form (issue 171).
func TestParenJOSELiteralPayloadUntyped(t *testing.T) {
	lit := "struct{ _ struct{} `jose:\"decrypt=our-signing,verify=partner-verify\"`; V string `json:\"v\"` }"
	for _, typ := range []string{"*" + lit, "*(" + lit + ")", "(*" + lit + ")", "(" + lit + ")"} {
		t.Run(typ, func(t *testing.T) {
			route, _ := analyzeParen(t, "ctx server.HandlerContext", "server.Result["+typ+"]")
			assert.Nil(t, route.Response)
		})
	}
}

// TestTypeShapeSeesThroughParens pins that a parenthesized expression, at
// every depth a Shape is decoded, decodes as the expression without them.
func TestTypeShapeSeesThroughParens(t *testing.T) {
	a := New("")
	cases := map[string]string{
		"(Address)":                   "Address",
		"((int))":                     "int",
		"*(string)":                   "*string",
		"(*Address)":                  "*Address",
		"*(*Address)":                 "**Address",
		"[](Address)":                 "[]Address",
		"[2](byte)":                   "[2]byte",
		"([]Address)":                 "[]Address",
		"map[(string)](int)":          "map[string]int",
		"(map[string]*(Address))":     "map[string]*Address",
		"(interface{})":               "interface{}",
		"(struct{ A int })":           "struct{ A int }",
		"(q.Item)":                    "q.Item",
		"Box[(Address)]":              "Box[Address]",
		"Pair[(string), *(Address)]":  "Pair[string, *Address]",
		"(Box[[](Address)])":          "Box[[]Address]",
		"map[string](Box[(Address)])": "map[string]Box[Address]",
	}
	for src, plain := range cases {
		t.Run(src, func(t *testing.T) {
			got, want := a.typeShape(mustParse(t, src)), a.typeShape(mustParse(t, plain))
			// An Instantiation keeps its type as written (Written), the only
			// place a parenthesis survives decoding: warnings name it.
			assert.Equal(t, renderShape(want), stripParens.Replace(renderShape(got)))
			assert.Equal(t, instArgs(want), instArgs(got), "instantiation arguments")
		})
	}
}

var stripParens = strings.NewReplacer("(", "", ")", "")

// instArgs renders the arguments of every Instantiation on s's element
// path, which renderShape omits (it renders an Instantiation as written).
func instArgs(s models.TypeShape) []string {
	var out []string
	walkResolvedLeaves(&s, func(leaf *models.TypeShape) {
		if leaf.Inst == nil {
			return
		}
		out = append(out, leaf.Inst.Base)
		for _, arg := range leaf.Inst.Args {
			out = append(out, renderShape(arg))
		}
	})
	return out
}

// TestParenDeclarationsTypeFields pins issue 167: a field typed through a
// declaration with a parenthesized right-hand side, or written in
// parentheses itself, resolves exactly as without them. PB, PBA and PS keep
// their own components.
func TestParenDeclarationsTypeFields(t *testing.T) {
	decls := "type Base struct {\n\tX int `json:\"x\"`\n}\n\ntype Cents int64\n\n" +
		"type PB (Base)\n\ntype PBA = (Base)\n\ntype PC (Cents)\n\n" +
		"type PS (struct {\n\tY string `json:\"y\"`\n})\n\ntype PSl ([]Base)\n"
	rows := []resolveRow{
		{"b", "PB", "$PB"}, {"ba", "PBA", "$PBA"}, {"c", "PC", "int64"}, {"s", "PS", "$PS"},
		{"sl", "PSl", "[]$Base"}, {"paren", "(int)", "int"}, {"pparen", "*(string)", "*string"},
		{"pelem", "[](Base)", "[]$Base"},
	}
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("mod", "module.go"): rowsModule("", decls, rowsBody(rows)),
	})
	want := map[string]string{}
	for _, r := range rows {
		want[r.json] = r.want
	}
	assert.Equal(t, want, renders(t, a.typeRegistry["Rows"]))
	for _, name := range []string{"PB", "PBA", "PS"} {
		assert.Contains(t, a.typeRegistry, name)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestParenMarshalerDeclaration pins that a declaration with a parenthesized
// right-hand side keeps the method set its target contributes: an alias of a
// Marshaler, and a struct literal embedding one, are Marshaler leaves as
// they are without the parentheses.
func TestParenMarshalerDeclaration(t *testing.T) {
	money := "type Money struct {\n\tC int `json:\"c\"`\n}\n\nfunc (Money) MarshalJSON() ([]byte, error) { return nil, nil }\n\n"
	for _, paren := range []bool{false, true} {
		wrap := func(s string) string {
			if paren {
				return "(" + s + ")"
			}
			return s
		}
		decls := money + "type MA = " + wrap("Money") + "\n\ntype ME " + wrap("struct {\n\tMoney\n}") + "\n"
		rows := "\tA MA `json:\"a\"`\n\tE ME `json:\"e\"`\n"
		a := analyzeDirectiveProject(t, map[string]string{
			"go.mod":                          resolveGoMod,
			filepath.Join("mod", "module.go"): rowsModule("", decls, rows),
		})
		got := renders(t, a.typeRegistry["Rows"])
		assert.True(t, strings.HasPrefix(got["a"], "marshal:"), "paren %v: a = %s", paren, got["a"])
		assert.True(t, strings.HasPrefix(got["e"], "marshal:"), "paren %v: e = %s", paren, got["e"])
	}
}

// TestParenDeclarationSites pins the declaration helpers that switch on a
// type declaration's right-hand side: each sees through its parentheses.
func TestParenDeclarationSites(t *testing.T) {
	src := "package mod\n\ntype Base struct{ X int }\n\ntype PB (Base)\n\ntype PBA = (Base)\n\n" +
		"type PS (struct{ Y string })\n\ntype PSl ([]Base)\n"
	path := filepath.Join(t.TempDir(), "mod.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	require.NoError(t, err)
	a := New(filepath.Dir(path))

	assert.NotNil(t, a.findStructInFile(f, "PS"), "a parenthesized struct literal is a struct declaration")
	assert.Nil(t, a.findStructInFile(f, "PB"))
	reachesStruct := func(name string) bool {
		st, pkg, file, declPath, ok := a.resolveTypeSpecChain(f, path, name, 0)
		return ok && st != nil && pkg == "mod" && file == f && declPath == path
	}
	for _, name := range []string{"PB", "PBA", "PS"} {
		assert.True(t, reachesStruct(name), name)
	}
	assert.False(t, reachesStruct("PSl"), "a parenthesized slice is not a struct")

	reExport := func(src string) bool {
		spec := parseTypeSpec(t, src)
		return isSameNamedReExport(spec)
	}
	assert.True(t, reExport("type User = (d.User)"))
	assert.True(t, reExport("type User = d.User"))
	assert.False(t, reExport("type User (d.User)"), "a defined type is not an alias")
	assert.False(t, reExport("type UA = (d.User)"))
}

// parseTypeSpec parses one type declaration.
func parseTypeSpec(t *testing.T, decl string) *ast.TypeSpec {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\n\n"+decl+"\n", 0)
	require.NoError(t, err)
	gen, ok := f.Decls[0].(*ast.GenDecl)
	require.True(t, ok)
	spec, ok := gen.Specs[0].(*ast.TypeSpec)
	require.True(t, ok)
	return spec
}

// TestParenReExportSharesComponent pins that an alias re-exporting a
// same-named type of another package written in parentheses (type User =
// (d.User)) shares d's User component, as type User = d.User does.
func TestParenReExportSharesComponent(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("d", "d.go"):        "package d\n\ntype User struct {\n\tID int `json:\"id\"`\n}\n",
		filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/d\"\n", "type User = (d.User)\n", "\tA User `json:\"a\"`\n\tB d.User `json:\"b\"`\n"),
	})
	assert.Equal(t, map[string]string{"a": "$User", "b": "$User"}, renders(t, a.typeRegistry["Rows"]))
	assert.NotContains(t, a.typeRegistry, "DUser")
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestPointeeShedsParens pins the shared helper behind the request checks.
func TestPointeeShedsParens(t *testing.T) {
	cases := map[string]string{
		"Box[A]":      "Box[A]",
		"*Box[A]":     "Box[A]",
		"(*Box[A])":   "Box[A]",
		"*(Box[A])":   "Box[A]",
		"((*(T)))":    "T",
		"**T":         "*T",
		"(struct{})":  "struct{}",
		"*([]string)": "[]string",
	}
	for src, want := range cases {
		t.Run(src, func(t *testing.T) {
			assert.Equal(t, want, types.ExprString(pointee(mustParse(t, src))))
		})
	}
}
