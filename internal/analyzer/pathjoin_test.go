package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizeGroupPrefix pins the go-bricks pathutil.NormalizePrefix rules.
func TestNormalizeGroupPrefix(t *testing.T) {
	tests := map[string]string{
		"":      "",
		"/":     "",
		"/api":  "/api",
		"api":   "/api",
		"v2/":   "/v2",
		"/v3//": "/v3",
	}
	for in, want := range tests {
		assert.Equal(t, want, normalizeGroupPrefix(in), "normalizeGroupPrefix(%q)", in)
	}
}

// TestJoinRoutePath pins the go-bricks RouteRegistrar.FullPath join, with the
// expected paths taken from issue 197's acceptance table.
func TestJoinRoutePath(t *testing.T) {
	tests := []struct {
		prefix, path, want string
	}{
		{"/api", "/", "/api"},
		{"/api", "", "/api"},
		{"/api", "/api", "/api"},
		{"/api", "grp", "/api/grp"},
		{"/api", "/apix", "/api/apix"},
		{"/api", "//x", "/api//x"},
		{"/api", "/api/x", "/api/x"},
		{"/api", "/api/", "/api/"},
		{"/api/v3", "/w", "/api/v3/w"},
		{"/api/v1", "/api/v1/s", "/api/v1/s"},
		{"", "nolead", "/nolead"},
		{"", "/", "/"},
		{"", "", "/"},
		{"", "/users/:id", "/users/:id"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, joinRoutePath(tt.prefix, tt.path), "joinRoutePath(%q, %q)", tt.prefix, tt.path)
	}
}

// TestGroupChainJoin checks the acceptance chain Group("/api").Group("v3/")
// with "/w" end to end through the composed helpers.
func TestGroupChainJoin(t *testing.T) {
	prefix := normalizeGroupPrefix("/api") + normalizeGroupPrefix("v3/")
	assert.Equal(t, "/api/v3/w", joinRoutePath(prefix, "/w"))
}
