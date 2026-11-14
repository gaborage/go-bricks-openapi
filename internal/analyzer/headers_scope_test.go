package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractResponseHeadersNilErrorReturnsOnly pins that a variable counts
// as the result only when some return gives it with the bare nil error:
// go-bricks drops the result whenever the error is non-nil.
func TestExtractResponseHeadersNilErrorReturnsOnly(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"error_only_ident", `if req.ID == "" {
		bad := server.Result[Resp]{}
		bad.Headers = http.Header{"X-Error-Only": {"1"}}
		return bad, server.NewNotFoundError("item")
	}
	return server.Result[Resp]{}, nil`, nil},
		{"error_only_bound_literal", `if req.ID == "" {
		res := server.Result[Resp]{Headers: http.Header{"X-Err": {"1"}}}
		return res, server.NewInternalServerError("x")
	}
	res := server.Result[Resp]{}
	return res, nil`, nil},
		{"error_variable", `res := server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	var apiErr server.IAPIError
	return res, apiErr`, nil},
		{"also_returned_with_nil", `res := server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	if req.ID == "" {
		return res, server.NewNotFoundError("item")
	}
	return res, nil`, []string{"Location"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
}

// TestExtractResponseHeadersMatchByDeclaration pins that writes are matched
// to the returned variable by declaration, not by name: an inner-block
// redeclaration, a range variable, or a shadowed named result is another
// variable.
func TestExtractResponseHeadersMatchByDeclaration(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"inner_block_shadow", `res := server.Result[Resp]{}
	if req.ID != "" {
		res := server.Result[Resp]{}
		res.Headers = http.Header{"X-Shadowed": {"1"}}
		_ = res
	}
	return res, nil`, nil},
		{"range_variable", `res := server.Result[Resp]{}
	for _, res := range []server.Result[Resp]{{}} {
		res.Headers = http.Header{"X-Range": {"1"}}
	}
	return res, nil`, nil},
		{"if_init_shadow", `res := server.Result[Resp]{}
	if res := server.Created(req); req.ID != "" {
		res.Headers.Set("X-Init", "1")
	}
	return res, nil`, nil},
		{"outer_write_inside_branch", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID != "" {
		res.Headers.Set("Location", "/x")
	}
	return res, nil`, []string{"Location"}},
		{"sibling_blocks_each_returned", `if req.ID == "" {
		res := server.Created(req)
		res.Headers = http.Header{"Location": {"/x"}}
		return res, nil
	}
	res := server.Accepted(req)
	res.Headers = http.Header{"Retry-After": {"1"}}
	return res, nil`, []string{"Location", "Retry-After"}},
		{"shadowed_return_only", `res := server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	if req.ID == "" {
		res := server.Accepted(req)
		return res, nil
	}
	return server.Created(req), nil`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
	t.Run("named_result_shadowed_in_loop", func(t *testing.T) {
		got := headersForSource(t, `package mod

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

func handle(req Req) (res server.Result[Resp], apiErr server.IAPIError) {
	for i := 0; i < 1; i++ {
		res := server.Result[Resp]{}
		res.Headers = http.Header{"X-Loop": {"1"}}
		_ = res
	}
	return
}
`, nil)
		assert.Nil(t, got)
	})
}

// TestExtractResponseHeadersLocalMap covers a header map built in a local
// variable and given to Headers: the names written to that variable's
// declaration count, before or after the link, and only when it is linked.
func TestExtractResponseHeadersLocalMap(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"set_then_returned_literal", `h := http.Header{}
	h.Set("Location", "/x")
	return server.Result[Resp]{Data: Resp{}, Headers: h}, nil`, []string{"Location"}},
		{"literal_add_index_then_field", `h := http.Header{"Etag": {"v"}}
	h.Add("Link", "<x>")
	h["X-Index"] = []string{"1"}
	res := server.Created(req)
	res.Headers = h
	return res, nil`, []string{"Etag", "Link", "X-Index"}},
		{"var_map_bound_literal", `var h = http.Header{"Location": {"/x"}}
	res := server.Result[Resp]{Headers: h}
	return res, nil`, []string{"Location"}},
		{"written_after_link", `h := make(http.Header)
	res := server.Result[Resp]{Headers: h}
	h.Set("Location", "/x")
	return res, nil`, []string{"Location"}},
		{"not_linked", `h := http.Header{"Location": {"/x"}}
	h.Set("Etag", "v")
	_ = h
	return server.Created(req), nil`, nil},
		{"linked_on_error_path_only", `h := http.Header{"Location": {"/x"}}
	if req.ID == "" {
		return server.Result[Resp]{Headers: h}, server.NewNotFoundError("x")
	}
	return server.Created(req), nil`, nil},
		{"shadowed_map_declaration", `h := http.Header{}
	if req.ID != "" {
		h := http.Header{}
		h.Set("X-Inner", "1")
		_ = h
	}
	return server.Result[Resp]{Headers: h}, nil`, nil},
		{"package_level_map_ignored", `hdrs.Set("Location", "/x")
	return server.Result[Resp]{Headers: hdrs}, nil`, nil},
		{"alias_of_field_not_followed", `res := server.Result[Resp]{Headers: http.Header{}}
	h := res.Headers
	h.Set("Location", "/x")
	return res, nil`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
}

// TestExtractResponseHeadersFieldNameValidity pins that only RFC 9110 token
// names are documented.
func TestExtractResponseHeadersFieldNameValidity(t *testing.T) {
	got := headersForBody(t, `res := server.Created(req)
	res.Headers = http.Header{"X Bad:Name": {"1"}, "x-request-id": {"1"}, "Über": {"1"}, "X-Ok_1.~!": {"1"}}
	return res, nil`)
	assert.Equal(t, []string{"X-Ok_1.~!", "X-Request-Id"}, got)
	for _, name := range []string{"Location", "X-A", "a1!#$%&'*+-.^_`|~"} {
		assert.True(t, validHeaderName(name), name)
	}
	for _, name := range []string{"", "X Bad", "X:Y", "Ü", "a\tb", "(x)"} {
		assert.False(t, validHeaderName(name), name)
	}
}

// TestExtractResponseHeadersKnownLimitations pins the documented limits: a
// later reset or Del of a header is not modeled, so the name stays.
func TestExtractResponseHeadersKnownLimitations(t *testing.T) {
	assert.Equal(t, []string{"Location"}, headersForBody(t, `res := server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	res.Headers = nil
	return res, nil`))
	assert.Equal(t, []string{"Location"}, headersForBody(t, `res := server.Created(req)
	res.Headers = http.Header{}
	res.Headers.Set("Location", "/a")
	res.Headers.Del("Location")
	return res, nil`))
}
