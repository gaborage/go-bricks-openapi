package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractResponseHeadersInitAndHolderPaths pins two reachability rules:
// a write in an if/switch/for initializer runs before every arm, so a serving
// return in any arm can follow it; and a write to a local map counts only for
// a serving return that both the map write and the link giving the map to the
// returned result can reach.
func TestExtractResponseHeadersInitAndHolderPaths(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"if_init_write_reaches_body_return", `res := server.Result[Resp]{}
	if res.Headers = (http.Header{"Location": {"/x"}}); req.ID == "" {
		return res, nil
	}
	return server.Result[Resp]{}, nil`, []string{"Location"}},
		{"if_init_write_reaches_else_return", `res := server.Result[Resp]{}
	if res.Headers = (http.Header{"Location": {"/x"}}); req.ID == "" {
		return server.Result[Resp]{}, nil
	} else {
		return res, nil
	}`, []string{"Location"}},
		{"switch_init_write_reaches_any_clause", `res := server.Result[Resp]{}
	switch res.Headers = (http.Header{"Location": {"/x"}}); req.ID {
	case "a":
		return server.Result[Resp]{}, nil
	case "b":
		return res, nil
	}
	return server.Result[Resp]{}, nil`, []string{"Location"}},
		{"type_switch_init_write", `res := server.Result[Resp]{}
	var x any = req
	switch res.Headers = (http.Header{"Location": {"/x"}}); x.(type) {
	case string:
		return res, nil
	}
	return server.Result[Resp]{}, nil`, []string{"Location"}},
		{"for_init_write_reaches_body_return", `res := server.Result[Resp]{}
	for res.Headers = (http.Header{"Location": {"/x"}}); req.ID == ""; {
		return res, nil
	}
	return server.Result[Resp]{}, nil`, []string{"Location"}},
		{"init_write_continues_climbing", `res := server.Result[Resp]{}
	if res.Headers = (http.Header{"Location": {"/x"}}); req.ID == "" {
		_ = req
	}
	return res, nil`, []string{"Location"}},
		{"arm_write_keeps_sibling_exclusion", `res := server.Result[Resp]{Headers: http.Header{}}
	if req.ID == "" {
		res.Headers.Set("Retry-After", "30")
	} else if req.Name == "" {
		return res, nil
	}
	return res, server.NewNotFoundError("x")`, nil},
		{"arm_write_under_init_keeps_sibling_exclusion", `res := server.Result[Resp]{Headers: http.Header{}}
	if n := len(req.ID); n == 0 {
		res.Headers.Set("Retry-After", "30")
	} else {
		return res, nil
	}
	return res, server.NewNotFoundError("x")`, nil},
		{"map_link_and_write_on_different_paths", `h := http.Header{}
	res := server.Result[Resp]{}
	if req.ID == "" {
		res.Headers = h
		return res, nil
	}
	h.Set("X-Map", "1")
	return res, nil`, nil},
		{"map_link_after_write_same_path", `h := http.Header{}
	h.Set("Location", "/x")
	res := server.Result[Resp]{}
	res.Headers = h
	return res, nil`, []string{"Location"}},
		{"map_write_after_link_same_path", `h := http.Header{}
	res := server.Result[Resp]{}
	res.Headers = h
	h.Set("Location", "/x")
	return res, nil`, []string{"Location"}},
		{"map_in_returned_literal_only_on_its_path", `h := http.Header{}
	if req.ID == "" {
		return server.Result[Resp]{Headers: h}, nil
	}
	h.Set("X-Map", "1")
	return server.Result[Resp]{}, nil`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, headersForBody(t, tt.body))
		})
	}
}
