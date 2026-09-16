package analyzer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// directiveModuleHeader is the boilerplate every directive fixture needs: a
// go-bricks module with one handler per route under test.
const directiveModuleHeader = `package mod
import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)
type Module struct{}
func (m *Module) Name() string { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error { return nil }
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }
func (m *Module) h(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) { return server.NewResult(200, Thing{}), nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
`

const (
	directiveGetThings = "GET /things"
	publicNoArgsMsg    = "directive public takes no arguments"
)

// analyzeDirectiveModule runs the analyzer over a module whose RegisterRoutes
// body is the given source, returning the discovered routes and the warnings.
func analyzeDirectiveModule(t *testing.T, body string) (routes []routeAndWarnings, warnings []string) {
	t.Helper()
	a, rs := analyzeSingleModule(t, directiveModuleHeader+body+"\n}\n")
	for i := range rs {
		routes = append(routes, routeAndWarnings{key: rs[i].Method + " " + rs[i].Path, statuses: rs[i].ErrorStatuses, public: rs[i].Public})
	}
	return routes, a.Warnings(t.Context())
}

type routeAndWarnings struct {
	key      string
	statuses []int
	public   bool
}

func routeByKey(t *testing.T, routes []routeAndWarnings, key string) routeAndWarnings {
	t.Helper()
	for _, r := range routes {
		if r.key == key {
			return r
		}
	}
	t.Fatalf("route %q not found", key)
	return routeAndWarnings{}
}

// warningsContaining returns the warnings whose text contains sub.
func warningsContaining(warnings []string, sub string) []string {
	var out []string
	for _, w := range warnings {
		if strings.Contains(w, sub) {
			out = append(out, w)
		}
	}
	return out
}

// TestParseDirective locks the line parser: prefix recognition, the name/args
// split, and the discarded trailing human comment.
func TestParseDirective(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantName string
		wantArgs string
		wantOK   bool
	}{
		{name: "public", text: "//openapi:public", wantName: "public", wantOK: true},
		{name: "errors list", text: "//openapi:errors 404,409", wantName: "errors", wantArgs: "404,409", wantOK: true},
		{name: "spaces after commas", text: "//openapi:errors 404, 409", wantName: "errors", wantArgs: "404, 409", wantOK: true},
		{name: "trailing comment", text: "//openapi:errors 404 // not found", wantName: "errors", wantArgs: "404", wantOK: true},
		{name: "tab before args", text: "//openapi:errors\t404,409", wantName: "errors", wantArgs: "404,409", wantOK: true},
		{name: "tab after public", text: "//openapi:public\t", wantName: "public", wantOK: true},
		{name: "indented", text: "\t//openapi:public ", wantName: "public", wantOK: true},
		{name: "ordinary comment", text: "// just a comment", wantOK: false},
		{name: "go generate", text: "//go:generate mockgen", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args, ok := parseDirective(tt.text)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

// TestErrorsDirectiveStampsStatuses verifies an //openapi:errors directive
// directly above a registration call lands on the route, tolerating whitespace
// after commas and a trailing human comment.
func TestErrorsDirectiveStampsStatuses(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors 404, 409
	server.GET(hr, r, "/things", m.h)

	// Conflicts only.
	//openapi:errors 409 // documented in the runbook
	server.POST(hr, r, "/things", m.h)

	server.DELETE(hr, r, "/things", m.h)`)

	assert.Empty(t, warnings, "well-formed directives must raise no diagnostics")
	assert.Equal(t, []int{404, 409}, routeByKey(t, routes, directiveGetThings).statuses)
	assert.Equal(t, []int{409}, routeByKey(t, routes, "POST /things").statuses)
	assert.Empty(t, routeByKey(t, routes, "DELETE /things").statuses, "a route with no directive declares no error statuses")
}

// TestErrorsDirectiveBadTokens verifies each malformed or out-of-range token
// raises a diagnostic naming it while the valid tokens are still applied.
func TestErrorsDirectiveBadTokens(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors 404,999,abc,399
	server.GET(hr, r, "/things", m.h)`)

	assert.Equal(t, []int{404}, routeByKey(t, routes, directiveGetThings).statuses,
		"valid tokens survive alongside invalid ones")
	for _, tok := range []string{`"999"`, `"abc"`, `"399"`} {
		assert.Len(t, warningsContaining(warnings, tok), 1, "expected exactly one diagnostic naming %s", tok)
	}
	assert.Empty(t, warningsContaining(warnings, `"404"`), "a valid token must not be diagnosed")
}

// TestErrorsDirectiveTabSeparatedArguments verifies a tab between the directive
// name and its arguments parses as arguments, not as part of the name.
func TestErrorsDirectiveTabSeparatedArguments(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, "\t//openapi:errors\t404,409\n\tserver.GET(hr, r, \"/things\", m.h)")

	assert.Equal(t, []int{404, 409}, routeByKey(t, routes, directiveGetThings).statuses)
	assert.Empty(t, warnings, "a tab-separated directive must not be reported as unknown")
}

// TestErrorsDirectiveRejectsSignedTokens verifies a signed token is treated as
// malformed rather than being silently accepted (+404) or read as merely
// out-of-range (-404) — strconv.Atoi alone would do both.
func TestErrorsDirectiveRejectsSignedTokens(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors +404,-404,409
	server.GET(hr, r, "/things", m.h)`)

	assert.Equal(t, []int{409}, routeByKey(t, routes, directiveGetThings).statuses)
	assert.Len(t, warningsContaining(warnings, `"+404"`), 1)
	assert.Len(t, warningsContaining(warnings, `"-404"`), 1)
}

// TestErrorsDirectiveSkipsEmptyTokens verifies a trailing or doubled comma is
// tolerated silently — an empty token names nothing worth reporting.
func TestErrorsDirectiveSkipsEmptyTokens(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors 404,,409,
	server.GET(hr, r, "/things", m.h)`)

	assert.Equal(t, []int{404, 409}, routeByKey(t, routes, directiveGetThings).statuses)
	assert.Empty(t, warnings, "empty tokens must not raise a diagnostic")
}

// TestErrorsDirectiveDeduplicatesAndSorts verifies the stamped list is
// deterministic: repeats collapse and the codes come out ascending, whether the
// repeat is within one directive or across two in the same comment group.
func TestErrorsDirectiveDeduplicatesAndSorts(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors 409,404,404
	//openapi:errors 409,403
	server.GET(hr, r, "/things", m.h)`)

	assert.Equal(t, []int{403, 404, 409}, routeByKey(t, routes, directiveGetThings).statuses)
	assert.Empty(t, warnings)
}

// TestErrorsDirectiveRangeEdges pins the accepted range at 400-599 inclusive.
func TestErrorsDirectiveRangeEdges(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors 400,599,600
	server.GET(hr, r, "/things", m.h)`)

	assert.Equal(t, []int{400, 599}, routeByKey(t, routes, directiveGetThings).statuses)
	assert.Len(t, warningsContaining(warnings, `"600"`), 1)
}

// TestErrorsDirectiveNoArguments diagnoses an argument-less errors directive.
func TestErrorsDirectiveNoArguments(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors
	server.GET(hr, r, "/things", m.h)`)

	assert.Empty(t, routeByKey(t, routes, directiveGetThings).statuses)
	assert.Len(t, warningsContaining(warnings, "directive errors requires at least one status code"), 1)
}

// TestPublicDirectiveWithArguments verifies public takes no arguments: the
// route is still public, but the stray argument raises a diagnostic that feeds
// --strict.
func TestPublicDirectiveWithArguments(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:public extra
	server.GET(hr, r, "/things", m.h)`)

	assert.True(t, routeByKey(t, routes, directiveGetThings).public)
	require.Len(t, warningsContaining(warnings, publicNoArgsMsg), 1)
	assert.Contains(t, warnings[0], `"extra"`, "the diagnostic names the offending argument")
}

// TestUnknownDirectiveDiagnosed verifies an unrecognised directive name is
// reported rather than silently dropped.
func TestUnknownDirectiveDiagnosed(t *testing.T) {
	_, warnings := analyzeDirectiveModule(t, `	//openapi:deprecated soon
	server.GET(hr, r, "/things", m.h)`)

	require.Len(t, warningsContaining(warnings, "unknown directive"), 1)
	assert.Contains(t, warnings[0], `"deprecated"`)
}

// TestDirectivePlacementRule verifies the placement rule is unchanged: only a
// comment group ENDING on the line directly above the registration call
// attaches. A group separated by a blank line contributes nothing to the route.
func TestDirectivePlacementRule(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors 404

	server.GET(hr, r, "/things", m.h)`)

	assert.Empty(t, routeByKey(t, routes, directiveGetThings).statuses,
		"a directive separated from the call by a blank line must not attach")
	assert.Empty(t, warnings, "a well-formed but unattached directive still parses cleanly")
}

// TestDirectivesCombineInOneGroup verifies both directives in a single comment
// group attach to the same registration.
func TestDirectivesCombineInOneGroup(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	// Health probe.
	//openapi:public
	//openapi:errors 503
	server.GET(hr, r, "/things", m.h)`)

	got := routeByKey(t, routes, directiveGetThings)
	assert.True(t, got.public)
	assert.Equal(t, []int{503}, got.statuses)
	assert.Empty(t, warnings)
}

// TestDirectiveDiagnosticsAreNotDuplicated verifies re-analysis of the same
// project reports each directive diagnostic once per run — the per-file index
// latch must not let repeated parses multiply warnings.
func TestDirectiveDiagnosticsAreNotDuplicated(t *testing.T) {
	_, warnings := analyzeDirectiveModule(t, `	//openapi:public extra
	server.GET(hr, r, "/things", m.h)
	//openapi:public extra
	server.POST(hr, r, "/things", m.h)`)

	assert.Len(t, warningsContaining(warnings, publicNoArgsMsg), 2,
		"one diagnostic per offending directive line, none repeated")
}
