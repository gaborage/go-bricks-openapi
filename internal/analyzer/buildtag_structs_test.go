package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedNonStructText opens the payload warning of a local named non-struct
// type (screenUnresolvedPayload), which a type whose build-tagged
// declarations disagree must never raise.
const namedNonStructText = "is a named non-struct type"

// btagModule renders a module file declaring Resp from respBody, with one
// GET route per entry of results (route path -> handler result), and a
// POST /req route taking reqType when it is set.
func btagModule(respBody string, results [][2]string, reqType string) string {
	var reg, handlers strings.Builder
	for i, r := range results {
		fmt.Fprintf(&reg, "\tserver.GET(hr, r, %q, m.h%d)\n", r[0], i)
		fmt.Fprintf(&handlers, "func (m *Module) h%d(ctx server.HandlerContext) (%s, server.IAPIError) {\n\tvar v %s\n\treturn v, nil\n}\n\n",
			i, r[1], r[1])
	}
	if reqType != "" {
		reg.WriteString("\tserver.POST(hr, r, \"/req\", m.post)\n")
		fmt.Fprintf(&handlers, "func (m *Module) post(req %s, ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) {\n"+
			"\treturn server.Result[Resp]{}, nil\n}\n\n", reqType)
	}
	return resolveModuleHead + "\ntype Resp struct {\n" + respBody + "}\n\n" +
		"func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" + reg.String() + "}\n\n" + handlers.String()
}

// TestDeclsDisagreePayloadRoot pins issue 180: a payload whose root is a
// type whose build-tagged declarations disagree raises the payload form of
// the build-tagged warning, as one holding it under a container does, never
// the named non-struct warning, and its route stays untyped. Copies that
// disagree on their shapes name that cause.
func TestDeclsDisagreePayloadRoot(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "rows_linux.go"): "//go:build linux\n\npackage mod\n\n" +
			"type A struct {\n\tX int `json:\"x\"`\n}\n\ntype Rows []A\n\ntype DI []int\n\ntype K []int\n",
		filepath.Join("mod", "rows_other.go"): "//go:build !linux\n\npackage mod\n\n" +
			"type B struct {\n\tY int `json:\"y\"`\n}\n\ntype Rows []B\n\ntype DI []string\n\ntype K map[string]int\n",
		filepath.Join("mod", "module.go"): btagModule("\tR  Rows `json:\"r\"`\n\tDI DI   `json:\"di\"`\n\tK  K    `json:\"k\"`\n",
			[][2]string{
				{"/resp", "server.Result[Resp]"}, {"/rows", "server.Result[Rows]"}, {"/di", "server.Result[DI]"},
				{"/dis", "server.Result[[]DI]"}, {"/k", "server.Result[K]"}, {"/bare", "Rows"},
			}, ""),
	})
	w := a.Warnings(t.Context())
	for goName, typ := range map[string]string{"R": "Rows", "DI": "DI", "K": "K"} {
		assert.Contains(t, w, fmt.Sprintf(declsDisagreePayloadWarning, typ, typ, declsShapeCause))
		if fw := fieldWarnings(a, goName); assert.Len(t, fw, 1, goName) {
			assert.Contains(t, fw[0], typ+"'s build-tagged declarations disagree on shape ("+declsShapeCause+";")
		}
	}
	assert.Contains(t, w, fmt.Sprintf(declsDisagreePayloadWarning, "[]DI", "DI", declsShapeCause))
	assert.Len(t, w, 8, "three fields, Result[[]DI], and Result[Rows], Result[DI], Result[K] and the bare Rows once each")
	for _, msg := range w {
		assert.NotContains(t, msg, namedNonStructText)
	}
	for _, path := range []string{"/rows", "/di", "/k", "/bare"} {
		resp := routeByPath(t, routes, path).Response
		require.NotNil(t, resp, path)
		assert.Empty(t, resp.Name, path)
		assert.Nil(t, resp.Resolution, path, "an untyped payload, as before")
	}
}

// TestDeclsDisagreeClashCause pins issue 180's cause: copies whose shapes
// merge but reach two structs of one name in different packages (type WS
// dw.Items beside type WS lx.Items) name that clash, in a field and in a
// payload.
func TestDeclsDisagreeClashCause(t *testing.T) {
	pkg := func(name, field string) string {
		return "package " + name + "\n\ntype Item struct {\n\t" + field + "\n}\n\ntype Items []Item\n"
	}
	variant := func(constraint, decl string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\nimport (\n\t\"github.com/example/app/dw\"\n\t\"github.com/example/app/lx\"\n)\n\n" +
			"var _ dw.Items\nvar _ lx.Items\n\n" + decl + "\n"
	}
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                             resolveGoMod,
		filepath.Join("dw", "dw.go"):         pkg("dw", "D int `json:\"d\"`"),
		filepath.Join("lx", "lx.go"):         pkg("lx", "L string `json:\"l\"`"),
		filepath.Join("mod", "ws_darwin.go"): variant("darwin", "type WS dw.Items"),
		filepath.Join("mod", "ws_linux.go"):  variant("linux", "type WS lx.Items"),
		filepath.Join("mod", "module.go"): btagModule("\tWS WS `json:\"ws\"`\n",
			[][2]string{{"/resp", "server.Result[Resp]"}, {"/ws", "server.Result[WS]"}}, "WS"),
	})
	w := a.Warnings(t.Context())
	assert.Contains(t, w, fmt.Sprintf(declsDisagreePayloadWarning, "WS", "WS", declsClashCause))
	if fw := fieldWarnings(a, "WS"); assert.Len(t, fw, 1) {
		assert.Contains(t, fw[0], "WS's build-tagged declarations disagree on shape ("+declsClashCause+";")
	}
	// Every build's WS is a slice, so the request keeps F26.
	assert.Contains(t, w, fmt.Sprintf(nonStructRequestWarning, "WS"))
	assert.Len(t, w, 3)
}

// btagStructFiles is the buildtag_structs fixture's project (issue 157):
// copies that each reach a struct, but not the same one, beside Same, whose
// copies both reach dw.Item. linux and other name the two tagged files.
func btagStructFiles(linux, other string) map[string]string {
	const head = "package mod\n\nimport (\n"
	return map[string]string{
		"go.mod":                     resolveGoMod,
		filepath.Join("dw", "dw.go"): "package dw\n\ntype Item struct {\n\tD int `json:\"d\"`\n}\n\ntype Other struct {\n\tO bool `json:\"o\"`\n}\n",
		filepath.Join("lx", "lx.go"): "package lx\n\ntype Item struct {\n\tL string `json:\"l\"`\n}\n",
		filepath.Join("mod", linux): "//go:build linux\n\n" + head +
			"\t\"github.com/example/app/dw\"\n\titem \"github.com/example/app/lx\"\n)\n\n" +
			"type W dw.Item\n\ntype WA = dw.Item\n\ntype WI item.Item\n\ntype Two dw.Item\n\n" +
			"type Loc struct {\n\tK int `json:\"k\"`\n}\n\ntype Same dw.Item\n",
		filepath.Join("mod", other): "//go:build !linux\n\n" + head +
			"\t\"github.com/example/app/dw\"\n\titem \"github.com/example/app/dw\"\n\t\"github.com/example/app/lx\"\n)\n\n" +
			"type W lx.Item\n\ntype WA = lx.Item\n\ntype WI item.Item\n\ntype Two dw.Other\n\ntype Loc dw.Item\n\ntype Same = dw.Item\n",
		filepath.Join("mod", "module.go"): btagModule(
			"\tW     W     `json:\"w\"`\n\tWA    WA    `json:\"wa\"`\n\tWI    WI    `json:\"wi\"`\n\tTwo   Two   `json:\"two\"`\n"+
				"\tLoc   Loc   `json:\"loc\"`\n\tOverW OverW `json:\"overW\"`\n\tWP    []W   `json:\"wp\"`\n\tSame  Same  `json:\"same\"`\n",
			[][2]string{{"/resp", "server.Result[Resp]"}, {"/w", "server.Result[W]"}, {"/loc", "server.Result[Loc]"}}, "W") +
			"type OverW W\n",
	}
}

// TestBuildTaggedStructCopies pins issue 157: a type whose build-tagged
// declarations each reach a struct, but not the same struct declaration, is
// documented as an untyped object with the build-tagged warning wherever it
// is used (fields, slice items, a payload, a request, and through a single
// declaration over it), and registers no component. The cause is a clash
// when the copies reach one struct name in two directories, else the shape.
// Same, whose copies reach one struct, keeps its component. The tagged
// files' names, which decide their order, change nothing.
func TestBuildTaggedStructCopies(t *testing.T) {
	for _, files := range [][2]string{{"w_linux.go", "w_other.go"}, {"zz_linux.go", "aa_other.go"}} {
		t.Run(files[0], func(t *testing.T) {
			a, routes := analyzeProjectRoutes(t, btagStructFiles(files[0], files[1]))
			assert.Equal(t, map[string]string{
				"w": "W", "wa": "WA", "wi": "WI", "two": "Two", "loc": "Loc", "overW": "W", "wp": "[]W", "same": "$Same",
			}, renders(t, a.typeRegistry["Resp"]))
			for _, absent := range []string{"W", "WA", "WI", "Two", "Loc", "OverW", "Item", "DwItem", "LxItem"} {
				assert.NotContains(t, a.typeRegistry, absent)
			}
			assert.Contains(t, a.typeRegistry, "Same")

			want := append([]string{
				fmt.Sprintf(declsDisagreePayloadWarning, "W", "W", declsClashCause),
				fmt.Sprintf(declsDisagreePayloadWarning, "Loc", "Loc", declsShapeCause),
				fmt.Sprintf(declsDisagreeRequestWarning, "W", "W", declsClashCause),
			}, structCopyFieldWarnings(t, a)...)
			assert.ElementsMatch(t, want, a.Warnings(t.Context()))

			for _, path := range []string{"/w", "/loc"} {
				resp := routeByPath(t, routes, path).Response
				require.NotNil(t, resp, path)
				assert.Empty(t, resp.Name, path)
			}
			post := routeByPath(t, routes, "/req")
			if post.Request != nil {
				assert.Empty(t, post.Request.Name)
			}
		})
	}
}

// structCopyFieldWarnings asserts the one build-tagged warning each Resp
// field of btagStructFiles raises, with the type where resolution stopped and
// its cause, and returns them.
func structCopyFieldWarnings(t *testing.T, a *ProjectAnalyzer) []string {
	t.Helper()
	var out []string
	for _, f := range []struct{ goName, declared, stop, cause string }{
		{"W", "W", "W", declsClashCause}, {"WA", "WA", "WA", declsClashCause}, {"WI", "WI", "WI", declsClashCause},
		{"Two", "Two", "Two", declsShapeCause}, {"Loc", "Loc", "Loc", declsShapeCause},
		{"OverW", "OverW", "W", declsClashCause}, {"WP", "[]W", "W", declsClashCause},
	} {
		fw := fieldWarnings(a, f.goName)
		if assert.Len(t, fw, 1, f.goName) {
			assert.Contains(t, fw[0], fmt.Sprintf("has type %s: %s's build-tagged declarations disagree on shape (%s;",
				f.declared, f.stop, f.cause))
			out = append(out, fw[0])
		}
	}
	return out
}

// TestBuildTaggedStructCopyRequests pins the request form of issue 157
// through a pointer and through a single declaration over the type: the
// build-tagged request warning names the type whose copies disagree, never
// F26. A container of such a type ([]W, a named slice or pointer over it) is
// no struct in any build, so it keeps F26.
func TestBuildTaggedStructCopyRequests(t *testing.T) {
	for _, c := range []struct{ req, want string }{
		{"*W", fmt.Sprintf(declsDisagreeRequestWarning, "*W", "W", declsClashCause)},
		{"OverW", fmt.Sprintf(declsDisagreeRequestWarning, "OverW", "W", declsClashCause)},
		{"*Loc", fmt.Sprintf(declsDisagreeRequestWarning, "*Loc", "Loc", declsShapeCause)},
		{"[]W", fmt.Sprintf(nonStructRequestWarning, "[]W")},
		{"WS", fmt.Sprintf(nonStructRequestWarning, "WS")},
		{"PW", fmt.Sprintf(nonStructRequestWarning, "PW")},
	} {
		t.Run(c.req, func(t *testing.T) {
			files := btagStructFiles("w_linux.go", "w_other.go")
			files[filepath.Join("mod", "module.go")] = btagModule("\tSame Same `json:\"same\"`\n", nil, c.req) +
				"type OverW W\n\ntype WS []W\n\ntype PW *W\n"
			a, _ := analyzeProjectRoutes(t, files)
			assert.Equal(t, []string{c.want}, a.Warnings(t.Context()))
		})
	}
}

// TestBuildTaggedStructCopyEmbeds pins what issue 157 leaves to its own
// decision: an untagged embed of a type whose copies reach different structs
// promotes the fields of the first copy by file order, with no warning.
func TestBuildTaggedStructCopyEmbeds(t *testing.T) {
	for _, c := range []struct {
		linux, other string
		w, loc       map[string]string
	}{
		{"w_linux.go", "w_other.go", map[string]string{"d": "int"}, map[string]string{"k": "int"}},
		{"zz_linux.go", "aa_other.go", map[string]string{"l": "string"}, map[string]string{"d": "int"}},
	} {
		t.Run(c.linux, func(t *testing.T) {
			files := btagStructFiles(c.linux, c.other)
			files[filepath.Join("mod", "module.go")] = btagModule("\tEW EW `json:\"ew\"`\n\tEL EL `json:\"el\"`\n",
				[][2]string{{"/resp", "server.Result[Resp]"}}, "") +
				"type EW struct {\n\tW\n}\n\ntype EL struct {\n\tLoc\n}\n"
			a, _ := analyzeProjectRoutes(t, files)
			assert.Empty(t, a.Warnings(t.Context()))
			assert.Equal(t, c.w, renders(t, a.typeRegistry["EW"]), "EW")
			assert.Equal(t, c.loc, renders(t, a.typeRegistry["EL"]), "EL")
		})
	}
}

// TestBuildTaggedCopiesOutsideIssue157 pins that the forms issue 199 owns
// are unchanged: copies of which one reaches no struct follow file order (the
// struct copy first documents its struct with no warning, the slice copy
// first disagrees), and same-package struct copies with different fields
// document the first copy's struct with no warning.
func TestBuildTaggedCopiesOutsideIssue157(t *testing.T) {
	const dw = "package dw\n\ntype Item struct {\n\tD int `json:\"d\"`\n}\n"
	tagged := func(constraint, imports, decl string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\n" + imports + decl + "\n"
	}
	const imp = "import \"github.com/example/app/dw\"\n\n"
	cases := []struct {
		name, a, b, want string
		fields           []string
	}{
		{"a struct copy beside a slice copy", tagged("linux", imp, "type M dw.Item"), tagged("!linux", imp, "type M []dw.Item"),
			"$M", []string{"d"}},
		{"a slice copy beside a struct copy", tagged("linux", imp, "type M []dw.Item"), tagged("!linux", imp, "type M dw.Item"),
			"M", nil},
		{"same-package struct copies", tagged("linux", "", "type M struct {\n\tD int `json:\"d\"`\n}"),
			tagged("!linux", "", "type M struct {\n\tL string `json:\"l\"`\n}"), "$M", []string{"d"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, _ := analyzeProjectRoutes(t, map[string]string{
				"go.mod":                       resolveGoMod,
				filepath.Join("dw", "dw.go"):   dw,
				filepath.Join("mod", "m_a.go"): c.a,
				filepath.Join("mod", "m_b.go"): c.b,
				filepath.Join("mod", "module.go"): btagModule("\tM M `json:\"m\"`\n",
					[][2]string{{"/resp", "server.Result[Resp]"}, {"/m", "server.Result[M]"}}, "M"),
			})
			assert.Equal(t, c.want, renders(t, a.typeRegistry["Resp"])["m"])
			if c.fields == nil {
				// The slice copy first: the copies disagree, as before; only
				// the payload's text is issue 180's.
				w := a.Warnings(t.Context())
				require.Len(t, w, 3)
				assert.Contains(t, w[0], "has type M: M's build-tagged declarations disagree on shape ("+declsShapeCause+";")
				assert.Equal(t, []string{fmt.Sprintf(declsDisagreePayloadWarning, "M", "M", declsShapeCause),
					fmt.Sprintf(nonStructRequestWarning, "M")}, w[1:])
				assert.NotContains(t, a.typeRegistry, "M")
				return
			}
			m := a.typeRegistry["M"]
			require.NotNil(t, m)
			var names []string
			for _, f := range m.Fields {
				names = append(names, f.JSONName)
			}
			assert.Equal(t, c.fields, names)
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}

// TestStructEndsCause pins the verdict over the structs a type's copies
// reach: one declaration agrees, one name in two directories clashes, and
// two names disagree on shape, whichever copy differs.
func TestStructEndsCause(t *testing.T) {
	end := func(dir, name string) chainEnd { return chainEnd{name: name, path: filepath.Join(dir, "x.go")} }
	cases := []struct {
		ends     []chainEnd
		cause    string
		disagree bool
	}{
		{[]chainEnd{end("dw", "Item"), end("dw", "Item")}, "", false},
		{[]chainEnd{end("dw", "Item"), end("lx", "Item")}, declsClashCause, true},
		{[]chainEnd{end("dw", "Item"), end("dw", "Other")}, declsShapeCause, true},
		{[]chainEnd{end("dw", "Item"), end("lx", "Item"), end("dw", "Other")}, declsShapeCause, true},
	}
	for _, c := range cases {
		cause, disagree := structEndsCause(c.ends)
		assert.Equal(t, c.cause, cause)
		assert.Equal(t, c.disagree, disagree)
	}
}

// btagQualifiedFiles is btagStructFiles with the tagged copies moved to
// package b, used from mod through b's qualifier: resp is Resp's body, results
// its GET routes and req the POST /req request type.
func btagQualifiedFiles(linux, other, resp string, results [][2]string, req string) map[string]string {
	files := btagStructFiles(linux, other)
	for _, name := range []string{linux, other} {
		src := files[filepath.Join("mod", name)]
		delete(files, filepath.Join("mod", name))
		files[filepath.Join("b", name)] = strings.Replace(src, "package mod", "package b", 1)
	}
	files[filepath.Join("mod", "module.go")] = strings.Replace(btagModule(resp, results, req),
		"import (\n", "import (\n\t\"github.com/example/app/b\"\n", 1)
	return files
}

// TestBuildTaggedStructCopiesQualified pins issue 157 through another
// package's qualifier: b.Loc, whose copies are a struct literal and dw.Item,
// is no struct component however its files are named, in a field, a slice,
// a payload and a request, while b.Same keeps its component silently.
func TestBuildTaggedStructCopiesQualified(t *testing.T) {
	for _, files := range [][2]string{{"w_linux.go", "w_other.go"}, {"zz_linux.go", "aa_other.go"}} {
		t.Run(files[0], func(t *testing.T) {
			a, routes := analyzeProjectRoutes(t, btagQualifiedFiles(files[0], files[1],
				"\tLoc  b.Loc   `json:\"loc\"`\n\tLocs []b.Loc `json:\"locs\"`\n\tSame b.Same  `json:\"same\"`\n",
				[][2]string{{"/resp", "server.Result[Resp]"}, {"/loc", "server.Result[b.Loc]"}}, "b.Loc"))
			assert.Equal(t, map[string]string{"loc": "Loc", "locs": "[]Loc", "same": "$Same"}, renders(t, a.typeRegistry["Resp"]))
			assert.NotContains(t, a.typeRegistry, "Loc")
			assert.Contains(t, a.typeRegistry, "Same")
			w := a.Warnings(t.Context())
			assert.Contains(t, w, fmt.Sprintf(declsDisagreePayloadWarning, "b.Loc", "b.Loc", declsShapeCause))
			assert.Contains(t, w, fmt.Sprintf(declsDisagreeRequestWarning, "b.Loc", "b.Loc", declsShapeCause))
			for _, goName := range []string{"Loc", "Locs"} {
				if fw := fieldWarnings(a, goName); assert.Len(t, fw, 1, goName) {
					assert.Contains(t, fw[0], "b.Loc's build-tagged declarations disagree on shape ("+declsShapeCause+";")
				}
			}
			assert.Len(t, w, 4)
			assert.Empty(t, routeByPath(t, routes, "/loc").Response.Name)
		})
	}
}

// TestBuildTaggedCopiesThroughQualifiedChains pins that copies are compared
// by the struct they reach through another package's own declarations: Ch,
// reaching dw.T in one build and lx.T = dw2.S in the other, disagrees in
// either file order, while copies reaching one struct by any route
// (identical copies, dw.T beside lx.U = dw.T) agree silently and keep their
// component in either order.
func TestBuildTaggedCopiesThroughQualifiedChains(t *testing.T) {
	const imports = "import (\n\t\"github.com/example/app/dw\"\n\t\"github.com/example/app/lx\"\n)\n\nvar _ dw.T\nvar _ lx.T\n\n"
	tagged := func(constraint, decls string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\n" + imports + decls
	}
	const (
		linux = "type Ch dw.T\n\ntype Id dw.T\n\ntype QU dw.T\n"
		other = "type Ch lx.T\n\ntype Id dw.T\n\ntype QU lx.U\n"
	)
	for _, names := range [][2]string{{"m_linux.go", "m_other.go"}, {"z_linux.go", "a_other.go"}} {
		t.Run(names[0], func(t *testing.T) {
			a, _ := analyzeProjectRoutes(t, map[string]string{
				"go.mod":                       resolveGoMod,
				filepath.Join("dw", "dw.go"):   "package dw\n\ntype T struct {\n\tD int `json:\"d\"`\n}\n",
				filepath.Join("dw2", "dw2.go"): "package dw2\n\ntype S struct {\n\tS string `json:\"s\"`\n}\n",
				filepath.Join("lx", "lx.go"): "package lx\n\nimport (\n\t\"github.com/example/app/dw\"\n\t\"github.com/example/app/dw2\"\n)\n\n" +
					"type T = dw2.S\n\ntype U = dw.T\n",
				filepath.Join("mod", names[0]): tagged("linux", linux),
				filepath.Join("mod", names[1]): tagged("!linux", other),
				filepath.Join("mod", "module.go"): btagModule("\tCh Ch `json:\"ch\"`\n\tId Id `json:\"id\"`\n\tQU QU `json:\"qu\"`\n",
					[][2]string{{"/resp", "server.Result[Resp]"}}, ""),
			})
			got := renders(t, a.typeRegistry["Resp"])
			assert.Equal(t, "Ch", got["ch"])
			assert.Equal(t, "$Id", got["id"])
			assert.Equal(t, "$QU", got["qu"], "copies reaching dw.T by two routes agree")
			if fw := fieldWarnings(a, "Ch"); assert.Len(t, fw, 1) {
				assert.Contains(t, fw[0], "Ch's build-tagged declarations disagree on shape ("+declsShapeCause+";")
			}
			assert.Len(t, a.Warnings(t.Context()), 1)
		})
	}
}
