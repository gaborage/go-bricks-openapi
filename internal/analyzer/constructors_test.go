package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inferredHandlerName is the function every inference fixture declares as the
// handler under test; a fixture's other declarations are helpers.
const inferredHandlerName = "h"

// inferredErrorsFor parses src as a whole file and runs inferErrorStatuses over
// its handler function, with the file's own server import aliases.
func inferredErrorsFor(t *testing.T, src string) []int {
	t.Helper()
	a := New(t.TempDir())
	file, err := parser.ParseFile(a.fileSet, "api.go", src, parser.ParseComments)
	require.NoError(t, err)
	aliases := a.extractImportAliases(file, serverImportPath)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != inferredHandlerName {
			continue
		}
		return a.inferErrorStatuses(fn, aliases)
	}
	t.Fatalf("function %q not found in source", inferredHandlerName)
	return nil
}

const inferSrcHeader = `package api

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

`

// TestInferErrorStatusesConstructorTable pins every go-bricks v0.53.0 error
// constructor to the status it produces.
func TestInferErrorStatusesConstructorTable(t *testing.T) {
	tests := []struct {
		constructor string
		want        int
	}{
		{"NewBadRequestError", 400},
		{"NewValidationError", 400},
		{"NewUnauthorizedError", 401},
		{"NewForbiddenError", 403},
		{"NewNotFoundError", 404},
		{"NewConflictError", 409},
		{"NewBusinessLogicError", 422},
		{"NewTooManyRequestsError", 429},
		{"NewInternalServerError", 500},
		{"NewServiceUnavailableError", 503},
	}
	for _, tt := range tests {
		t.Run(tt.constructor, func(t *testing.T) {
			src := inferSrcHeader + `func h() error {
	return server.` + tt.constructor + `("boom")
}`
			assert.Equal(t, []int{tt.want}, inferredErrorsFor(t, src))
		})
	}
}

// TestInferErrorStatusesMultipleSortedUnique verifies a handler calling several
// constructors contributes each status once, in ascending order.
func TestInferErrorStatusesMultipleSortedUnique(t *testing.T) {
	src := inferSrcHeader + `func h(id string) error {
	if id == "" {
		return server.NewConflictError("dup")
	}
	if id == "x" {
		return server.NewNotFoundError("gone")
	}
	return server.NewConflictError("dup again")
}`
	assert.Equal(t, []int{404, 409}, inferredErrorsFor(t, src))
}

// TestInferErrorStatusesNestedScopes verifies calls inside an if block and
// inside a closure declared in the body both count: the handler body is walked
// in full, closures included.
func TestInferErrorStatusesNestedScopes(t *testing.T) {
	src := inferSrcHeader + `func h(id string) error {
	check := func() error {
		return server.NewForbiddenError("nope")
	}
	if id == "" {
		return server.NewNotFoundError("gone")
	}
	return check()
}`
	assert.Equal(t, []int{403, 404}, inferredErrorsFor(t, src))
}

// TestInferErrorStatusesHelperNotFollowed pins the documented limit: a
// constructor called from a helper the handler calls contributes nothing.
func TestInferErrorStatusesHelperNotFollowed(t *testing.T) {
	src := inferSrcHeader + `func helper() error {
	return server.NewConflictError("dup")
}

func h() error {
	return helper()
}`
	assert.Empty(t, inferredErrorsFor(t, src))
}

// TestInferErrorStatusesBaseAPIError verifies the third argument of
// NewBaseAPIError is resolved as a status expression, and that an unresolvable
// or out-of-range one contributes nothing.
func TestInferErrorStatusesBaseAPIError(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want []int
	}{
		{name: "int literal", arg: "410", want: []int{410}},
		{name: "http constant", arg: "http.StatusGone", want: []int{410}},
		{name: "variable", arg: "code", want: nil},
		{name: "computed", arg: "codeFor()", want: nil},
		{name: "out of range", arg: "http.StatusOK", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := inferSrcHeader + `func h(code int) error {
	return server.NewBaseAPIError("GONE", "gone", ` + tt.arg + `)
}`
			got := inferredErrorsFor(t, src)
			if tt.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestInferErrorStatusesTooFewArgs verifies a NewBaseAPIError call missing its
// status argument is ignored rather than panicking.
func TestInferErrorStatusesTooFewArgs(t *testing.T) {
	src := inferSrcHeader + `func h() error {
	return server.NewBaseAPIError("GONE")
}`
	assert.Empty(t, inferredErrorsFor(t, src))
}

// TestInferErrorStatusesNonServerCalls verifies only calls on the go-bricks
// server package selector count: a same-named constructor on another package,
// a bare function call, and a method call all contribute nothing.
func TestInferErrorStatusesNonServerCalls(t *testing.T) {
	src := inferSrcHeader + `func h(svc *Service) error {
	NewNotFoundError("local")
	errs.NewConflictError("other package")
	svc.NewNotFoundError("method")
	return nil
}`
	assert.Empty(t, inferredErrorsFor(t, src))
}

// TestInferErrorStatusesAliasedImport verifies an aliased server import is
// honoured and the literal "server" qualifier is then not.
func TestInferErrorStatusesAliasedImport(t *testing.T) {
	src := `package api

import srv "github.com/gaborage/go-bricks/server"

func h() error {
	server.NewConflictError("not the framework")
	return srv.NewNotFoundError("gone")
}`
	assert.Equal(t, []int{404}, inferredErrorsFor(t, src))
}

// TestInferErrorStatusesNoBody verifies a body-less declaration (an external
// function) yields nothing.
func TestInferErrorStatusesNoBody(t *testing.T) {
	a := New(t.TempDir())
	fn := &ast.FuncDecl{Name: ast.NewIdent("h")}
	assert.Empty(t, a.inferErrorStatuses(fn, nil))
}

// TestStatusForConstructorIgnoresErrorConstructors verifies success-status
// resolution is unchanged by the shared table: an error constructor never
// documents a success status.
func TestStatusForConstructorIgnoresErrorConstructors(t *testing.T) {
	assert.Equal(t, 0, statusForConstructor("NewNotFoundError", nil, nil))
	assert.Equal(t, 0, statusForConstructor("NewBaseAPIError", []ast.Expr{
		&ast.BasicLit{Kind: token.STRING, Value: `"GONE"`},
		&ast.BasicLit{Kind: token.STRING, Value: `"gone"`},
		&ast.BasicLit{Kind: token.INT, Value: "410"},
	}, nil))
}

// TestHTTPStatusConstantsCoverErrorRange verifies the widened constant set
// resolves the 4xx/5xx names an error constructor argument idiomatically uses.
func TestHTTPStatusConstantsCoverErrorRange(t *testing.T) {
	for name, want := range map[string]int{
		"StatusNotFound":            404,
		"StatusConflict":            409,
		"StatusUnprocessableEntity": 422,
		"StatusTooManyRequests":     429,
		"StatusInternalServerError": 500,
		"StatusServiceUnavailable":  503,
	} {
		assert.Equal(t, want, httpStatusConstants[name], name)
	}
}

// TestMergeErrorStatuses verifies the union helper sorts, deduplicates, and
// leaves the existing set untouched when nothing is added.
func TestMergeErrorStatuses(t *testing.T) {
	assert.Equal(t, []int{404, 409}, mergeErrorStatuses([]int{409}, []int{404, 409}))
	assert.Equal(t, []int{404}, mergeErrorStatuses(nil, []int{404, 404}))
	assert.Nil(t, mergeErrorStatuses(nil, nil))
	existing := []int{409, 404}
	assert.Equal(t, []int{409, 404}, mergeErrorStatuses(existing, nil), "no additions leaves the set as-is")
}

// TestRouteErrorStatusesUnionWithDirective verifies the end-to-end Survey: the
// statuses inferred from a handler's constructor calls union with the ones its
// `//openapi:errors` Directive declares, deduplicated and sorted, while a route
// whose handler calls nothing keeps only what it declares.
func TestRouteErrorStatusesUnionWithDirective(t *testing.T) {
	src := `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{}
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (m *Module) get(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	if false {
		return server.Result[Thing]{}, server.NewNotFoundError("gone")
	}
	return server.Result[Thing]{}, server.NewConflictError("dup")
}
func (m *Module) list(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	return server.NewResult(200, Thing{}), nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:errors 409, 422
	server.GET(hr, r, "/things/:id", m.get)
	//openapi:errors 404
	server.GET(hr, r, "/things", m.list)
}
`
	_, routes := analyzeSingleModule(t, src)
	byPath := map[string][]int{}
	for i := range routes {
		byPath[routes[i].Path] = routes[i].ErrorStatuses
	}
	assert.Equal(t, []int{404, 409, 422}, byPath["/things/{id}"], "inferred 404+409 union the declared 409+422")
	assert.Equal(t, []int{404}, byPath["/things"], "a handler calling no constructor keeps only the declaration")
}

// TestInferErrorStatusesShadowedQualifier verifies a package qualifier the
// handler body declares itself names that local, not the import: the call is
// not a framework call and contributes no status. A declaration positioned
// after the call shadows nothing there, matching Go's own scoping.
func TestInferErrorStatusesShadowedQualifier(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []int
	}{
		{
			name: "server shadowed by a short declaration",
			src: inferSrcHeader + `func h() error {
	server := newFake()
	return server.NewNotFoundError("gone")
}`,
		},
		{
			name: "server shadowed by a parameter",
			src: inferSrcHeader + `func h(server *fake) error {
	return server.NewNotFoundError("gone")
}`,
		},
		{
			name: "server shadowed by a var declaration in an inner block",
			src: inferSrcHeader + `func h(id string) error {
	if id == "" {
		var server fake
		return server.NewConflictError("dup")
	}
	return nil
}`,
		},
		{
			name: "http shadowed by a local var",
			src: inferSrcHeader + `func h() error {
	http := newFakeClient()
	_ = http
	return server.NewBaseAPIError("GONE", "gone", http.StatusGone)
}`,
		},
		{
			name: "shadow declared after the call does not suppress it",
			src: inferSrcHeader + `func h() error {
	if err := server.NewNotFoundError("gone"); err != nil {
		return err
	}
	server := newFake()
	_ = server
	return nil
}`,
			want: []int{404},
		},
		{
			name: "shadow in a sibling block does not suppress the outer call",
			src: inferSrcHeader + `func h(id string) error {
	if id == "" {
		server := newFake()
		_ = server
	}
	return server.NewConflictError("dup")
}`,
			want: []int{409},
		},
		{
			name: "unshadowed control case",
			src: inferSrcHeader + `func h() error {
	return server.NewBaseAPIError("GONE", "gone", http.StatusGone)
}`,
			want: []int{410},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferredErrorsFor(t, tt.src)
			if tt.want == nil {
				assert.Empty(t, got, "a shadowed qualifier names a local, not the package")
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
