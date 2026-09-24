package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopedModuleSrc wraps a RegisterRoutes body (plus optional package-level
// declarations) in a minimal go-bricks module.
func scopedModuleSrc(pkgDecls, body string) string {
	return `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{}
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(d *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
` + pkgDecls + `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
` + body + `
}
func (m *Module) h(ctx server.HandlerContext) (server.Result[int], server.IAPIError) { return server.OK(0), nil }
`
}

func TestScopedConstRoutePaths(t *testing.T) {
	tests := []struct {
		name      string
		pkgDecls  string
		body      string
		wantPaths []string
	}{
		{
			name:      "function local const",
			body:      "const local = \"/local\"\n\tserver.GET(hr, r, local, m.h)",
			wantPaths: []string{"/local"},
		},
		{
			name:      "nested block const",
			body:      "if true {\n\t\tconst nested = \"/nested\"\n\t\tserver.GET(hr, r, nested, m.h)\n\t}",
			wantPaths: []string{"/nested"},
		},
		{
			name:      "typed local const",
			body:      "type Path = string\n\tconst typed Path = \"/typed\"\n\tserver.GET(hr, r, typed, m.h)",
			wantPaths: []string{"/typed"},
		},
		{
			name:      "inner const shadows outer const",
			pkgDecls:  "const shadowed = \"/pkg\"",
			body:      "const shadowed = \"/fn\"\n\tif true {\n\t\tconst shadowed = \"/block\"\n\t\tserver.GET(hr, r, shadowed, m.h)\n\t}\n\tserver.POST(hr, r, shadowed, m.h)",
			wantPaths: []string{"/block", "/fn"},
		},
		{
			name:      "const declared after the registration does not bind",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "server.GET(hr, r, p, m.h)\n\tconst p = \"/later\"\n\t_ = p",
			wantPaths: []string{"/pkg"},
		},
		{
			name:      "const declared after the registration with no outer binding is unresolved",
			body:      "server.GET(hr, r, p, m.h)\n\tconst p = \"/later\"\n\t_ = p",
			wantPaths: nil,
		},
		{
			name:      "non string local const shadows a package const",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "const p = 42\n\t_ = p\n\tserver.GET(hr, r, p, m.h)",
			wantPaths: nil,
		},
		{
			name:      "local var shadows a package const",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "var p = compute()\n\tserver.GET(hr, r, p, m.h)",
			wantPaths: nil,
		},
		{
			name:      "short declaration shadows a package const",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "p := compute()\n\tserver.GET(hr, r, p, m.h)",
			wantPaths: nil,
		},
		{
			name:      "if init binding shadows a package const",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "if p := compute(); true {\n\t\tserver.GET(hr, r, p, m.h)\n\t}",
			wantPaths: nil,
		},
		{
			name:      "range binding shadows a package const",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "for _, p := range items {\n\t\tserver.GET(hr, r, p, m.h)\n\t}",
			wantPaths: nil,
		},
		{
			name:      "select receive variable shadows a package const",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "select {\n\tcase p := <-ch:\n\t\tserver.GET(hr, r, p, m.h)\n\t}",
			wantPaths: nil,
		},
		{
			name:      "short declaration reusing a local keeps it shadowing a package const",
			pkgDecls:  "const p = \"/pkg\"",
			body:      "p := compute()\n\tserver.GET(hr, r, p, m.h)\n\tn, p := 1, compute()\n\t_, _ = n, p",
			wantPaths: nil,
		},
		{
			name:      "group prefix from a function local const",
			body:      "const base = \"/api/v1\"\n\tapi := r.Group(base)\n\tserver.GET(hr, api, \"/widgets\", m.h)",
			wantPaths: []string{"/api/v1/widgets"},
		},
		{
			name:      "unresolved group prefix drops the route rather than losing the prefix",
			body:      "api := r.Group(compute())\n\tserver.GET(hr, api, \"/widgets\", m.h)",
			wantPaths: nil,
		},
		{
			name:      "nested group inherits an unresolved parent prefix",
			body:      "outer := r.Group(compute())\n\tinner := outer.Group(\"/x\")\n\tserver.GET(hr, inner, \"/y\", m.h)",
			wantPaths: nil,
		},
		{
			name:      "sibling block const does not leak",
			body:      "if true {\n\t\tconst sib = \"/sib\"\n\t\t_ = sib\n\t}\n\tif true {\n\t\tserver.GET(hr, r, sib, m.h)\n\t}",
			wantPaths: nil,
		},
		{
			name:      "package level concatenation",
			pkgDecls:  "const base = \"/api\"\nconst version = \"/v1\"",
			body:      "server.GET(hr, r, base+version+\"/things\", m.h)",
			wantPaths: []string{"/api/v1/things"},
		},
		{
			name:      "local const concatenated with package const",
			pkgDecls:  "const base = \"/api\"",
			body:      "const leaf = \"/leaf\"\n\tserver.GET(hr, r, base+leaf, m.h)",
			wantPaths: []string{"/api/leaf"},
		},
		{
			name:      "parenthesized concatenation",
			pkgDecls:  "const base = \"/api\"",
			body:      "server.GET(hr, r, base+(\"/a\"+\"/b\"), m.h)",
			wantPaths: []string{"/api/a/b"},
		},
		{
			name:      "non const leaf makes the whole expression unresolved",
			pkgDecls:  "const base = \"/api\"",
			body:      "server.GET(hr, r, base+buildPath(), m.h)",
			wantPaths: nil,
		},
		{
			name:      "var with a literal initializer stays unresolved",
			body:      "var p = \"/varpath\"\n\tserver.GET(hr, r, p, m.h)",
			wantPaths: nil,
		},
		{
			name:      "short variable declaration stays unresolved",
			body:      "p := \"/shortpath\"\n\tserver.GET(hr, r, p, m.h)",
			wantPaths: nil,
		},
		{
			name:      "qualified constant stays unresolved",
			body:      "server.GET(hr, r, other.Path, m.h)",
			wantPaths: nil,
		},
		{
			name:      "non string const is not a path",
			body:      "const n = 42\n\tserver.GET(hr, r, n, m.h)",
			wantPaths: nil,
		},
		{
			name:      "const in a switch case body",
			body:      "switch {\n\tcase true:\n\t\tconst c = \"/case\"\n\t\tserver.GET(hr, r, c, m.h)\n\t}",
			wantPaths: []string{"/case"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, routes := analyzeSingleModule(t, scopedModuleSrc(tt.pkgDecls, tt.body))
			var got []string
			for i := range routes {
				got = append(got, routes[i].Path)
			}
			assert.Equal(t, tt.wantPaths, got)
		})
	}
}

// TestUnresolvedRoutePathWarningNamesConcept pins the reworded diagnostic for
// both registration forms: it must name the Unresolved route concept.
func TestUnresolvedRoutePathWarningNamesConcept(t *testing.T) {
	a, routes := analyzeSingleModule(t, scopedModuleSrc("", "server.GET(hr, r, buildPath(), m.h)"))
	assert.Empty(t, routes)
	require.True(t, containsRawAddSubstr(a.Warnings(t.Context()), "unresolved route: skipping the server.GET registration"),
		"expected a reworded server-verb warning, got: %v", a.Warnings(t.Context()))

	dir := writeAnalyzerProject(t, "module.go", rawAddModuleSrc(
		"r.Add(http.MethodGet, buildPath(), m.ping)"))
	ra := New(dir)
	_, err := ra.AnalyzeProject()
	require.NoError(t, err)
	require.True(t, containsRawAddSubstr(ra.Warnings(t.Context()), "unresolved route: skipping the r.Add registration"),
		"expected a reworded .Add warning, got: %v", ra.Warnings(t.Context()))
}

// TestUnresolvedGroupPrefixWarning pins the diagnostic for a route registered
// on a group whose own prefix could not be resolved: the route is dropped, not
// emitted at a prefix-less path.
func TestUnresolvedGroupPrefixWarning(t *testing.T) {
	a, routes := analyzeSingleModule(t, scopedModuleSrc("",
		"api := r.Group(compute())\n\tserver.GET(hr, api, \"/widgets\", m.h)"))
	assert.Empty(t, routes, "a route on an unresolved group must be dropped")
	require.True(t, containsRawAddSubstr(a.Warnings(t.Context()),
		`unresolved route: skipping the server.GET registration on registrar "api"`),
		"expected an unresolved-group-prefix warning, got: %v", a.Warnings(t.Context()))

	dir := writeAnalyzerProject(t, "module.go", rawAddModuleSrc(
		"api := r.Group(compute())\n\tapi.Add(http.MethodGet, \"/widgets\", m.ping)"))
	ra := New(dir)
	_, err := ra.AnalyzeProject()
	require.NoError(t, err)
	require.True(t, containsRawAddSubstr(ra.Warnings(t.Context()),
		`unresolved route: skipping the api.Add registration on registrar "api"`),
		"expected an unresolved-group-prefix warning for .Add, got: %v", ra.Warnings(t.Context()))
}

func TestConstScopesResolve(t *testing.T) {
	var empty constScopes
	_, found := empty.resolve("x", token.Pos(10))
	assert.False(t, found)

	chain := empty.
		push(map[string]binding{"x": {value: "/outer", ok: true, from: 1}, "y": {value: "/y", ok: true, from: 1}}).
		push(map[string]binding{"x": {value: "/inner", ok: true, from: 5}})

	b, found := chain.resolve("x", token.Pos(10))
	require.True(t, found)
	assert.Equal(t, "/inner", b.value, "the innermost visible binding wins")

	b, found = chain.resolve("x", token.Pos(3))
	require.True(t, found)
	assert.Equal(t, "/outer", b.value, "before the inner declaration, the outer binding is the visible one")

	b, found = chain.resolve("y", token.Pos(10))
	require.True(t, found)
	assert.Equal(t, "/y", b.value)

	_, found = chain.resolve("z", token.Pos(10))
	assert.False(t, found)

	shadow := empty.push(map[string]binding{"n": {from: 1}})
	b, found = shadow.resolve("n", token.Pos(10))
	require.True(t, found, "a valueless local binding is still visible")
	assert.False(t, b.ok, "and it carries no resolvable value")
}

// TestConstScopesPushDoesNotAliasSiblings verifies two scopes pushed onto the
// same chain never see each other's bindings (a shared backing array would
// leak one sibling block's bindings into the next).
func TestConstScopesPushDoesNotAliasSiblings(t *testing.T) {
	base := constScopes(nil).push(map[string]binding{"root": {value: "/root", ok: true, from: 1}})
	left := base.push(map[string]binding{"only": {value: "/left", ok: true, from: 2}})
	right := base.push(map[string]binding{"other": {value: "/right", ok: true, from: 2}})

	b, found := left.resolve("only", token.Pos(10))
	require.True(t, found)
	assert.Equal(t, "/left", b.value)
	_, found = right.resolve("only", token.Pos(10))
	assert.False(t, found, "the right sibling must not see the left sibling's binding")

	assert.Equal(t, base, base.push(nil), "pushing an empty scope is a no-op")
	assert.Equal(t, base, base.push(map[string]binding{}), "pushing an empty scope is a no-op")
}

func TestStmtBindings(t *testing.T) {
	src := `package p
func f() {
	const a = "/a"
	const (
		b = "/b"
		c = 3
		d
	)
	var e = "/e"
	g, _ := h()
	if true {
		const inner = "/inner"
		_ = inner
	}
	type T struct{}
}`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	require.NoError(t, err)
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	require.True(t, ok)

	got := stmtBindings(fn.Body.List)

	resolvable := map[string]string{}
	shadowOnly := []string{}
	for name, b := range got {
		if b.ok {
			resolvable[name] = b.value
			continue
		}
		shadowOnly = append(shadowOnly, name)
	}
	assert.Equal(t, map[string]string{"a": "/a", "b": "/b"}, resolvable,
		"only string constants declared directly in the statement list carry a value")
	assert.ElementsMatch(t, []string{"c", "d", "e", "g"}, shadowOnly,
		"every other direct declaration binds the name without a value; the blank identifier and nested blocks do not")

	for name, b := range got {
		assert.Positive(t, int(b.from), "binding %s must record where it enters scope", name)
	}
}

// TestShortDeclReuseKeepsFirstBinding pins Go's rule that a `:=` naming a
// variable its own block already declares assigns that variable rather than
// declaring a new one: the binding keeps the position where it first entered
// scope, so a use between the two statements still resolves to it.
func TestShortDeclReuseKeepsFirstBinding(t *testing.T) {
	src := `package p
func f() {
	a := x()
	_ = a
	b, a := y()
}`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	require.NoError(t, err)
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	require.True(t, ok)

	got := stmtBindings(fn.Body.List)
	assert.Equal(t, fn.Body.List[0].End(), got["a"].from, "a reused name keeps its first declaration")
	assert.Equal(t, fn.Body.List[2].End(), got["b"].from, "a new name enters scope at its own statement")
}

func TestScopeBindings(t *testing.T) {
	src := `package p
func f() {
	switch x := g(); x {
	case true:
		const c = "/c"
		_ = c
	}
	select {
	case <-ch:
		const s = "/s"
		_ = s
	case msg := <-ch:
		_ = msg
	}
	for i := 0; i < 3; i++ {
	}
	for k, v := range mm {
		_, _ = k, v
	}
	if y := g(); y {
	}
	switch tv := any(z).(type) {
	default:
		_ = tv
	}
}`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	require.NoError(t, err)

	bound := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		scope, introduces := scopeBindings(n)
		if !introduces {
			return true
		}
		for name := range scope {
			bound[name] = true
		}
		return true
	})

	for _, name := range []string{"c", "s", "msg", "i", "k", "v", "y", "x", "tv"} {
		assert.True(t, bound[name], "%s must bind in the scope its statement introduces", name)
	}

	_, introduces := scopeBindings(&ast.Ident{Name: "x"})
	assert.False(t, introduces, "a non-scope node introduces no scope")
}

// TestWalkScopedNilBody guards the nil-body short circuit (a method declared
// without a body).
func TestWalkScopedNilBody(t *testing.T) {
	called := false
	walkScoped(nil, nil, func(ast.Node, constScopes) { called = true })
	assert.False(t, called)
}

// TestCollectConstStrings covers the package-level constant reader directly:
// only a name bound to a string literal is recorded, and a spec that is not a
// value spec contributes nothing.
func TestCollectConstStrings(t *testing.T) {
	src := `package p
import "fmt"
const (
	a = "/a"
	b
	c = 3
)`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	require.NoError(t, err)

	into := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		require.True(t, ok)
		for _, spec := range gen.Specs {
			collectConstStrings(spec, into)
		}
	}
	assert.Equal(t, map[string]string{"a": "/a"}, into,
		"an import spec, an implicit repetition and a non-string value are all skipped")
}

// TestSignatureBindingsShadowPackageConsts covers the names a function
// signature binds: a parameter, a named result, a closure parameter and a
// receiver all shadow a package-level constant, so a route path naming one is
// an Unresolved route rather than the package constant's value.
func TestSignatureBindingsShadowPackageConsts(t *testing.T) {
	tests := []struct {
		name      string
		decls     string
		body      string
		wantPaths []string
	}{
		{
			name: "helper parameter shadows a package const",
			decls: "const p = \"/pkg\"\n" +
				"func (m *Module) sub(hr *server.HandlerRegistry, r server.RouteRegistrar, p string) {\n" +
				"\tserver.GET(hr, r, p, m.h)\n}",
			body:      "m.sub(hr, r, \"/x\")",
			wantPaths: nil,
		},
		{
			name: "named result shadows a package const",
			decls: "const p = \"/pkg\"\n" +
				"func (m *Module) sub(hr *server.HandlerRegistry, r server.RouteRegistrar) (p string) {\n" +
				"\tserver.GET(hr, r, p, m.h)\n\treturn\n}",
			body:      "m.sub(hr, r)",
			wantPaths: nil,
		},
		{
			name:      "closure parameter shadows a package const",
			decls:     "const p = \"/pkg\"",
			body:      "func(p string) {\n\t\tserver.GET(hr, r, p, m.h)\n\t}(\"/x\")",
			wantPaths: nil,
		},
		{
			name: "a differently named parameter leaves the package const alone",
			decls: "const p = \"/pkg\"\n" +
				"func (m *Module) sub(hr *server.HandlerRegistry, r server.RouteRegistrar, other string) {\n" +
				"\tserver.GET(hr, r, p, m.h)\n}",
			body:      "m.sub(hr, r, \"/x\")",
			wantPaths: []string{"/pkg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, routes := analyzeSingleModule(t, scopedModuleSrc(tt.decls, tt.body))
			var got []string
			for i := range routes {
				got = append(got, routes[i].Path)
			}
			assert.Equal(t, tt.wantPaths, got)
		})
	}
}

func TestFuncTypeBindings(t *testing.T) {
	src := `package p
func f(a, b string, _ int) (c string, err error) { return }`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	require.NoError(t, err)
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	require.True(t, ok)

	scope := funcTypeBindings(fn.Type, "m", fn.Body.Pos())

	names := make([]string, 0, len(scope))
	for name, b := range scope {
		names = append(names, name)
		assert.False(t, b.ok, "a signature binding never carries a constant value")
	}
	assert.ElementsMatch(t, []string{"a", "b", "c", "err", "m"}, names,
		"parameters, named results and the receiver bind; the blank identifier does not")

	assert.Empty(t, funcTypeBindings(nil, "", token.NoPos),
		"a missing signature and an unnamed receiver bind nothing")
	assert.Empty(t, funcTypeBindings(nil, "_", token.NoPos),
		"a blank receiver binds nothing")
}

// TestEmptyStringConstResolves covers the empty-string constant: `const p = ""`
// is a resolvable value, not a missing one, so a path folded on it keeps its
// literal part and a group opened on it behaves like a group with no prefix.
func TestEmptyStringConstResolves(t *testing.T) {
	tests := []struct {
		name      string
		pkgDecls  string
		body      string
		wantPaths []string
	}{
		{
			name:      "local empty const folds into the path",
			body:      "const prefix = \"\"\n\tserver.GET(hr, r, prefix+\"/health\", m.h)",
			wantPaths: []string{"/health"},
		},
		{
			name:      "package level empty const folds into the path",
			pkgDecls:  "const prefix = \"\"",
			body:      "server.GET(hr, r, prefix+\"/health\", m.h)",
			wantPaths: []string{"/health"},
		},
		{
			name:      "group on an empty const carries no prefix and is not unresolved",
			body:      "const prefix = \"\"\n\tapi := r.Group(prefix)\n\tserver.GET(hr, api, \"/widgets\", m.h)",
			wantPaths: []string{"/widgets"},
		},
		{
			name:      "empty const alone is a valid root path",
			body:      "const prefix = \"\"\n\tserver.GET(hr, r, prefix, m.h)",
			wantPaths: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, routes := analyzeSingleModule(t, scopedModuleSrc(tt.pkgDecls, tt.body))
			var got []string
			for i := range routes {
				got = append(got, routes[i].Path)
			}
			assert.Equal(t, tt.wantPaths, got)
			assert.False(t, containsRawAddSubstr(a.Warnings(t.Context()), "unresolved route"),
				"an empty string constant must not read as unresolved: %v", a.Warnings(t.Context()))
		})
	}
}

// TestHelperInheritsCallerGroupPrefix verifies a helper called with an already
// grouped registrar accumulates its own groups onto the inherited prefix
// instead of restarting from the root.
func TestHelperInheritsCallerGroupPrefix(t *testing.T) {
	_, routes := analyzeSingleModule(t, scopedModuleSrc(
		"func (m *Module) sub(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n"+
			"\tadmin := r.Group(\"/admin\")\n"+
			"\tserver.GET(hr, admin, \"/users\", m.h)\n"+
			"\tserver.GET(hr, r, \"/ping\", m.h)\n}",
		"api := r.Group(\"/api\")\n\tm.sub(hr, api)"))

	got := make([]string, 0, len(routes))
	for i := range routes {
		got = append(got, routes[i].Path)
	}
	assert.Equal(t, []string{"/api/admin/users", "/api/ping"}, got,
		"a sub-group inside a helper must nest under the prefix its caller passed in")
}

// TestShadowIndex verifies the index answers "is this name locally declared
// here" over the same block model constant resolution uses: a declaration
// covers its own block from its position onward, and nothing outside it.
func TestShadowIndex(t *testing.T) {
	src := `package api

func handle(http string) error {
	if cond {
		server := fake()
		_ = server
	}
	x := 1
	_ = x
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "api.go", src, parser.ParseComments)
	require.NoError(t, err)
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	require.True(t, ok)
	idx := newShadowIndex(fn)

	body := fn.Body
	ifStmt, ok := body.List[0].(*ast.IfStmt)
	require.True(t, ok)
	assert.True(t, idx.shadows("http", body.Pos()), "a parameter shadows over the whole body")
	assert.True(t, idx.shadows("server", ifStmt.Body.End()-1), "a block declaration shadows inside its block")
	assert.False(t, idx.shadows("server", body.End()-1), "and nowhere outside it")
	assert.False(t, idx.shadows("x", body.List[0].Pos()), "a declaration shadows nothing above itself")
	assert.True(t, idx.shadows("x", body.End()-1), "and everything below it")
	assert.False(t, idx.shadows("absent", body.Pos()), "an undeclared name is never shadowed")
}

// TestShadowIndexNoBody verifies a body-less declaration indexes to nothing and
// the nil index answers every query false.
func TestShadowIndexNoBody(t *testing.T) {
	assert.Nil(t, newShadowIndex(&ast.FuncDecl{Name: ast.NewIdent("h")}))
	assert.Nil(t, newShadowIndex(nil))
	assert.False(t, shadowIndex(nil).shadows("server", token.NoPos))
}
