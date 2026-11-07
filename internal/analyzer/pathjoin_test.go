package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizeGroupPrefix pins the go-bricks pathutil.NormalizePrefix rules,
// including every case of go-bricks v0.71.1's own TestNormalizePrefix.
func TestNormalizeGroupPrefix(t *testing.T) {
	tests := map[string]string{
		"":         "",
		"/":        "",
		"/api":     "/api",
		"api":      "/api",
		"v2/":      "/v2",
		"/v3//":    "/v3",
		"foo":      "/foo",
		"/foo":     "/foo",
		"/foo/":    "/foo",
		"foo/":     "/foo",
		"foo/bar/": "/foo/bar",
		"/foo///":  "/foo",
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
		{"/api", "/t/", "/api/t/"},
		{"/api", "/:id", "/api/:id"},
		{"/api/v3", "/w", "/api/v3/w"},
		{"/api/v1", "/api/v1/s", "/api/v1/s"},
		// Only the group's WHOLE prefix is stripped, never an ancestor's.
		{"/api/v1", "/api/x", "/api/v1/api/x"},
		{"", "nolead", "/nolead"},
		{"", "/", "/"},
		{"", "", "/"},
		{"", "/users/:id", "/users/:id"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, joinRoutePath(tt.prefix, tt.path), "joinRoutePath(%q, %q)", tt.prefix, tt.path)
	}
}

// TestJoinRoutePathPathutilCases replays go-bricks v0.71.1's
// TestEnsureLeadingSlash (on a prefix-less group) and TestStripPathPrefix (on
// a "/foo" group) through joinRoutePath: a stripped remainder rejoins the
// prefix, an unstripped path is appended to it.
func TestJoinRoutePathPathutilCases(t *testing.T) {
	tests := []struct {
		name, prefix, path, want string
	}{
		{"empty_becomes_root", "", "", "/"},
		{"missing_slash_added", "", "foo", "/foo"},
		{"existing_slash_preserved", "", "/foo", "/foo"},
		{"root_preserved", "", "/", "/"},
		{"nested_path_preserved", "", "/foo/bar", "/foo/bar"},
		{"trailing_slash_preserved", "", "/foo/", "/foo/"},
		{"exact_match", "/foo", "/foo", "/foo"},
		{"child_path", "/foo", "/foo/bar", "/foo/bar"},
		{"deep_child", "/foo", "/foo/bar/baz", "/foo/bar/baz"},
		{"non_matching", "/foo", "/other", "/foo/other"},
		{"partial_word_match_rejected", "/foo", "/foobar", "/foo/foobar"},
		{"partial_word_match_with_slash_rejected", "/foo", "/foobar/baz", "/foo/foobar/baz"},
		{"trailing_slash_path_under_prefix", "/foo", "/foo/", "/foo/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, joinRoutePath(tt.prefix, tt.path))
		})
	}
}

// TestGroupChainJoin checks group chains end to end through the composed
// helpers, as groupBinding accumulates them.
func TestGroupChainJoin(t *testing.T) {
	tests := []struct {
		groups     []string
		path, want string
	}{
		{[]string{"/api", "v3/"}, "/w", "/api/v3/w"},
		{[]string{"v2/"}, "/y", "/v2/y"},
		{[]string{"/"}, "/sg", "/sg"},
	}
	for _, tt := range tests {
		prefix := ""
		for _, g := range tt.groups {
			prefix += normalizeGroupPrefix(g)
		}
		assert.Equal(t, tt.want, joinRoutePath(prefix, tt.path), "groups %q + %q", tt.groups, tt.path)
	}
}
