package analyzer

import (
	"go/types"
	"strings"
	"testing"

	"github.com/gaborage/go-bricks-openapi/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Issue 143: a handler returning *server.Result[T] or
// *server.ResultWithMeta[T] is documented like its value form. go-bricks
// serves both alike: the wrappers' provider methods have value receivers, so
// the pointer satisfies the interfaces its response writer type-asserts.

// TestPayloadExprPointerWrapper pins that payloadExpr sheds exactly one
// written pointer to a server result wrapper, honouring the server import's
// alias, and leaves every other pointer as written.
func TestPayloadExprPointerWrapper(t *testing.T) {
	a := New("")
	srv := map[string]struct{}{"srv": {}}
	tests := []struct {
		src     string
		aliases map[string]struct{}
		want    string
	}{
		{"server.Result[Item]", nil, "Item"},
		{"*server.Result[Item]", nil, "Item"},
		{"*server.ResultWithMeta[Item]", nil, "Item"},
		{"*server.Result[*Item]", nil, "*Item"},
		{"*(server.Result[Item])", nil, "Item"},
		{"(*server.Result[Item])", nil, "Item"},
		{"*srv.Result[Item]", srv, "Item"},
		{"*srv.Result[Item]", nil, "*srv.Result[Item]"},
		{"**server.Result[Item]", nil, "**server.Result[Item]"},
		{"*Envelope[Item]", nil, "*Envelope[Item]"},
		{"*pkg.Result[Item]", nil, "*pkg.Result[Item]"},
		{"*Item", nil, "*Item"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			assert.Equal(t, tt.want, types.ExprString(a.payloadExpr(mustParse(t, tt.src), tt.aliases)))
		})
	}
}

// TestExtractSuccessStatusAddressOf pins the success status of a pointer
// handler's return: &server.Result[T]{...} reads the literal under the
// nil-error gate, &res reads res's binding with no gate, and a binding
// res := &server.Result[T]{...} reads its literal. Every other address-of or
// unary operand stays 0.
func TestExtractSuccessStatusAddressOf(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"literal_nil", "\treturn &server.Result[Resp]{Status: http.StatusCreated, Data: req}, nil", 201},
		{"literal_no_status", "\treturn &server.ResultWithMeta[Resp]{Data: req}, nil", 200},
		{"literal_parenthesized", "\treturn &(server.Result[Resp]{Status: http.StatusCreated}), nil", 201},
		{
			"literal_with_error_after_created",
			"\tif req.Name != \"\" {\n\t\treturn server.Created(req), nil\n\t}\n\treturn &server.Result[Resp]{Status: http.StatusAccepted}, apiErr",
			201,
		},
		{"bound_identifier", "\tres := server.Created(req)\n\treturn &res, nil", 201},
		{
			"bound_identifier_after_conflict",
			"\tif req.Name == \"\" {\n\t\treturn nil, server.NewConflictError(\"dup\")\n\t}\n\tres := server.Created(req)\n\treturn &res, nil",
			201,
		},
		{"unbound_identifier", "\treturn &res, nil", 0},
		{"binding_to_literal", "\tres := &server.Result[Resp]{Status: http.StatusAccepted, Data: req}\n\treturn res, nil", 202},
		{"binding_to_literal_status_write", "\tres := &server.Result[Resp]{Data: req}\n\tres.Status = http.StatusCreated\n\treturn res, nil", 201},
		{"binding_to_address_of_non_literal", "\tres := &other\n\treturn res, nil", 0},
		{"binding_to_non_address_unary", "\tres := -other\n\treturn res, nil", 0},
		{"nil_result", "\treturn nil, apiErr", 0},
		{"non_address_unary", "\treturn <-ch, nil", 0},
		{"address_of_field", "\treturn &x.f, nil", 0},
		{"non_server_literal", "\treturn &Envelope[Resp]{Status: http.StatusCreated}, nil", 0},
		// go-bricks serves *NoContentResult as NoContentResult: a 204.
		{"no_content_literal", "\treturn &server.NoContentResult{}, nil", 204},
		// &res of a pointer binding is a **server.Result[T]: no provider
		// methods, so go-bricks serves the default status, never the wrapper's.
		{"address_of_pointer_binding", "\tres := &server.Result[Resp]{Status: http.StatusNoContent}\n\treturn &res, nil", 0},
		{"address_of_parenthesized_pointer_binding", "\tres := (&server.Result[Resp]{Status: http.StatusCreated})\n\treturn &res, nil", 0},
		{"address_of_pointer_var_decl", "\tvar res = &server.Result[Resp]{Status: http.StatusCreated}\n\treturn &res, nil", 0},
		{"address_of_address_binding", "\tres := server.Result[Resp]{Status: http.StatusCreated}\n\tp := &res\n\treturn &p, nil", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, statusForHandlerBody(t, tt.body))
		})
	}
}

// TestDoublePointerResultWrapperWarns pins that **server.Result[T], which
// go-bricks encodes as the Result struct itself under data, is not unwrapped:
// it stays untyped and raises exactly the generic payload warning, so
// --strict fails on it.
func TestDoublePointerResultWrapperWarns(t *testing.T) {
	route, warnings := analyzeParen(t, "ctx server.HandlerContext", "**server.Result[Named]")
	want := "response type **server.Result[Named]: server.Result[Named] is a generic type instantiation, which is not resolved — emitting an untyped schema"
	assert.Equal(t, []string{want}, warnings)
	assert.Nil(t, route.Response)
}

// TestDoublePointerResultWrapperStatus pins that a **server.Result[T]
// handler returning &res, res bound to &server.Result[T]{Status: ...}, keeps
// the default status: go-bricks' type assertions against its result
// providers fail on the double pointer, so it serves the default writer's
// 200, never the literal's 204.
func TestDoublePointerResultWrapperStatus(t *testing.T) {
	src := strings.NewReplacer("PARAMS", "ctx server.HandlerContext", "RESULT", "**server.Result[Named]",
		"return out, nil", "res := &server.Result[Named]{Status: 204}\n\treturn &res, nil").Replace(parenModSrc)
	_, routes := analyzeSingleModule(t, src)
	route := routeForPath(t, routes, "POST /items")
	assert.Equal(t, 0, route.SuccessStatus)
	assert.Nil(t, route.Response)
}

// TestPointerResultWrapperJOSEParity pins that a jose-tagged payload is
// documented the same through *server.Result[J] as through server.Result[J]:
// the pointer form takes the value form's path unchanged.
func TestPointerResultWrapperJOSEParity(t *testing.T) {
	jose := "\ntype J struct {\n\t_   struct{} `jose:\"encrypt=k3,sign=k4\"`\n\tPAN string `json:\"pan\"`\n}\n"
	analyze := func(result string) (resp *models.TypeInfo, status int) {
		src := strings.NewReplacer("PARAMS", "req CreateReq, ctx server.HandlerContext", "RESULT", result).Replace(parenModSrc + jose)
		_, routes := analyzeSingleModule(t, src)
		route := routeForPath(t, routes, "POST /items")
		require.NotNil(t, route.Response, result)
		return route.Response, route.SuccessStatus
	}
	valueResp, valueStatus := analyze("server.Result[J]")
	ptrResp, ptrStatus := analyze("*server.Result[J]")
	assert.Equal(t, valueResp, ptrResp)
	assert.Equal(t, valueStatus, ptrStatus)
}

// TestPointerNoContentResultStatus pins the one change issue 143 makes to a
// *server.NoContentResult handler, whose payload stays untyped: a returned
// &server.NoContentResult{} or &res bound to server.NoContent() now resolves
// to 204, as go-bricks serves it (NoContentResult's ResultMeta has a value
// receiver), documented bodiless with no payload warning. Any other return
// shape keeps the default 200 and the payload warning.
func TestPointerNoContentResultStatus(t *testing.T) {
	const unresolved = "response type server.NoContentResult resolves to no schema component — emitting an untyped schema (declare it as a struct in the project for a typed spec)"
	tests := []struct {
		name     string
		ret      string
		status   int
		warnings []string
	}{
		{"literal", "return &server.NoContentResult{}, nil", 204, nil},
		{"bound", "res := server.NoContent()\n\treturn &res, nil", 204, nil},
		{"other", "return out, nil", 0, []string{unresolved}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := strings.NewReplacer("PARAMS", "req CreateReq, ctx server.HandlerContext", "RESULT", "*server.NoContentResult",
				"return out, nil", tt.ret).Replace(parenModSrc)
			a, routes := analyzeSingleModule(t, src)
			route := routeForPath(t, routes, "POST /items")
			assert.Equal(t, tt.status, route.SuccessStatus)
			assert.Equal(t, tt.warnings, a.Warnings(t.Context()))
			require.NotNil(t, route.Response)
			assert.Equal(t, tt.status == 204, route.Response.NoContent, "a 204 is documented bodiless")
		})
	}
}
