package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headersPreamble opens a handler returning server.Result[Resp] with unnamed
// results; a test appends the body and the closing brace. The source is only
// parsed, so the unresolved names and unused imports are fine.
const headersPreamble = `package mod

import (
	"net/http"
	nethttp "net/http"

	"github.com/labstack/echo/v4"
	"github.com/gaborage/go-bricks/server"
)

const hdrLocation = "Location"

func handle(req Req, ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) {
`

// headersForSource parses a whole file and runs extractResponseHeaders over
// its first function declaration under the given server import aliases.
func headersForSource(t *testing.T, src string, aliases map[string]struct{}) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "handler.go", src, parser.ParseComments)
	require.NoError(t, err)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return New(t.TempDir()).extractResponseHeaders(fn, aliases)
		}
	}
	require.Fail(t, "source declares no function")
	return nil
}

// headersForBody wraps a handler body in headersPreamble.
func headersForBody(t *testing.T, body string) []string {
	t.Helper()
	return headersForSource(t, headersPreamble+body+"\n}\n", nil)
}

// TestExtractResponseHeadersWriteShapes covers each recognized write shape on
// a result identifier.
func TestExtractResponseHeadersWriteShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"assign_http_header_literal", `res := server.Created(req)
	res.Headers = http.Header{"Location": []string{"/x"}}
	return res, nil`, []string{"Location"}},
		{"assign_aliased_net_http_literal", `res := server.Created(req)
	res.Headers = nethttp.Header{"Location": {"/x"}}
	return res, nil`, []string{"Location"}},
		{"assign_map_literal", `res := server.Created(req)
	res.Headers = map[string][]string{"Location": {"/x"}, "X-Trace": {"t"}}
	return res, nil`, []string{"Location", "X-Trace"}},
		{"set_after_make", `res := server.Created(req)
	res.Headers = make(http.Header)
	res.Headers.Set("Location", "/x")
	return res, nil`, []string{"Location"}},
		{"add", `res := server.Accepted(req)
	res.Headers = http.Header{}
	res.Headers.Add("Retry-After", "30")
	return res, nil`, []string{"Retry-After"}},
		{"index_assign", `res := server.Created(req)
	res.Headers = http.Header{}
	res.Headers["Location"] = []string{"/x"}
	return res, nil`, []string{"Location"}},
		{"bound_result_literal", `res := server.Result[Resp]{Status: http.StatusCreated, Headers: http.Header{"Location": {"/x"}}}
	return res, nil`, []string{"Location"}},
		{"var_bound_result_literal", `var res = server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}
	return res, nil`, []string{"Location"}},
		{"bound_with_meta_literal", `res := server.ResultWithMeta[Resp]{Status: http.StatusCreated, Headers: http.Header{"Location": {"/x"}}}
	return res, nil`, []string{"Location"}},
		{"returned_literal", `return server.Result[Resp]{Status: http.StatusCreated, Headers: http.Header{"Location": {"/x"}}}, nil`, []string{"Location"}},
		{"returned_address_of_literal", `return &server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}, nil`, []string{"Location"}},
		{"returned_address_of_ident", `res := server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	return &res, nil`, []string{"Location"}},
		{"bound_address_of_literal", `res := &server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}
	res.Headers.Set("ETag", "v1")
	return res, nil`, []string{"Etag", "Location"}},
		{"parenthesized_selector", `res := server.Created(req)
	(res).Headers.Set("Location", "/x")
	return res, nil`, []string{"Location"}},
		{"branch_write", `res := server.Created(req)
	res.Headers = http.Header{}
	if req.ID != "" {
		res.Headers.Set("ETag", "v1")
	}
	for range 1 {
		res.Headers.Add("X-Loop", "1")
	}
	return res, nil`, []string{"Etag", "X-Loop"}},
		{"ambiguous_status_still_yields_headers", `res := server.Created(req)
	if req.ID == "" {
		res = server.Accepted(req)
	}
	res.Headers = http.Header{"Location": {"/x"}}
	return res, nil`, []string{"Location"}},
		{"no_write", `return server.Created(req), nil`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
}

// TestExtractResponseHeadersNamedResults covers the second result-identifier
// rule: the first named result counts only when the body has a bare return.
func TestExtractResponseHeadersNamedResults(t *testing.T) {
	const head = `package mod

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

func handle(req Req, ctx server.HandlerContext) (res server.Result[Resp], apiErr server.IAPIError) {
`
	t.Run("bare_return", func(t *testing.T) {
		got := headersForSource(t, head+`	res = server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	return
}
`, nil)
		assert.Equal(t, []string{"Location"}, got)
	})
	t.Run("no_bare_return_and_not_returned", func(t *testing.T) {
		got := headersForSource(t, head+`	res = server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	return server.Created(req), nil
}
`, nil)
		assert.Nil(t, got)
	})
	t.Run("bare_return_inside_closure_does_not_count", func(t *testing.T) {
		got := headersForSource(t, head+`	res.Headers = http.Header{"Location": {"/x"}}
	f := func() { return }
	f()
	return server.Created(req), nil
}
`, nil)
		assert.Nil(t, got)
	})
}

// TestExtractResponseHeadersKeys covers which keys are documented and how
// they are normalized: only string literals, canonicalized, deduplicated,
// sorted, with Content-Type and the empty name dropped.
func TestExtractResponseHeadersKeys(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"lowercase_canonicalized", `res := server.Created(req)
	res.Headers = http.Header{"location": {"/x"}}
	return res, nil`, []string{"Location"}},
		{"duplicates_collapse", `res := server.Created(req)
	res.Headers = http.Header{"location": {"/x"}}
	res.Headers.Set("LOCATION", "/y")
	res.Headers.Add("Location", "/z")
	return res, nil`, []string{"Location"}},
		{"sorted", `res := server.Created(req)
	res.Headers = http.Header{}
	res.Headers.Set("X-B", "1")
	res.Headers.Set("Etag", "1")
	res.Headers.Set("Location", "1")
	return res, nil`, []string{"Etag", "Location", "X-B"}},
		{"content_type_dropped", `res := server.Created(req)
	res.Headers = http.Header{"content-type": {"text/plain"}}
	res.Headers.Set("Content-Type", "text/plain")
	return res, nil`, nil},
		{"empty_name_dropped", `res := server.Created(req)
	res.Headers = http.Header{"": {"x"}}
	return res, nil`, nil},
		{"constant_keys_ignored", `res := server.Created(req)
	res.Headers = http.Header{echo.HeaderLocation: {"/x"}, hdrLocation: {"/x"}}
	res.Headers.Set(echo.HeaderETag, "v")
	res.Headers[hdrLocation] = []string{"/x"}
	return res, nil`, nil},
		{"variable_and_call_keys_ignored", `res := server.Created(req)
	k := "Location"
	res.Headers = http.Header{}
	res.Headers.Set(k, "/x")
	res.Headers.Add(name(), "/x")
	return res, nil`, nil},
		{"del_ignored", `res := server.Created(req)
	res.Headers = http.Header{}
	res.Headers.Del("Location")
	return res, nil`, nil},
		{"literal_value_not_composite_ignored", `res := server.Created(req)
	res.Headers = make(http.Header)
	return res, nil`, nil},
		{"call_without_args_ignored", `res := server.Created(req)
	res.Headers.Set()
	return res, nil`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
}

// TestExtractResponseHeadersSkips covers what is never documented: writes in
// function literals, writes to a local that is not a result identifier, a
// returned literal whose error is not nil, and a literal whose qualifier is a
// local shadowing the server package.
func TestExtractResponseHeadersSkips(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"called_closure", `res := server.Created(req)
	f := func() { res.Headers = http.Header{"Location": {"/x"}} }
	f()
	return res, nil`},
		{"deferred_closure", `res := server.Created(req)
	res.Headers = http.Header{}
	defer func() { res.Headers.Set("Location", "/x") }()
	return res, nil`},
		{"goroutine", `res := server.Created(req)
	res.Headers = http.Header{}
	go func() { res.Headers["Location"] = []string{"/x"} }()
	return res, nil`},
		{"closure_returned_literal", `f := func() (server.Result[Resp], error) {
		return server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}, nil
	}
	_ = f
	return server.Created(req), nil`},
		{"closure_return_ident_not_result", `other := server.Created(req)
	other.Headers = http.Header{"Location": {"/x"}}
	f := func() server.Result[Resp] { return other }
	_ = f
	return server.Created(req), nil`},
		{"non_result_local", `other := server.Created(req)
	other.Headers = http.Header{"Location": {"/x"}}
	other.Headers.Set("ETag", "v")
	return server.Created(req), nil`},
		{"unbound_literal", `_ = server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}
	return server.Created(req), nil`},
		{"literal_bound_to_non_result", `other := server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}
	_ = other
	return server.Created(req), nil`},
		{"returned_literal_with_error", `return server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}, errNope`},
		{"headers_element_not_composite", `return server.Result[Resp]{Headers: make(http.Header)}, nil`},
		{"non_wrapper_literal", `return server.Other[Resp]{Headers: http.Header{"Location": {"/x"}}}, nil`},
		{"shadowed_server_literal", `server := pkg{}
	return server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}, nil`},
		{"other_field_literal", `res := server.Created(req)
	res.Meta = map[string]any{"Location": "/x"}
	res.Trailers.Set("Location", "/x")
	res.Trailers["Location"] = []string{"/x"}
	var a, b = pair()
	const c = "Location"
	_, _ = a, b
	return res, nil`},
		{"positional_and_other_elements", `var res = server.Result[Resp]{Data: Resp{}, Headers: make(http.Header)}
	return res, nil`},
		{"shadowed_server_bound_literal", `server := pkg{}
	res := server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}
	return res, nil`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Nil(t, headersForBody(t, tt.body))
		})
	}
}

// TestExtractResponseHeadersWrapperGate covers the handler gate: only a
// handler whose declared first result is a Result wrapper, by value or behind
// one pointer and under any server import alias, collects headers.
func TestExtractResponseHeadersWrapperGate(t *testing.T) {
	const body = `	res := build(req)
	res.Headers = http.Header{"Location": {"/x"}}
	return res, nil
}
`
	sig := func(imp, result string) string {
		return "package mod\n\nimport (\n\t\"net/http\"\n\n\t" + imp + "\n)\n\nfunc handle(req Req) (" + result + ", error) {\n" + body
	}
	const plain = `"github.com/gaborage/go-bricks/server"`
	const aliased = `srv "github.com/gaborage/go-bricks/server"`
	srvAliases := map[string]struct{}{"srv": {}}
	tests := []struct {
		name    string
		src     string
		aliases map[string]struct{}
		want    []string
	}{
		{"result", sig(plain, "server.Result[Resp]"), nil, []string{"Location"}},
		{"result_with_meta", sig(plain, "server.ResultWithMeta[Resp]"), nil, []string{"Location"}},
		{"pointer_result", sig(plain, "*server.Result[Resp]"), nil, []string{"Location"}},
		{"aliased_import", sig(aliased, "srv.Result[Resp]"), srvAliases, []string{"Location"}},
		{"bare_payload", sig(plain, "Resp"), nil, nil},
		{"no_content_result", sig(plain, "server.NoContentResult"), nil, nil},
		{"non_server_result", sig(plain, "other.Result[Resp]"), nil, nil},
		{"no_results", "package mod\n\nfunc handle() {\n\tres.Headers = http.Header{\"Location\": {\"/x\"}}\n}\n", nil, nil},
		{"no_body", "package mod\n\nimport \"github.com/gaborage/go-bricks/server\"\n\nfunc handle() server.Result[Resp]\n", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForSource(t, tt.src, tt.aliases))
		})
	}
}

// TestResponseHeadersReachRoute pins the plumbing: a typed registration's
// route carries its handler's header names, and a raw Add registration of
// the same handler carries none.
func TestResponseHeadersReachRoute(t *testing.T) {
	src := `package mod

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Item struct {
	ID string ` + "`json:\"id\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/items", m.create)
	r.Add(http.MethodPost, "/raw", m.create)
}

func (m *Module) create(ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{})
	res.Headers = http.Header{"location": {"/items/1"}}
	return res, nil
}
`
	a, routes := analyzeSingleModule(t, src)
	assert.Equal(t, []string{"Location"}, routeForPath(t, routes, "POST /items").ResponseHeaders)
	assert.Nil(t, routeForPath(t, routes, "POST /raw").ResponseHeaders)
	warnings := a.Warnings()
	assert.Empty(t, warnings, strings.Join(warnings, "\n"))
}
