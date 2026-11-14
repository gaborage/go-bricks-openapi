package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestBuildResponsesHeaders pins issue 71 in the generator: a route's
// response header names are documented on its success response, and only
// there, in every success flavor — envelope, raw, JOSE and a bodiless 204 —
// each as a bare string-typed Header Object.
func TestBuildResponsesHeaders(t *testing.T) {
	headers := []string{"Location", "Retry-After"}
	want := map[string]*OpenAPIHeader{
		"Location":    {Schema: &OpenAPIProperty{Type: typeString}},
		"Retry-After": {Schema: &OpenAPIProperty{Type: typeString}},
	}
	tests := []struct {
		name    string
		route   models.Route
		success string
	}{
		{"envelope", models.Route{Response: &models.TypeInfo{Name: "Item"}, SuccessStatus: 201}, "201"},
		{"raw", models.Route{Response: &models.TypeInfo{Name: "Item"}, RawResponse: true, SuccessStatus: 202}, "202"},
		{"jose", models.Route{Response: &models.TypeInfo{Name: "Sealed", JOSE: true}}, "200"},
		{"no_content", models.Route{Response: &models.TypeInfo{NoContent: true}, SuccessStatus: 204}, "204"},
	}
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := tt.route
			route.Method, route.Path, route.ResponseHeaders = "POST", "/a", headers
			route.ErrorStatuses = []int{404}
			responses := gen.buildResponses(&route)
			require.Contains(t, responses, tt.success)
			assert.Equal(t, want, responses[tt.success].Headers)
			for code, resp := range responses {
				if code != tt.success {
					assert.Nil(t, resp.Headers, "error response %s must carry no headers", code)
				}
			}
		})
	}
}

// TestBuildResponsesNoHeaders verifies a route without header names emits no
// headers key at all.
func TestBuildResponsesNoHeaders(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	route := &models.Route{Method: "GET", Path: "/a", Response: &models.TypeInfo{Name: "Item"}}
	assert.Nil(t, gen.buildResponses(route)["200"].Headers)
	route.ResponseHeaders = []string{}
	assert.Nil(t, gen.buildResponses(route)["200"].Headers)
}
