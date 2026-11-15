package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractResponseHeadersErrorPathWrites pins that a write whose
// innermost return-terminated statement list ends in a non-nil-error return
// is dropped: go-bricks answers that path with the error, so the header
// never reaches the success response.
func TestExtractResponseHeadersErrorPathWrites(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"write_before_error_return", `res := server.Result[Resp]{}
	if req.ID == "" {
		res.Headers = http.Header{"Retry-After": {"30"}}
		return res, server.NewServiceUnavailableError("down")
	}
	return res, nil`, nil},
		{"set_and_map_write_before_error_return", `res := server.Result[Resp]{Headers: http.Header{}}
	h := http.Header{}
	res2 := server.Result[Resp]{Headers: h}
	if req.ID == "" {
		res.Headers.Set("Retry-After", "30")
		h.Set("X-Map", "1")
		var x = server.Result[Resp]{Headers: http.Header{"X-Var": {"1"}}}
		_ = x
		return res, server.NewServiceUnavailableError("down")
	}
	_ = res2
	return res, nil`, nil},
		{"case_clause_error_return", `res := server.Result[Resp]{Headers: http.Header{}}
	switch req.ID {
	case "":
		res.Headers.Set("Retry-After", "30")
		return res, server.NewServiceUnavailableError("down")
	}
	return res, nil`, nil},
		{"inner_nil_return_wins", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		if req.Name != "" {
			res.Headers.Set("Location", "/x")
			return res, nil
		}
		return res, server.NewNotFoundError("x")
	}
	return res, nil`, []string{"Location"}},
		{"branch_without_return_falls_through_to_nil", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		res.Headers.Set("Etag", "v")
	}
	return res, nil`, []string{"Etag"}},
		{"nested_block_falls_through_to_error", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		if req.Name != "" {
			res.Headers.Set("Retry-After", "30")
		}
		return res, server.NewServiceUnavailableError("down")
	}
	return res, nil`, nil},
		{"bare_return_block_is_not_an_error_return", `res := server.Created(req)
	if req.ID == "" {
		res.Headers.Set("Etag", "v")
		return res, nil
	}
	func() { return }()
	return res, nil`, []string{"Etag"}},
		{"nil_return_reachable_before_error_return", `res := server.Result[Resp]{Headers: http.Header{"Location": {"/x"}}}
	if req.ID != "" {
		return res, nil
	}
	return res, server.NewNotFoundError("x")`, []string{"Location"}},
		{"nested_nil_return_after_write", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		res.Headers.Set("Location", "/x")
		for range 1 {
			switch req.Name {
			case "a":
				return res, nil
			}
		}
		return res, server.NewNotFoundError("x")
	}
	return res, nil`, []string{"Location"}},
		{"nil_return_before_write_does_not_count", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		if req.Name == "a" {
			return res, nil
		}
		res.Headers.Set("Retry-After", "30")
		return res, server.NewServiceUnavailableError("down")
	}
	return res, nil`, nil},
		{"closure_nil_return_does_not_count", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		res.Headers.Set("Retry-After", "30")
		f := func() (server.Result[Resp], error) { return res, nil }
		_ = f
		return res, server.NewServiceUnavailableError("down")
	}
	return res, nil`, nil},
		{"sibling_case_return_does_not_rescue", `res := server.Result[Resp]{Headers: http.Header{}}
	switch req.ID {
	case "bad":
		res.Headers.Set("Retry-After", "30")
	case "ok":
		return res, nil
	}
	return res, server.NewNotFoundError("x")`, nil},
		{"sibling_select_clause_does_not_rescue", `res := server.Result[Resp]{Headers: http.Header{}}
	select {
	case <-ctx.Done():
		res.Headers.Set("Retry-After", "30")
	default:
		return res, nil
	}
	return res, server.NewNotFoundError("x")`, nil},
		{"other_if_arm_does_not_rescue", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		res.Headers.Set("Retry-After", "30")
	} else {
		return res, nil
	}
	return res, server.NewNotFoundError("x")`, nil},
		{"earlier_loop_return_does_not_rescue", `res := server.Result[Resp]{Headers: http.Header{}}
	for i := range 3 {
		if i == 1 {
			return res, nil
		}
		res.Headers.Set("Retry-After", "30")
	}
	return res, server.NewNotFoundError("x")`, nil},
		{"later_case_body_statement_rescues", `res := server.Result[Resp]{Headers: http.Header{}}
	switch req.ID {
	case "ok":
		res.Headers.Set("Location", "/x")
		return res, nil
	}
	return res, server.NewNotFoundError("x")`, []string{"Location"}},
		{"return_after_switch_rescues", `res := server.Result[Resp]{Headers: http.Header{}}
	switch req.ID {
	case "ok":
		res.Headers.Set("Location", "/x")
	}
	return res, nil`, []string{"Location"}},
		{"map_write_needs_reachable_holder_return", `h := http.Header{}
	res := server.Result[Resp]{Headers: h}
	if req.ID == "" {
		h.Set("Retry-After", "30")
		return res, server.NewNotFoundError("x")
	}
	h.Set("Location", "/x")
	return res, nil`, []string{"Location"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
}

// namedHead opens a handler with named results res and e.
const namedHead = `package mod

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

func handle(req Req) (res server.Result[Resp], e server.IAPIError) {
`

// TestExtractResponseHeadersBareReturnNamedError pins that a bare return
// counts the first named result only when the named error result is never
// assigned anything but the literal nil outside function literals.
func TestExtractResponseHeadersBareReturnNamedError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"error_assigned_non_nil", `res.Headers = http.Header{"X-Bare-Err": {"1"}}
	e = server.NewNotFoundError("x")
	return`, nil},
		{"error_assigned_in_branch", `res.Headers = http.Header{"Location": {"/x"}}
	if req.ID == "" {
		e = server.NewNotFoundError("x")
	}
	return`, nil},
		{"error_multi_assigned", `res.Headers = http.Header{"Location": {"/x"}}
	_, e = load()
	return`, nil},
		{"error_assigned_nil_only", `res.Headers = http.Header{"Location": {"/x"}}
	e = nil
	return`, []string{"Location"}},
		{"error_never_assigned", `res.Headers = http.Header{"Location": {"/x"}}
	return`, []string{"Location"}},
		{"shadowing_error_declared_in_block", `res.Headers = http.Header{"Location": {"/x"}}
	if req.ID == "" {
		e := server.NewNotFoundError("x")
		_ = e
	}
	return`, []string{"Location"}},
		{"error_assigned_in_closure_ignored", `res.Headers = http.Header{"Location": {"/x"}}
	f := func() { e = server.NewNotFoundError("x") }
	_ = f
	return`, []string{"Location"}},
		{"bare_return_of_named_does_not_rescue_other", `other := server.Created(req)
	if req.ID == "" {
		other.Headers = http.Header{"Location": {"/x"}}
		if req.Name == "a" {
			return
		}
		return other, server.NewNotFoundError("x")
	}
	return other, nil`, nil},
		{"serving_bare_return_rescues_named_write", `if req.ID == "" {
		res.Headers = http.Header{"Location": {"/x"}}
		if req.Name == "a" {
			return
		}
		return res, server.NewNotFoundError("x")
	}
	return res, nil`, []string{"Location"}},
		{"unrelated_nil_return_does_not_rescue", `other := server.Created(req)
	if req.ID == "" {
		other.Headers.Set("Retry-After", "30")
		if req.Name == "a" {
			return res, nil
		}
		return other, server.NewNotFoundError("x")
	}
	return other, nil`, nil},
		{"non_serving_bare_return_does_not_rescue", `other := server.Created(req)
	if req.ID == "" {
		other.Headers = http.Header{"Location": {"/x"}}
		if req.Name == "a" {
			return
		}
		return other, server.NewNotFoundError("x")
	}
	e = server.NewNotFoundError("y")
	return other, nil`, nil},
		{"explicit_nil_return_still_counts", `res.Headers = http.Header{"Location": {"/x"}}
	e = server.NewNotFoundError("x")
	return res, nil`, []string{"Location"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForSource(t, namedHead+tt.body+"\n}\n", nil))
		})
	}
	t.Run("blank_error_result", func(t *testing.T) {
		src := `package mod

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

func handle(req Req) (res server.Result[Resp], _ server.IAPIError) {
	res.Headers = http.Header{"Location": {"/x"}}
	return
}
`
		assert.Equal(t, []string{"Location"}, headersForSource(t, src, nil))
	})
}

// TestExtractResponseHeadersReassignedMap pins that a linked local map that
// is ever reassigned after its declaration contributes nothing: which map the
// result ends up holding depends on the order of the writes.
func TestExtractResponseHeadersReassignedMap(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"reassigned_literal", `h := http.Header{"X-First": {"1"}}
	res := server.Result[Resp]{Headers: h}
	h = http.Header{"X-Later": {"1"}}
	_ = h
	return res, nil`, nil},
		{"reassigned_make", `h := http.Header{}
	res := server.Result[Resp]{Headers: h}
	h = make(http.Header)
	h.Set("X-New-Map", "1")
	return res, nil`, nil},
		{"reassigned_in_multi_define", `h := http.Header{"Location": {"/x"}}
	h, n := http.Header{}, 1
	_ = n
	return server.Result[Resp]{Headers: h}, nil`, nil},
		{"field_reassigned_keeps_direct_writes", `res := server.Created(req)
	res.Headers = http.Header{"Location": {"/x"}}
	res = server.Accepted(req)
	return res, nil`, []string{"Location"}},
		{"parenthesized_var_decl_is_not_reassignment", `var (
		h = http.Header{"Location": {"/x"}}
		n = 1
	)
	_ = n
	return server.Result[Resp]{Headers: h}, nil`, []string{"Location"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
}
