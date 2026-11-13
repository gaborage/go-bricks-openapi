package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestRouteNodeKey pins the port of go-bricks v0.71.1's routeNodeKey: which
// method/path pairs echo treats as one route node.
func TestRouteNodeKey(t *testing.T) {
	tests := []struct {
		name  string
		paths []string // every path here must give one key
		want  string
	}{
		{"exact match", []string{"/dup", "/dup"}, "GET /dup"},
		{"param-name variant", []string{"/u/:id", "/u/:uid"}, "GET /u/:"},
		{"mid-segment param", []string{"/m/a:x", "/m/a:y"}, "GET /m/a:"},
		{"param name runs to the next slash", []string{"/o/:a/x", "/o/:b/x"}, "GET /o/:/x"},
		{"escaped colon stays literal", []string{`/e/a\:x`}, `GET /e/a\:x`},
		{"wildcard truncation", []string{"/files/*", "/files/*rest", "/files/*/x"}, "GET /files/*"},
		{"a star inside a param name does not truncate", []string{"/a/:x*y", "/a/:z"}, "GET /a/:"},
		{"missing leading slash", []string{"twin", "/twin"}, "GET /twin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, p := range tt.paths {
				assert.Equal(t, tt.want, routeNodeKey("GET", p), "path %q", p)
			}
		})
	}
	assert.NotEqual(t, routeNodeKey("GET", "/dup"), routeNodeKey("POST", "/dup"), "the method is part of the key")
	assert.NotEqual(t, routeNodeKey("GET", `/e/a\:x`), routeNodeKey("GET", "/e/a:x"), "an escaped colon is not a parameter")
}

// TestRouteNodesClaim pins the seen-set: a key is claimed once, and a nil set
// (a direct body walk with no module) claims everything and records nothing.
func TestRouteNodesClaim(t *testing.T) {
	s := routeNodes{}
	assert.True(t, s.claim("GET /x"))
	assert.False(t, s.claim("GET /x"))
	assert.True(t, s.claim("POST /x"))

	var none routeNodes
	assert.True(t, none.claim("GET /x"))
	assert.True(t, none.claim("GET /x"))
}

// TestKeepFirstRoutesIgnoresUnkeyedRoutes pins that a route with no node key
// is never compared, so two of them both survive.
func TestKeepFirstRoutesIgnoresUnkeyedRoutes(t *testing.T) {
	a := New(t.TempDir())
	routes := []models.Route{{Method: "GET", Path: "/x"}, {Method: "GET", Path: "/x"}}
	got := a.keepFirstRoutes(routes, map[string]*models.Route{})
	assert.Len(t, got, 2)
	assert.Empty(t, a.Warnings(t.Context()))
}

// duplicateRoutesFixture is the golden fixture covering every duplicate row.
var duplicateRoutesFixture = filepath.Join("..", "spectest", "testdata", "duplicate_routes")

// TestDuplicateRoutesFixture pins the fixture's warnings (one per losing
// registration, naming both sites), the surviving routes, and that a loser's
// payload is registered only when another module registered it.
func TestDuplicateRoutesFixture(t *testing.T) {
	a := New(duplicateRoutesFixture)
	project, err := a.AnalyzeProject()
	require.NoError(t, err)

	warnings := a.Warnings(t.Context())
	want := []string{
		"route GET /dup at module.go:74:2 is the same route node as GET /dup at module.go:73:2",
		"route GET /u/:uid at module.go:76:2 is the same route node as GET /u/:id at module.go:75:2",
		"route GET /feat at module.go:80:3 is the same route node as GET /feat at module.go:78:3",
		"route GET /files/*rest at module.go:83:2 is the same route node as GET /files/* at module.go:82:2",
		"route GET /share at module.go:86:2 is the same route node as GET /share at module.go:85:2",
		"route POST /make at module.go:88:2 is the same route node as POST /make at module.go:87:2",
		"route GET /dup at " + filepath.Join("other", "module.go") + ":25:2 is the same route node as GET /dup at module.go:73:2",
	}
	require.Len(t, warnings, len(want), "%v", warnings)
	for i, w := range warnings {
		assert.True(t, strings.HasPrefix(w, want[i]), "warning %d: %s", i, w)
		assert.Contains(t, w, "documenting the first; the second is dropped from the spec")
		assert.NotContains(t, w, "start", "the warning must not claim the service fails to start")
	}
	assert.Empty(t, a.UnresolvedRoutes(t.Context()), "a dropped duplicate is not an Unresolved route")

	var paths []string
	for _, m := range project.Modules {
		for _, r := range m.Routes {
			paths = append(paths, r.Method+" "+r.Path)
		}
	}
	assert.ElementsMatch(t, []string{
		"GET /dup", "GET /u/{id}", "GET /feat", "GET /files/*", "GET /keep", "GET /share", "POST /make",
	}, paths)

	names := map[string]bool{}
	for _, ti := range project.Types {
		names[ti.Name] = true
	}
	for _, gone := range []string{"B", "Office", "GetU2", "V2", "CreateTwo"} {
		assert.False(t, names[gone], "%s is registered only by a losing route in its own module", gone)
	}
	for _, kept := range []string{"A", "GetU", "V1", "Shared", "CreateOne", "Epsilon"} {
		assert.True(t, names[kept], "%s must stay registered", kept)
	}
}

// dupModuleHeader is a module's declarations ahead of its RegisterRoutes.
const dupModuleHeader = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{ h *Handler }

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type First struct {
	F string ` + "`json:\"f\"`" + `
}

type Second struct {
	S string ` + "`json:\"s\"`" + `
}

type Third struct {
	T string ` + "`json:\"t\"`" + `
}

type Handler struct{}

func (m *Module) first(ctx server.HandlerContext) (server.Result[First], server.IAPIError) {
	return server.OK(First{}), nil
}

func (m *Module) second(ctx server.HandlerContext) (server.Result[Second], server.IAPIError) {
	return server.OK(Second{}), nil
}

func (h *Handler) third(ctx server.HandlerContext) (server.Result[Third], server.IAPIError) {
	return server.OK(Third{}), nil
}
`

// analyzeDupModule analyzes one module whose source is dupModuleHeader plus
// rest, returning the analyzer, its routes and its registered type names.
func analyzeDupModule(t *testing.T, rest string) (a *ProjectAnalyzer, routes []models.Route, names map[string]bool) {
	t.Helper()
	a, routes = analyzeProjectRoutes(t, map[string]string{
		"go.mod":    "module github.com/example/svc\n\ngo 1.25\n",
		"module.go": dupModuleHeader + rest,
	})
	names = map[string]bool{}
	for _, ti := range a.typeRegistry {
		names[ti.Name] = true
	}
	return a, routes, names
}

// TestDuplicateRouteAcrossDelegates pins that the module's seen-set reaches
// every walker: a loser in a same-receiver helper, a package-level helper or
// a handler-field delegate registers no payload, and is dropped with a
// warning.
func TestDuplicateRouteAcrossDelegates(t *testing.T) {
	tests := []struct {
		name string
		rest string
		gone string
	}{
		{"same-receiver helper", `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", m.first)
	m.more(hr, r)
}

func (m *Module) more(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", m.second)
}
`, "Second"},
		{"package-level helper", `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", m.first)
	more(hr, r)
}

func more(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", pkgSecond)
}

func pkgSecond(ctx server.HandlerContext) (server.Result[Second], server.IAPIError) {
	return server.OK(Second{}), nil
}
`, "Second"},
		{"handler-field delegate", `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", m.first)
	m.h.RegisterRoutes(hr, r)
}

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", h.third)
}
`, "Third"},
		{"a delegate's route is first", `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.h.RegisterRoutes(hr, r)
	server.GET(hr, r, "/x", m.second)
}

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", h.third)
}
`, "Second"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, routes, names := analyzeDupModule(t, tt.rest)
			require.Len(t, routes, 1)
			assert.False(t, names[tt.gone], "the losing route's payload %s must not be registered", tt.gone)
			warnings := a.Warnings(t.Context())
			require.Len(t, warnings, 1, "%v", warnings)
			assert.Contains(t, warnings[0], "route GET /x at module.go:")
			assert.Contains(t, warnings[0], "is the same route node as GET /x at module.go:")
		})
	}
}

// TestDuplicateRouteRawAddClaimsTheNode pins that a raw Add registration
// claims its node: a later typed registration of the same node registers no
// payload and is dropped, and a later Add is dropped too.
func TestDuplicateRouteRawAddClaimsTheNode(t *testing.T) {
	a, routes, names := analyzeDupModule(t, `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	r.Add("GET", "/raw/:id", m.first)
	server.GET(hr, r, "/raw/:key", m.second)
	r.Add("GET", "/raw/:other", m.first)
}
`)
	require.Len(t, routes, 1)
	assert.Equal(t, "/raw/{id}", routes[0].Path)
	assert.False(t, names["Second"])
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 2, "%v", warnings)
	assert.Contains(t, warnings[0], "route GET /raw/:key at module.go:")
	assert.Contains(t, warnings[1], "route GET /raw/:other at module.go:")
}

// TestDuplicateRouteKeysTheJoinedPath pins that the key is taken on the path
// go-bricks joins, not the strings as written: a group's "/" is the group
// itself, and a missing leading slash is added.
func TestDuplicateRouteKeysTheJoinedPath(t *testing.T) {
	a, routes, _ := analyzeDupModule(t, `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/api", m.first)
	api := r.Group("/api")
	server.GET(hr, api, "/", m.second)
	server.GET(hr, r, "/twin", m.first)
	server.GET(hr, r, "twin", m.second)
}
`)
	assert.Len(t, routes, 2)
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 2, "%v", warnings)
	assert.Contains(t, warnings[0], "route GET /api at module.go:")
	assert.Contains(t, warnings[1], "route GET /twin at module.go:")
}

// TestDuplicateRouteMidSegmentParam pins that a mid-segment parameter pair is
// one route node: the first is documented, templated, and the second dropped.
func TestDuplicateRouteMidSegmentParam(t *testing.T) {
	a, routes, _ := analyzeDupModule(t, `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/m/a:x", m.first)
	server.GET(hr, r, "/m/a:y", m.second)
}
`)
	require.Len(t, routes, 1)
	assert.Equal(t, "/m/a{x}", routes[0].Path)
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 1, "%v", warnings)
	assert.Contains(t, warnings[0], "route GET /m/a:y at module.go:")
	assert.Contains(t, warnings[0], "same route node as GET /m/a:x at module.go:")
}

// TestDuplicateRouteDistinctNodesUntouched pins that routes on different
// nodes (another method, an escaped colon) raise nothing.
func TestDuplicateRouteDistinctNodesUntouched(t *testing.T) {
	a, routes, _ := analyzeDupModule(t, `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", m.first)
	server.POST(hr, r, "/x", m.second)
	server.GET(hr, r, "/e/a:x", m.first)
	server.GET(hr, r, "/e/a\\:x", m.second)
}
`)
	assert.Len(t, routes, 4)
	assert.Empty(t, a.Warnings(t.Context()))
}

// lineOf returns the 1-based line of the first line of src containing needle.
func lineOf(t *testing.T, src, needle string) int {
	t.Helper()
	for i, l := range strings.Split(src, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	require.Failf(t, "needle not found", "%q", needle)
	return 0
}

// TestDuplicateRouteHelperCalledTwiceSites pins that a helper called twice
// gives two distinguishable sites: the registration inside the helper, then
// the outermost delegate call that reached it (also through a nested helper).
func TestDuplicateRouteHelperCalledTwiceSites(t *testing.T) {
	rest := `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.helper(hr, r) // call one
	m.outer(hr, r)  // call two
}

func (m *Module) outer(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.helper(hr, r)
}

func (m *Module) helper(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/h", m.first)
}
`
	a, routes, _ := analyzeDupModule(t, rest)
	require.Len(t, routes, 1)
	src := dupModuleHeader + rest
	reg := lineOf(t, src, `"/h"`)
	one := lineOf(t, src, "// call one")
	two := lineOf(t, src, "// call two")
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 1, "%v", warnings)
	want := fmt.Sprintf("route GET /h at module.go:%d:2 via module.go:%d:2 is the same route node as GET /h at module.go:%d:2 via module.go:%d:2",
		reg, two, reg, one)
	assert.Contains(t, warnings[0], want)
}

// TestDuplicateRouteBuildTaggedWinnerIsStable pins that two build-tagged
// RegisterRoutes files for one module choose the same winner on every run:
// the file first in sorted path order.
func TestDuplicateRouteBuildTaggedWinnerIsStable(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module github.com/example/svc\n\ngo 1.25\n",
		"module.go": `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) dev(ctx server.HandlerContext) (server.Result[string], server.IAPIError) {
	return server.OK(""), nil
}

func (m *Module) prod(ctx server.HandlerContext) (server.Result[string], server.IAPIError) {
	return server.OK(""), nil
}
`,
		"routes_dev.go": `//go:build dev

package svc

import "github.com/gaborage/go-bricks/server"

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", m.dev)
}
`,
		"routes_prod.go": `//go:build !dev

package svc

import "github.com/gaborage/go-bricks/server"

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/x", m.prod)
}
`,
	}
	for rel, src := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(src), 0o600))
	}
	for range 16 {
		a := New(dir)
		project, err := a.AnalyzeProject()
		require.NoError(t, err)
		require.Len(t, project.Modules, 1)
		require.Len(t, project.Modules[0].Routes, 1)
		assert.Equal(t, "dev", project.Modules[0].Routes[0].HandlerName)
		warnings := a.Warnings(t.Context())
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "route GET /x at routes_prod.go:8:2 is the same route node as GET /x at routes_dev.go:8:2")
	}
}
