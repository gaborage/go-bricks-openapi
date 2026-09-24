package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// pathDependentReasonTail is the part of a path-dependent Unresolved route's
// reason that names the cause, shared by every registrar it can mention.
const pathDependentReasonTail = "depends on control flow: it, or a registrar it derives from, is reassigned inside a nested block, loop or closure, or after a closure that reads it"

// untracedReasonTail is the cause part of an untraced Unresolved route's
// reason.
const untracedReasonTail = "cannot be traced: it, or a registrar it derives from, holds a value that is not a Group(...) call or another known registrar"

// pd returns, in order, the Reason of a path-dependent Unresolved route
// registered on each named registrar.
func pd(recvs ...string) []string {
	return prefixReasons(pathDependentReasonTail, recvs)
}

// ut is pd's sibling for untraced registrars.
func ut(recvs ...string) []string {
	return prefixReasons(untracedReasonTail, recvs)
}

func prefixReasons(tail string, recvs []string) []string {
	reasons := make([]string, 0, len(recvs))
	for _, recv := range recvs {
		reasons = append(reasons, `group prefix on registrar "`+recv+`" `+tail)
	}
	return reasons
}

// orderedRoutePaths returns the emitted routes' paths in walk (source) order.
func orderedRoutePaths(routes []models.Route) []string {
	paths := make([]string, 0, len(routes))
	for i := range routes {
		paths = append(paths, routes[i].Path)
	}
	return paths
}

// TestRegistrarPrefixIsPositional pins that a registration sees the group
// registrar binding in effect at its own position, honoring lexical scope: a
// same-block `=` reassignment is resolved exactly, an inner `:=` shadows only
// inside its block, and a reassignment from a nested block, loop body or
// closure makes the prefix path-dependent once the statement is left (a
// sibling branch still sees the earlier binding) — those routes are Unresolved
// routes, never emitted under a guessed prefix. So are routes on a registrar
// holding a value that cannot be traced to a Group(...) call, including a
// parameter or local shadowing a group registrar.
func TestRegistrarPrefixIsPositional(t *testing.T) {
	tests := []struct {
		name string
		body string
		// wantPaths are the emitted routes, in source order.
		wantPaths []string
		// wantUnresolved are the Reasons of the Unresolved routes, in source
		// order.
		wantUnresolved []string
	}{
		{
			name: "straight-line reassignment in the same block",
			body: `api := r.Group("/a")
	server.GET(hr, api, "/1", m.h)
	api = r.Group("/b")
	server.GET(hr, api, "/2", m.h)`,
			wantPaths: []string{"/a/1", "/b/2"},
		},
		{
			name: "reassignment inside an if block is path-dependent after it",
			body: `g := r.Group("/c")
	server.GET(hr, g, "/4", m.h)
	if enabled {
		g = r.Group("/d")
		server.GET(hr, g, "/5", m.h)
	}
	server.GET(hr, g, "/6", m.h)`,
			wantPaths:      []string{"/c/4", "/d/5"},
			wantUnresolved: pd("g"),
		},
		{
			name: "inner short declaration shadows only inside its block",
			body: `s := r.Group("/e")
	if enabled {
		s := r.Group("/f")
		server.GET(hr, s, "/7", m.h)
	}
	server.GET(hr, s, "/8", m.h)`,
			wantPaths: []string{"/f/7", "/e/8"},
		},
		{
			name: "reassignment inside a for body poisons the whole body and after",
			body: `g := r.Group("/c")
	server.GET(hr, g, "/before", m.h)
	for i := 0; i < 2; i++ {
		server.GET(hr, g, "/top", m.h)
		g = r.Group("/d")
		server.GET(hr, g, "/bottom", m.h)
	}
	server.GET(hr, g, "/after", m.h)`,
			wantPaths:      []string{"/c/before"},
			wantUnresolved: pd("g", "g", "g"),
		},
		{
			name: "conditional reassignment inside a range body poisons the body",
			body: `g := r.Group("/c")
	for _, v := range versions {
		if v != "" {
			g = r.Group("/d")
		}
		server.GET(hr, g, "/in", m.h)
	}`,
			wantUnresolved: pd("g"),
		},
		{
			name: "reassignment inside a closure poisons the closure and everything after it",
			body: `g := r.Group("/c")
	server.GET(hr, g, "/before", m.h)
	reset := func() {
		server.GET(hr, g, "/inner-top", m.h)
		g = r.Group("/d")
		server.GET(hr, g, "/inner", m.h)
	}
	server.GET(hr, g, "/after", m.h)
	g = r.Group("/e")
	reset()
	server.GET(hr, g, "/later", m.h)`,
			wantPaths:      []string{"/c/before"},
			wantUnresolved: pd("g", "g", "g", "g"),
		},
		{
			name: "a closure reading a registrar reassigned after it is path-dependent",
			body: `g := r.Group("/c")
	register := func() {
		server.GET(hr, g, "/deferred", m.h)
	}
	g = r.Group("/d")
	register()
	server.GET(hr, g, "/now", m.h)`,
			wantPaths:      []string{"/d/now"},
			wantUnresolved: pd("g"),
		},
		{
			name: "a closure reading a registrar that is not reassigned after it resolves",
			body: `g := r.Group("/c")
	g = r.Group("/d")
	func() {
		server.GET(hr, g, "/x", m.h)
	}()`,
			wantPaths: []string{"/d/x"},
		},
		{
			name: "a nested group composes onto the binding in effect",
			body: `api := r.Group("/a")
	api = r.Group("/b")
	v1 := api.Group("/v1")
	server.GET(hr, v1, "/x", m.h)
	api = api.Group("/c")
	server.GET(hr, api, "/y", m.h)`,
			wantPaths: []string{"/b/v1/x", "/b/c/y"},
		},
		{
			name: "a group derived from a path-dependent registrar is path-dependent",
			body: `g := r.Group("/c")
	if enabled {
		g = r.Group("/d")
	}
	v1 := g.Group("/v1")
	server.GET(hr, v1, "/x", m.h)`,
			wantUnresolved: pd("v1"),
		},
		{
			name: "a same-block reassignment after a nested one resolves again",
			body: `g := r.Group("/c")
	if enabled {
		g = r.Group("/d")
	}
	g = r.Group("/e")
	server.GET(hr, g, "/x", m.h)`,
			wantPaths: []string{"/e/x"},
		},
		{
			name: "a same-block reassignment after a loop resolves again",
			body: `g := r.Group("/c")
	for i := 0; i < 2; i++ {
		g = r.Group("/d")
	}
	g = r.Group("/e")
	server.GET(hr, g, "/x", m.h)`,
			wantPaths: []string{"/e/x"},
		},
		{
			name: "a registrar declared inside a loop body is straight-line there",
			body: `for i := 0; i < 2; i++ {
		g := r.Group("/a")
		server.GET(hr, g, "/1", m.h)
		g = r.Group("/b")
		server.GET(hr, g, "/2", m.h)
	}`,
			wantPaths: []string{"/a/1", "/b/2"},
		},
		{
			name: "a var registrar assigned in both branches resolves per branch only",
			body: `var api server.RouteRegistrar
	if enabled {
		api = r.Group("/a")
		server.GET(hr, api, "/in-if", m.h)
	} else {
		api = r.Group("/b")
		server.GET(hr, api, "/in-else", m.h)
	}
	server.GET(hr, api, "/after", m.h)`,
			wantPaths:      []string{"/a/in-if", "/b/in-else"},
			wantUnresolved: pd("api"),
		},
		{
			name: "a short declaration reusing an inner registrar does not redeclare it",
			body: `api := r.Group("/outer")
	if enabled {
		api := r.Group("/a")
		server.GET(hr, api, "/1", m.h)
		n, api := 0, other
	}
	server.GET(hr, api, "/2", m.h)`,
			wantPaths: []string{"/a/1", "/outer/2"},
		},
		{
			name: "reassigning the registrar parameter itself",
			body: `r = r.Group("/v1")
	server.GET(hr, r, "/x", m.h)
	if enabled {
		r = r.Group("/v2")
	}
	server.GET(hr, r, "/y", m.h)`,
			wantPaths:      []string{"/v1/x"},
			wantUnresolved: pd("r"),
		},
		{
			// A parameter lives in the function body's own block, so a `:=`
			// naming it assigns the parameter rather than declaring a new
			// variable that would shadow it.
			name: "a short declaration naming a parameter reassigns the parameter",
			body: `n, r := 0, newGroup()
	server.GET(hr, r, "/1", m.h)`,
			wantUnresolved: ut("r"),
		},
		{
			name: "a closure reading a parameter a later short declaration reassigns is path-dependent",
			body: `defer func() {
		server.GET(hr, r, "/late", m.h)
	}()
	v1, r := r.Group("/v1"), r.Group("/api")
	server.GET(hr, v1, "/x", m.h)
	server.GET(hr, r, "/y", m.h)`,
			wantPaths:      []string{"/v1/x", "/api/y"},
			wantUnresolved: pd("r"),
		},
		{
			name: "a short declaration naming a closure parameter reassigns the parameter",
			body: `register := func(g server.RouteRegistrar) {
		g = r.Group("/a")
		defer func() {
			server.GET(hr, g, "/late", m.h)
		}()
		n, g := 0, r.Group("/b")
		server.GET(hr, g, "/now", m.h)
	}
	register(r)`,
			wantPaths:      []string{"/b/now"},
			wantUnresolved: pd("g"),
		},
		{
			name: "a closure created in a reassignment's right-hand side is path-dependent",
			body: `api := r.Group("/a")
	api = newGroup(func() {
		server.GET(hr, api, "/in-closure", m.h)
	})
	server.GET(hr, api, "/after", m.h)`,
			wantUnresolved: append(pd("api"), ut("api")...),
		},
		{
			name: "a reassignment inside a bare block is straight-line code",
			body: `var g server.RouteRegistrar
	api := r.Group("/api")
	{
		g = api.Group("/items")
		server.GET(hr, g, "", m.h)
	}
	server.GET(hr, g, "/after-block", m.h)`,
			wantPaths: []string{"/api/items", "/api/items/after-block"},
		},
		{
			name: "a bare block inside a case or select clause carries its write to the clause",
			body: `v := r.Group("/v")
	switch {
	case enabled:
		{
			v = r.Group("/w")
		}
		server.GET(hr, v, "/case", m.h)
	}
	s := r.Group("/s")
	select {
	case <-ch:
		{
			s = r.Group("/t")
		}
		server.GET(hr, s, "/select", m.h)
	}`,
			wantPaths: []string{"/w/case", "/t/select"},
		},
		{
			name: "an if nested in a bare block leaves the registrar path-dependent after it",
			body: `g := r.Group("/g")
	{
		if enabled {
			g = r.Group("/x")
		}
	}
	server.GET(hr, g, "/after-nested-if", m.h)`,
			wantUnresolved: pd("g"),
		},
		{
			name: "a select clause's reassignment leaves the registrar path-dependent after the select",
			body: `s := r.Group("/s")
	select {
	case <-ch:
		s = r.Group("/s2")
	default:
	}
	server.GET(hr, s, "/after-select", m.h)`,
			wantUnresolved: pd("s"),
		},
		{
			// The inner `:=` takes effect at the end of its statement, so the
			// write it records is to the inner api, never to the outer one
			// the closure reads.
			name: "a closure reading an outer registrar a later inner short declaration shadows resolves",
			body: `api := r.Group("/a")
	f := func() {
		server.GET(hr, api, "/x", m.h)
	}
	f()
	if enabled {
		api := r.Group("/b")
		server.GET(hr, api, "/y", m.h)
	}`,
			wantPaths: []string{"/a/x", "/b/y"},
		},
		{
			name: "a select clause's own short declaration shadows only inside the clause",
			body: `api := r.Group("/a")
	select {
	case api := <-ch:
		server.GET(hr, api, "/in", m.h)
	default:
		server.GET(hr, api, "/default", m.h)
	}
	server.GET(hr, api, "/after", m.h)`,
			wantPaths:      []string{"/a/default", "/a/after"},
			wantUnresolved: ut("api"),
		},
		{
			name: "a non-group local shadowing a group registrar is untraced",
			body: `api := r.Group("/a")
	if enabled {
		api := other
		server.GET(hr, api, "/x", m.h)
	}`,
			wantUnresolved: ut("api"),
		},
		{
			name: "a closure parameter shadowing a group registrar is untraced",
			body: `api := r.Group("/api")
	register := func(api server.RouteRegistrar) {
		server.GET(hr, api, "/users", m.h)
		api.Add("GET", "/health", m.h)
	}
	register(api)`,
			wantUnresolved: ut("api", "api"),
		},
		{
			name: "a closure parameter shadowing the prefix-less root registrar stays a registrar",
			body: `register := func(r server.RouteRegistrar) {
		r.Add("GET", "/health", m.h)
	}
	register(r)`,
			wantPaths: []string{"/health"},
		},
		{
			name: "a range variable shadowing a group registrar is untraced",
			body: `api := r.Group("/a")
	for _, api := range groups {
		server.GET(hr, api, "/x", m.h)
	}`,
			wantUnresolved: ut("api"),
		},
		{
			name: "a registrar declared in a for-loop init and reassigned in its body is path-dependent",
			body: `for g := r.Group("/a"); i < 2; i++ {
		server.GET(hr, g, "/top", m.h)
		g = g.Group("/b")
		server.GET(hr, g, "/bottom", m.h)
	}`,
			wantUnresolved: pd("g", "g"),
		},
		{
			name: "a registrar declared in a for-loop init and reassigned in its post statement is path-dependent",
			body: `for g := r.Group("/a"); i < 2; g = g.Group("/b") {
		server.GET(hr, g, "/x", m.h)
	}`,
			wantUnresolved: pd("g"),
		},
		{
			name: "a registrar declared in a for-loop init and never reassigned resolves",
			body: `for g := r.Group("/a"); i < 2; i++ {
		server.GET(hr, g, "/x", m.h)
	}`,
			wantPaths: []string{"/a/x"},
		},
		{
			name: "an else branch sees the binding from before the if",
			body: `g := r.Group("/c")
	if enabled {
		g = r.Group("/d")
		server.GET(hr, g, "/in-if", m.h)
	} else if other {
		server.GET(hr, g, "/in-else-if", m.h)
	} else {
		server.GET(hr, g, "/in-else", m.h)
	}
	server.GET(hr, g, "/after", m.h)`,
			wantPaths:      []string{"/d/in-if", "/c/in-else-if", "/c/in-else"},
			wantUnresolved: pd("g"),
		},
		{
			name: "a later case clause sees the binding from before the switch",
			body: `k := r.Group("/k")
	switch {
	case enabled:
		k = r.Group("/k2")
		server.GET(hr, k, "/case1", m.h)
	default:
		server.GET(hr, k, "/case2", m.h)
	}
	server.GET(hr, k, "/after", m.h)`,
			wantPaths:      []string{"/k2/case1", "/k/case2"},
			wantUnresolved: pd("k"),
		},
		{
			name: "a case clause reached by fallthrough sees the registrar as path-dependent",
			body: `k := r.Group("/k")
	switch {
	case enabled:
		k = r.Group("/k2")
		server.GET(hr, k, "/case1", m.h)
		fallthrough
	case other:
		fallthrough
	case more:
		server.GET(hr, k, "/chained", m.h)
	case empty:
	default:
		server.GET(hr, k, "/default", m.h)
	}
	server.GET(hr, k, "/after", m.h)`,
			wantPaths:      []string{"/k2/case1", "/k/default"},
			wantUnresolved: pd("k", "k"),
		},
		{
			// go/types accepts a labeled fallthrough and one followed by empty
			// statements as the clause's final statement; both still reach
			// the next clause.
			name: "a labeled or semicolon-trailed fallthrough still reaches the next clause",
			body: `k := r.Group("/k")
	switch {
	case enabled:
		k = r.Group("/k2")
		if other {
			goto L
		}
	L:
		fallthrough
	case more:
		server.GET(hr, k, "/labeled", m.h)
	}
	j := r.Group("/j")
	switch {
	case enabled:
		j = r.Group("/j2")
		fallthrough;;
	case more:
		server.GET(hr, j, "/semicolon", m.h)
	}`,
			wantUnresolved: pd("k", "j"),
		},
		{
			name: "an assignment in an if statement's init always runs",
			body: `a := r.Group("/x")
	if a = r.Group("/always"); enabled {
		server.GET(hr, a, "/in", m.h)
	}
	server.GET(hr, a, "/after", m.h)`,
			wantPaths: []string{"/always/in", "/always/after"},
		},
		{
			name: "an if-init registrar reassigned in the body keeps its init value in the else",
			body: `if g := r.Group("/a"); enabled {
		g = r.Group("/b")
		server.GET(hr, g, "/in-if", m.h)
	} else {
		server.GET(hr, g, "/in-else", m.h)
	}`,
			wantPaths: []string{"/b/in-if", "/a/in-else"},
		},
		{
			name: "a registrar assigned another registrar takes that registrar's binding",
			body: `api := r.Group("/a")
	server.GET(hr, api, "/1", m.h)
	api = r
	server.GET(hr, api, "/2", m.h)
	v1 := r.Group("/v1")
	if enabled {
		v1 := v1
		server.GET(hr, v1, "/3", m.h)
	}`,
			wantPaths: []string{"/a/1", "/2", "/v1/3"},
		},
		{
			name: "a non-group reassignment inside a nested block or loop is path-dependent",
			body: `api := r.Group("/api")
	legacy := r.Group("/legacy")
	if enabled {
		api = legacy
	}
	server.GET(hr, api, "/after-if", m.h)
	g := r.Group("/g")
	for range items {
		server.GET(hr, g, "/in-loop", m.h)
		g = legacy
	}`,
			wantUnresolved: pd("api", "g"),
		},
		{
			name: "a registrar reassigned to an untraceable value is untraced",
			body: `api := r.Group("/a")
	api = newGroup()
	server.GET(hr, api, "/x", m.h)
	v1 := api.Group("/v1")
	server.GET(hr, v1, "/y", m.h)`,
			wantUnresolved: ut("api", "v1"),
		},
		{
			name: "tuple assignments pair each registrar with its own value",
			body: `api := r.Group("/a")
	var other server.RouteRegistrar
	api, other = r.Group("/b"), r.Group("/o")
	server.GET(hr, api, "/1", m.h)
	server.GET(hr, other, "/2", m.h)
	x, y := r.Group("/x"), api.Group("/y")
	server.GET(hr, x, "/3", m.h)
	server.GET(hr, y, "/4", m.h)
	n, api := 0, r.Group("/c")
	server.GET(hr, api, "/5", m.h)
	t := r.Group("/t")
	if enabled {
		t, _ = r.Group("/t2"), 0
	}
	server.GET(hr, t, "/6", m.h)`,
			wantPaths:      []string{"/b/1", "/o/2", "/x/3", "/b/y/4", "/c/5"},
			wantUnresolved: pd("t"),
		},
		{
			name: "var declarations bind registrars",
			body: `var api = r.Group("/a")
	server.GET(hr, api, "/1", m.h)
	api = r.Group("/b")
	server.GET(hr, api, "/2", m.h)
	var g server.RouteRegistrar = r.Group("/g")
	server.GET(hr, g, "/3", m.h)`,
			wantPaths: []string{"/a/1", "/b/2", "/g/3"},
		},
		{
			name: "a grouped var declaration composes each spec onto the ones before it",
			body: `var (
		api   = r.Group("/api")
		admin = api.Group("/admin")
	)
	server.GET(hr, admin, "/stats", m.h)
	if enabled {
		var (
			r = r.Group("/x")
			v = r.Group("/v")
		)
		server.GET(hr, v, "/inner", m.h)
	}`,
			wantPaths: []string{"/api/admin/stats", "/x/v/inner"},
		},
		{
			name: "a range clause assigning a registrar is path-dependent",
			body: `g := r.Group("/g")
	for _, g = range groups {
		server.GET(hr, g, "/in", m.h)
	}
	server.GET(hr, g, "/after", m.h)`,
			wantUnresolved: pd("g", "g"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, routes := analyzeSingleModule(t, scopedModuleSrc("", tt.body))
			if tt.wantPaths == nil {
				assert.Empty(t, routes)
			} else {
				assert.Equal(t, tt.wantPaths, orderedRoutePaths(routes))
			}

			unresolved := a.UnresolvedRoutes(t.Context())
			require.Len(t, unresolved, len(tt.wantUnresolved), "unresolved: %+v", unresolved)
			for i, reason := range tt.wantUnresolved {
				assert.Equal(t, reason, unresolved[i].Reason)
			}
			assert.Len(t, a.Warnings(t.Context()), len(tt.wantUnresolved),
				"every Unresolved route must raise exactly one warning: %v", a.Warnings(t.Context()))
		})
	}
}

// TestPathDependentPrefixWarning pins the user-facing diagnostic for both
// registration forms: it names the registrar and the control-flow cause, so it
// cannot be mistaken for an unresolvable prefix argument.
func TestPathDependentPrefixWarning(t *testing.T) {
	a, routes := analyzeSingleModule(t, scopedModuleSrc("",
		"g := r.Group(\"/c\")\n\tif enabled {\n\t\tg = r.Group(\"/d\")\n\t}\n\tserver.GET(hr, g, \"/x\", m.h)"))
	assert.Empty(t, routes)
	require.True(t, containsRawAddSubstr(a.Warnings(t.Context()),
		`unresolved route: skipping the server.GET registration on registrar "g", whose group prefix `+pathDependentReasonTail),
		"expected a path-dependent group-prefix warning, got: %v", a.Warnings(t.Context()))

	dir := writeAnalyzerProject(t, "module.go", rawAddModuleSrc(
		"g := r.Group(\"/c\")\n\tfor i := 0; i < 2; i++ {\n\t\tg = r.Group(\"/d\")\n\t\tg.Add(http.MethodGet, \"/x\", m.ping)\n\t}"))
	ra := New(dir)
	project, err := ra.AnalyzeProject()
	require.NoError(t, err)
	require.Len(t, project.Modules, 1)
	assert.Empty(t, project.Modules[0].Routes)
	require.True(t, containsRawAddSubstr(ra.Warnings(t.Context()),
		`unresolved route: skipping the g.Add registration on registrar "g", whose group prefix `+pathDependentReasonTail),
		"expected a path-dependent group-prefix warning for .Add, got: %v", ra.Warnings(t.Context()))
	unresolved := ra.UnresolvedRoutes(t.Context())
	require.Len(t, unresolved, 1)
	assert.Equal(t, "g.Add", unresolved[0].Form)
}

// TestHelperSeesRegistrarBindingAtCallSite pins that a helper called with a
// reassigned registrar inherits the prefix in effect at each call, and that a
// path-dependent registrar handed to a helper makes the helper's routes
// Unresolved routes rather than letting them fall back to a guessed prefix.
func TestHelperSeesRegistrarBindingAtCallSite(t *testing.T) {
	const helper = "func (m *Module) sub(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" +
		"\tserver.GET(hr, r, \"/x\", m.h)\n}"

	_, routes := analyzeSingleModule(t, scopedModuleSrc(helper,
		"api := r.Group(\"/a\")\n\tm.sub(hr, api)\n\tapi = r.Group(\"/b\")\n\tm.sub(hr, api)"))
	assert.Equal(t, []string{"/a/x", "/b/x"}, orderedRoutePaths(routes))

	a, routes := analyzeSingleModule(t, scopedModuleSrc(helper,
		"api := r.Group(\"/a\")\n\tif enabled {\n\t\tapi = r.Group(\"/b\")\n\t}\n\tm.sub(hr, api)"))
	assert.Empty(t, routes)
	unresolved := a.UnresolvedRoutes(t.Context())
	require.Len(t, unresolved, 1)
	assert.Equal(t, `group prefix on registrar "r" `+pathDependentReasonTail, unresolved[0].Reason)
}

// TestReassignedRegistrarStillWarnsWhenPassed pins that a reassigned registrar
// is still recognized as one when handed to a call the walk cannot follow, so
// the dropped-registrations warning keeps firing.
func TestReassignedRegistrarStillWarnsWhenPassed(t *testing.T) {
	a, _ := analyzeSingleModule(t, scopedModuleSrc("",
		"api := r.Group(\"/a\")\n\tapi = r.Group(\"/b\")\n\tregisterElsewhere(hr, api)"))
	assert.True(t, containsRawAddSubstr(a.Warnings(t.Context()), "skipping registerElsewhere(...): it receives a route registrar"),
		"expected the passed-registrar warning, got: %v", a.Warnings(t.Context()))
}

// TestGroupCallArg covers the expression shapes that open a group registrar
// and the ones that do not.
func TestGroupCallArg(t *testing.T) {
	tests := []struct {
		expr       string
		wantParent string
		wantOK     bool
	}{
		{expr: `r.Group("/a")`, wantParent: "r", wantOK: true},
		{expr: `api.Group("/a", mw)`, wantParent: "api", wantOK: true},
		{expr: `prefix`},
		{expr: `group("/a")`},
		{expr: `r.Other("/a")`},
		{expr: `r.Group()`},
		{expr: `m.r.Group("/a")`},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.expr)
			require.NoError(t, err)

			parent, arg, ok := groupCallArg(expr)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			assert.Equal(t, tt.wantParent, parent.Name)
			assert.NotNil(t, arg)
		})
	}

	_, _, ok := groupCallArg(nil)
	assert.False(t, ok, "a write with no right-hand side of its own opens no group")
}

// TestBindingWrites pins which statements write variables, what each target
// receives, and where the writes take effect. A declaration's writes are its
// specs', one spec at a time.
func TestBindingWrites(t *testing.T) {
	tests := []struct {
		stmt     string
		wantLHS  []string
		wantRHS  []bool // whether each target receives an expression of its own
		wantAtOK bool   // whether the writes take effect at a real position
	}{
		{stmt: `a := r.Group("/a")`, wantLHS: []string{"a"}, wantRHS: []bool{true}, wantAtOK: true},
		{stmt: `a, _, b = x, y, z`, wantLHS: []string{"a", "b"}, wantRHS: []bool{true, true}, wantAtOK: true},
		{stmt: `a, b := f()`, wantLHS: []string{"a", "b"}, wantRHS: []bool{false, false}, wantAtOK: true},
		{stmt: `m.api = r`, wantLHS: []string{}, wantAtOK: true},
		{stmt: `a += b`},
		{stmt: `var a, b = x, y`, wantLHS: []string{"a", "b"}, wantRHS: []bool{true, true}, wantAtOK: true},
		{stmt: `var (a = x; b = a.Group("/b"))`, wantLHS: []string{"a"}, wantRHS: []bool{true}, wantAtOK: true},
		{stmt: `var a server.RouteRegistrar`, wantAtOK: true},
		{stmt: `const a = "/a"`, wantLHS: []string{"a"}, wantRHS: []bool{true}, wantAtOK: true},
		{stmt: `for _, g = range gs {}`, wantLHS: []string{"g"}, wantRHS: []bool{false}, wantAtOK: true},
		{stmt: `for _, g := range gs {}`},
		{stmt: `f(x)`},
	}
	for _, tt := range tests {
		t.Run(tt.stmt, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "p.go", "package p\nfunc f() {\n\t"+tt.stmt+"\n}", 0)
			require.NoError(t, err)
			fn, ok := file.Decls[0].(*ast.FuncDecl)
			require.True(t, ok)

			// A declaration writes spec by spec, so its first spec is the
			// node that carries the writes.
			var node ast.Node = fn.Body.List[0]
			if decl, isDecl := node.(*ast.DeclStmt); isDecl {
				gen, isGen := decl.Decl.(*ast.GenDecl)
				require.True(t, isGen)
				node = gen.Specs[0]
			}
			writes, at := bindingWrites(node)
			assert.Equal(t, tt.wantAtOK, at.IsValid())
			require.Len(t, writes, len(tt.wantLHS))
			for i, write := range writes {
				assert.Equal(t, tt.wantLHS[i], write.lhs.Name)
				assert.Equal(t, tt.wantRHS[i], write.rhs != nil)
			}
		})
	}
}

// TestRegistrarWriteTakesEffectAfterItsRightHandSide pins that a call inside a
// registrar write's right-hand side still sees the binding from before the
// write, as Go evaluates it: the helper registers under the old prefix, and
// only the routes after the statement see the new, untraced value.
func TestRegistrarWriteTakesEffectAfterItsRightHandSide(t *testing.T) {
	const helper = "func (m *Module) wrap(hr *server.HandlerRegistry, r server.RouteRegistrar) server.RouteRegistrar {\n" +
		"\tserver.GET(hr, r, \"/wrapped\", m.h)\n\treturn r\n}"

	a, routes := analyzeSingleModule(t, scopedModuleSrc(helper,
		"api := r.Group(\"/a\")\n\tapi = m.wrap(hr, api)\n\tserver.GET(hr, api, \"/after\", m.h)"))
	assert.Equal(t, []string{"/a/wrapped"}, orderedRoutePaths(routes))
	unresolved := a.UnresolvedRoutes(t.Context())
	require.Len(t, unresolved, 1)
	assert.Equal(t, ut("api"), []string{unresolved[0].Reason})
}

// TestUntracedPrefixWarning pins the user-facing diagnostic for a registrar
// whose value cannot be followed to a Group(...) call.
func TestUntracedPrefixWarning(t *testing.T) {
	a, routes := analyzeSingleModule(t, scopedModuleSrc("",
		"api := r.Group(\"/a\")\n\tapi = newGroup()\n\tserver.GET(hr, api, \"/x\", m.h)"))
	assert.Empty(t, routes)
	require.True(t, containsRawAddSubstr(a.Warnings(t.Context()),
		`unresolved route: skipping the server.GET registration on registrar "api", whose group prefix `+untracedReasonTail),
		"expected an untraced group-prefix warning, got: %v", a.Warnings(t.Context()))
}

// TestHiddenDeclarations pins which outer declarations an identifier's own
// declaration hides.
func TestHiddenDeclarations(t *testing.T) {
	outer := map[string]binding{"api": {from: 10}, "r": {from: 1}}
	inner := map[string]binding{"api": {from: 20}, "later": {from: 90}}
	scopes := constScopes(nil).push(outer).push(inner)

	assert.Equal(t, []token.Pos{10, token.NoPos}, scopes.hidden("api", 30), "the inner api hides the outer one, then the package level")
	assert.Equal(t, []token.Pos{token.NoPos}, scopes.hidden("api", 15), "before the inner api enters scope, the outer one is innermost")
	assert.Equal(t, []token.Pos{token.NoPos}, scopes.hidden("r", 30))
	assert.Empty(t, scopes.hidden("later", 30), "a declaration not yet in scope hides nothing")
	assert.Empty(t, scopes.hidden("other", 30))
}

// TestRegistrarPrefixForNonIdentifier pins that a registrar argument that is
// not an identifier carries no prefix and is not a failure.
func TestRegistrarPrefixForNonIdentifier(t *testing.T) {
	prefix, recv := registrarPrefixFor(&ast.SelectorExpr{X: ast.NewIdent("m"), Sel: ast.NewIdent("group")}, bindingView{})
	assert.Equal(t, registrarPrefix{resolved: true}, prefix)
	assert.Empty(t, recv)
}

// TestRegistrarFramesDeclareWhatTheWalkDeclares pins that a function body's
// frame leaves a name its signature declares to the signature, as the scoped
// walk does: `n, r := 1, f(r)` assigns the parameter r rather than declaring a
// body-level r, in a function literal's body as in the walked one.
func TestRegistrarFramesDeclareWhatTheWalkDeclares(t *testing.T) {
	src := `package p
func f(r int) {
	n, r := 1, g(r)
	_ = func(q int) {
		k, q := 2, g(q)
		_ = k
	}
	_ = n
}`
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	require.NoError(t, err)
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	require.True(t, ok)
	outer := funcTypeBindings(fn.Type, "", fn.Body.Pos())
	tracker := newRegistrarTracker(fn.Body, outer, nil)

	scopes := map[*ast.BlockStmt]map[string]binding{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		tracker.advance(n)
		if block, isBlock := n.(*ast.BlockStmt); isBlock && tracker.top.node == n {
			scopes[block] = tracker.top.scope
		}
		return true
	})

	lit, ok := fn.Body.List[1].(*ast.AssignStmt).Rhs[0].(*ast.FuncLit)
	require.True(t, ok)
	assert.Contains(t, scopes[fn.Body], "n")
	assert.NotContains(t, scopes[fn.Body], "r", "the walked body's frame must leave its parameter to the signature")
	assert.Contains(t, scopes[lit.Body], "k")
	assert.NotContains(t, scopes[lit.Body], "q", "a function literal's body frame must leave its parameter to the signature")
}
