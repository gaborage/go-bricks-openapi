package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handlerPreamble is the file header every success-status fixture body is
// wrapped in: a handler whose signature matches the framework's shape. The
// source is only parsed, never type-checked, so the unresolved Req/Resp names
// and the unused net/http import are fine.
const handlerPreamble = `package mod

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

func handle(req Req, ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) {
`

// statusForSource parses a whole file and runs extractSuccessStatus over its
// first function declaration under the given server import aliases.
func statusForSource(t *testing.T, src string, aliases map[string]struct{}) int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "handler.go", src, parser.ParseComments)
	require.NoError(t, err)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return New(t.TempDir()).extractSuccessStatus(fn, aliases)
		}
	}
	require.Fail(t, "source declares no function")
	return 0
}

// statusForHandlerBody wraps a handler body in the standard preamble and
// resolves its success status with no explicit import aliases (the literal
// "server" qualifier).
func statusForHandlerBody(t *testing.T, body string) int {
	t.Helper()
	return statusForSource(t, handlerPreamble+body+"\n}\n", nil)
}

// TestExtractSuccessStatusDirectReturn locks the pre-existing behaviour: a
// return whose first result is the constructor call itself.
func TestExtractSuccessStatusDirectReturn(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"accepted", "\treturn server.Accepted(req), nil", 202},
		{"created", "\treturn server.Created(req), nil", 201},
		{"no_content", "\treturn server.NoContent(), nil", 204},
		{"new_result_const", "\treturn server.NewResult(http.StatusAccepted, req), nil", 202},
		{"new_result_literal", "\treturn server.NewResult(201, req), nil", 201},
		{"unrecognized", "\treturn buildIt(req), nil", 0},
		{"bare_return", "\treturn", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForHandlerBody(t, tt.body))
		})
	}
}

// TestExtractSuccessStatusBoundToConstructor covers a returned identifier that
// was bound to a recognised constructor earlier in the same body — the defect
// this change fixes (issue #67).
func TestExtractSuccessStatusBoundToConstructor(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"define",
			"\tres := server.Created(req)\n\treturn res, nil",
			201,
		},
		{
			"define_then_mutated_headers",
			"\tres := server.Accepted(req)\n\tres.Headers = http.Header{}\n\treturn res, nil",
			202,
		},
		{
			"var_decl",
			"\tvar res = server.Accepted(req)\n\treturn res, nil",
			202,
		},
		{
			"new_result",
			"\tres := server.NewResult(http.StatusAccepted, req)\n\treturn res, nil",
			202,
		},
		{
			// A const declaration and a value-less var spec are both skipped; the
			// `var res = ...` binding between them still resolves.
			"var_block_with_noise",
			"\tconst c = 1\n\tvar res = server.Accepted(req)\n\tvar unused server.Result[Resp]\n\t_ = unused\n\treturn res, nil",
			202,
		},
		{
			// A compound assignment is not a binding, so the `:=` above still stands.
			"compound_assignment_is_not_a_binding",
			"\tres := server.Created(req)\n\tcount := 0\n\tcount += 1\n\t_ = count\n\treturn res, nil",
			201,
		},
		{
			"non_server_call",
			"\tres := buildIt(req)\n\treturn res, nil",
			0,
		},
		{
			"unbound_ident",
			"\treturn res, nil",
			0,
		},
		{
			"multi_value_binding",
			"\tres, err := build(req)\n\treturn res, err",
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForHandlerBody(t, tt.body))
		})
	}
}

// TestExtractSuccessStatusCompositeLiteral covers a result built by hand. A
// literal without a Status key is an explicit 200 (the framework's own zero
// behaviour), distinguishable in-process from the 0 "unknown" fallback even
// though both emit "200".
func TestExtractSuccessStatusCompositeLiteral(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"no_status_key",
			"\tres := server.Result[Resp]{Data: req}\n\treturn res, nil",
			200,
		},
		{
			"status_key_const",
			"\tres := server.Result[Resp]{Status: http.StatusCreated, Data: req}\n\treturn res, nil",
			201,
		},
		{
			"status_key_literal",
			"\tres := server.Result[Resp]{Status: 202, Data: req}\n\treturn res, nil",
			202,
		},
		{
			"status_key_unresolvable",
			"\tres := server.Result[Resp]{Status: code, Data: req}\n\treturn res, nil",
			0,
		},
		{
			"result_with_meta",
			"\tres := server.ResultWithMeta[Resp]{Status: http.StatusCreated}\n\treturn res, nil",
			201,
		},
		{
			// Defensive: a multi-type-argument generic parses as *ast.IndexListExpr,
			// the sibling of the single-argument *ast.IndexExpr.
			"multi_type_argument_generic",
			"\tres := server.ResultWithMeta[Resp, Meta]{Status: http.StatusCreated}\n\treturn res, nil",
			201,
		},
		{
			"positional_literal",
			"\tres := server.Result[Resp]{req}\n\treturn res, nil",
			200,
		},
		{
			"non_server_literal",
			"\tres := Envelope[Resp]{Status: http.StatusCreated}\n\treturn res, nil",
			0,
		},
		{
			// NoContentResult has no Status field; it is 204 by construction, the
			// same as server.NoContent().
			"no_content_result_literal",
			"\tres := server.NoContentResult{}\n\treturn res, nil",
			204,
		},
		{
			"non_server_no_content_literal",
			"\tres := NoContentResult{}\n\treturn res, nil",
			0,
		},
		{
			// A doubly-qualified type is not a package-level server reference.
			"nested_selector_no_content_literal",
			"\tres := pkg.server.NoContentResult{}\n\treturn res, nil",
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForHandlerBody(t, tt.body))
		})
	}
}

// TestExtractSuccessStatusFieldWrite covers a later `<ident>.Status = ...`
// write, which overrides the status implied by the binding.
func TestExtractSuccessStatusFieldWrite(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"literal_then_write",
			"\tres := server.Result[Resp]{Data: req}\n\tres.Status = http.StatusCreated\n\treturn res, nil",
			201,
		},
		{
			"constructor_then_write",
			"\tres := server.Created(req)\n\tres.Status = http.StatusAccepted\n\treturn res, nil",
			202,
		},
		{
			"last_write_wins",
			"\tres := server.Result[Resp]{Data: req}\n\tres.Status = 201\n\tres.Status = 202\n\treturn res, nil",
			202,
		},
		{
			"unresolvable_write_clears_status",
			"\tres := server.Created(req)\n\tres.Status = code\n\treturn res, nil",
			0,
		},
		{
			"write_to_other_field_ignored",
			"\tres := server.Created(req)\n\tres.Data = req\n\treturn res, nil",
			201,
		},
		{
			"write_to_unbound_ident_ignored",
			"\tother.Status = 202\n\tres := server.Created(req)\n\treturn res, nil",
			201,
		},
		{
			"write_through_nested_selector_ignored",
			"\tres := server.Created(req)\n\tm.res.Status = 202\n\treturn res, nil",
			201,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForHandlerBody(t, tt.body))
		})
	}
}

// TestExtractSuccessStatusControlFlowDependent pins the conservative fallback
// for control-flow-dependent bindings and writes: only assignments that are
// statements of the handler body's own statement list are honoured. A write or
// binding nested in an if/for/switch body may or may not execute, so documenting
// its status would be a guess — the binding is invalidated instead.
func TestExtractSuccessStatusControlFlowDependent(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"conditional_status_write",
			"\tres := server.Accepted(req)\n\tif req.Name == \"\" {\n\t\tres.Status = http.StatusCreated\n\t}\n\treturn res, nil",
			0,
		},
		{
			"conditional_binding",
			"\tif req.Name == \"\" {\n\t\tres := server.Created(req)\n\t\t_ = res\n\t}\n\tres := server.Accepted(req)\n\treturn res, nil",
			0,
		},
		{
			"status_write_inside_loop",
			"\tres := server.Accepted(req)\n\tfor range req.Items {\n\t\tres.Status = 201\n\t}\n\treturn res, nil",
			0,
		},
		{
			"status_write_inside_switch_case",
			"\tres := server.Accepted(req)\n\tswitch req.Name {\n\tcase \"\":\n\t\tres.Status = 201\n\t}\n\treturn res, nil",
			0,
		},
		{
			"multi_value_reassignment_invalidates",
			"\tres := server.Accepted(req)\n\tres, err = build(req)\n\treturn res, err",
			0,
		},
		{
			// Top-level binding plus top-level write still resolves — the guard must
			// not swallow the unconditional shape this feature exists for.
			"top_level_binding_and_write",
			"\tres := server.Accepted(req)\n\tres.Status = http.StatusCreated\n\treturn res, nil",
			201,
		},
		{
			// A nested const declaration is not a binding and must not invalidate.
			"nested_const_decl_survives",
			"\tres := server.Accepted(req)\n\tif req.Name == \"\" {\n\t\tconst c = 1\n\t\t_ = c\n\t}\n\treturn res, nil",
			202,
		},
		{
			// A conditional write to a field other than Status cannot change the
			// status, so the binding survives.
			"conditional_header_write_survives",
			"\tres := server.Accepted(req)\n\tif req.Name == \"\" {\n\t\tres.Headers = http.Header{}\n\t}\n\treturn res, nil",
			202,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForHandlerBody(t, tt.body))
		})
	}
}

// TestExtractSuccessStatusAmbiguousBinding pins the conservative fallback: an
// identifier bound more than once yields 0 and the generator's default 200. No
// branch-sensitive flow analysis is attempted.
func TestExtractSuccessStatusAmbiguousBinding(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"reassigned_in_branch",
			"\tres := server.Created(req)\n\tif req.Name == \"\" {\n\t\tres = server.Accepted(req)\n\t}\n\treturn res, nil",
			0,
		},
		{
			"defined_twice_in_branches",
			"\tif req.Name == \"\" {\n\t\tres := server.Created(req)\n\t\treturn res, nil\n\t}\n\tres := server.Accepted(req)\n\treturn res, nil",
			0,
		},
		{
			"status_write_after_ambiguity_ignored",
			"\tres := server.Created(req)\n\tres = server.Accepted(req)\n\tres.Status = 201\n\treturn res, nil",
			0,
		},
		{
			"blank_identifier_binding",
			"\t_ = server.Created(req)\n\t_ = server.Accepted(req)\n\treturn server.Accepted(req), nil",
			202,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForHandlerBody(t, tt.body))
		})
	}
}

// TestExtractSuccessStatusIgnoresClosures verifies bindings and returns inside
// a nested func literal belong to the closure, not the handler.
func TestExtractSuccessStatusIgnoresClosures(t *testing.T) {
	body := "\tgo func() {\n\t\tinner := server.Created(req)\n\t\t_ = inner\n\t}()\n\treturn res, nil"
	assert.Equal(t, 0, statusForHandlerBody(t, body),
		"a binding made inside a closure must not resolve the handler's returned identifier")
}

// TestExtractSuccessStatusHonoursImportAlias verifies the binding path uses the
// same alias resolution as the direct-return path.
func TestExtractSuccessStatusHonoursImportAlias(t *testing.T) {
	src := `package mod

import srv "github.com/gaborage/go-bricks/server"

func handle(req Req, ctx srv.HandlerContext) (srv.Result[Resp], srv.IAPIError) {
	res := srv.Accepted(req)
	return res, nil
}
`
	aliases := map[string]struct{}{"srv": {}}
	assert.Equal(t, 202, statusForSource(t, src, aliases), "aliased server import resolves through the binding")
	assert.Equal(t, 0, statusForSource(t, src, map[string]struct{}{"other": {}}),
		"an unrelated alias set must not match the srv qualifier")
}

// TestExtractSuccessStatusNoBody covers a declaration without a body (an
// assembly or external stub), which must not panic.
func TestExtractSuccessStatusNoBody(t *testing.T) {
	src := "package mod\n\nfunc handle(req Req) (Resp, error)\n"
	assert.Equal(t, 0, statusForSource(t, src, nil))
}
