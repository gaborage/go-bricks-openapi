package analyzer

import (
	"os"
	"path/filepath"
	"strconv"
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
	detachedMsg        = "not directly above an analyzed route registration"

	pathDependentGroupPrefixMsg = "whose group prefix depends on control flow"
	untracedGroupPrefixMsg      = "whose group prefix cannot be traced"
)

// directiveLoc is the "file:line: " prefix of a directive diagnostic raised on
// the bodyLine-th line (1-based) of the body analyzeDirectiveModule wraps.
func directiveLoc(bodyLine int) string {
	line := strings.Count(directiveModuleHeader, "\n") + bodyLine
	return filepath.Join("mod", "module.go") + ":" + strconv.Itoa(line) + ": "
}

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

// TestDirectivePlacementRule verifies the placement rule: only a comment group
// ENDING on the line directly above the registration call attaches. A group
// separated by a blank line contributes nothing to the route and is reported
// as a Detached directive, located at its directive line.
func TestDirectivePlacementRule(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:errors 404

	server.GET(hr, r, "/things", m.h)`)

	assert.Empty(t, routeByKey(t, routes, directiveGetThings).statuses,
		"a directive separated from the call by a blank line must not attach")
	assert.Equal(t, []string{
		directiveLoc(1) + "directive errors has no effect — it is not directly above an analyzed route registration",
	}, warnings, "a well-formed but unattached directive warns exactly once")
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

// TestDetachedDirectivePlacements verifies every position a recognised
// Directive cannot attach from raises exactly one detached warning and leaves
// the route untouched: one rule covers them all, with no syntactic pass needed
// to tell them apart.
func TestDetachedDirectivePlacements(t *testing.T) {
	tests := []struct {
		name string
		body string
		line int // body line of the directive
	}{
		{
			name: "above an enclosing if",
			body: `	//openapi:public
	if true {
		server.GET(hr, r, "/things", m.h)
	}`,
			line: 1,
		},
		{
			name: "inside the call's arguments",
			body: `	server.GET(hr, r,
		//openapi:public
		"/things", m.h)`,
			line: 2,
		},
		{
			name: "above an unrelated statement",
			body: `	//openapi:public
	h := m.h
	server.GET(hr, r, "/things", h)`,
			line: 1,
		},
		{
			name: "after an opening brace",
			body: `	if true { //openapi:public
		server.GET(hr, r, "/things", m.h)
	}`,
			line: 1,
		},
		{
			name: "after the last registration",
			body: `	server.GET(hr, r, "/things", m.h)
	//openapi:public`,
			line: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes, warnings := analyzeDirectiveModule(t, tt.body)

			assert.False(t, routeByKey(t, routes, directiveGetThings).public, "a detached directive must not attach")
			assert.Equal(t, []string{
				directiveLoc(tt.line) + "directive public has no effect — it is not directly above an analyzed route registration",
			}, warnings)
		})
	}
}

// TestDetachedDirectiveOnlyFirstOfTwoGroups verifies that of two directive
// groups separated by a blank line, the one ending directly above the call
// attaches and only the other is reported.
func TestDetachedDirectiveOnlyFirstOfTwoGroups(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	//openapi:public

	//openapi:errors 404
	server.GET(hr, r, "/things", m.h)`)

	got := routeByKey(t, routes, directiveGetThings)
	assert.False(t, got.public, "the separated group must not attach")
	assert.Equal(t, []int{404}, got.statuses, "the adjacent group still attaches")
	require.Len(t, warningsContaining(warnings, detachedMsg), 1)
	assert.Equal(t, directiveLoc(1)+"directive public has no effect — it is not directly above an analyzed route registration", warnings[0])
}

// TestDetachedDirectiveNamesEveryRecognisedDirective verifies a detached group
// is reported once, naming each recognised directive once in source order, at
// the line of its FIRST recognised directive — the unknown name is left to the
// unknown-directive diagnostic.
func TestDetachedDirectiveNamesEveryRecognisedDirective(t *testing.T) {
	_, warnings := analyzeDirectiveModule(t, `	// Legacy probe.
	//openapi:deprecated
	//openapi:public
	//openapi:errors 404
	//openapi:errors 409

	server.GET(hr, r, "/things", m.h)`)

	require.Len(t, warnings, 2)
	assert.Contains(t, warnings[0], `unknown directive "deprecated"`)
	assert.Equal(t, directiveLoc(3)+"directives public, errors have no effect — they are not directly above an analyzed route registration", warnings[1])
}

// TestDetachedDirectiveUnknownOnlyGroup verifies a group holding only unknown
// names is reported as unknown and never also as detached.
func TestDetachedDirectiveUnknownOnlyGroup(t *testing.T) {
	_, warnings := analyzeDirectiveModule(t, `	//openapi:deprecated

	server.GET(hr, r, "/things", m.h)`)

	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "unknown directive")
}

// TestDirectiveAboveDroppedRegistrationNotDetached verifies a group attaches as
// soon as the call below it is recognised as a registration, before its path,
// prefix, or method resolves — including a group prefix that depends on
// control flow or cannot be traced: the route's own skip diagnostic already
// covers it, so no detached warning is added.
func TestDirectiveAboveDroppedRegistrationNotDetached(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		dropWarn string
	}{
		{
			name: "unresolved path",
			body: `	p := "/things"
	//openapi:public
	server.GET(hr, r, p, m.h)`,
			dropWarn: "unresolved route",
		},
		{
			name: "unresolved group prefix",
			body: `	pre := "/v1"
	g := r.Group(pre)
	//openapi:errors 404
	server.GET(hr, g, "/things", m.h)`,
			dropWarn: "unresolved route",
		},
		{
			name: "RegisterHandler with a non-static method",
			body: `	meth := "GET"
	//openapi:public
	server.RegisterHandler(hr, r, meth, "/things", m.h)`,
			dropWarn: "skipping a server.RegisterHandler route",
		},
		{
			name: "Add with a non-static method",
			body: `	meth := "GET"
	//openapi:public
	r.Add(meth, "/things", m.h)`,
			dropWarn: "skipping a r.Add route",
		},
		{
			name: "Add with too few arguments",
			body: `	//openapi:public
	r.Add("GET", "/things")`,
			dropWarn: "skipping a r.Add route",
		},
		{
			name: "registrar reassigned inside an if, route after it",
			body: `	api := r.Group("/a")
	if m != nil {
		api = r.Group("/b")
	}
	//openapi:public
	server.GET(hr, api, "/things", m.h)`,
			dropWarn: pathDependentGroupPrefixMsg,
		},
		{
			name: "Add on a registrar reassigned inside an if, route after it",
			body: `	api := r.Group("/a")
	if m != nil {
		api = r.Group("/b")
	}
	//openapi:errors 404
	api.Add("GET", "/things", m.h)`,
			dropWarn: pathDependentGroupPrefixMsg,
		},
		{
			name: "closure parameter shadowing a group registrar",
			body: `	api := r.Group("/a")
	register := func(api server.RouteRegistrar) {
		//openapi:public
		server.GET(hr, api, "/things", m.h)
	}
	register(api)`,
			dropWarn: untracedGroupPrefixMsg,
		},
		{
			name: "Add on a closure parameter shadowing a group registrar",
			body: `	api := r.Group("/a")
	register := func(api server.RouteRegistrar) {
		//openapi:public
		api.Add("GET", "/things", m.h)
	}
	register(api)`,
			dropWarn: untracedGroupPrefixMsg,
		},
		{
			name: "route in a loop whose registrar the loop reassigns",
			body: `	api := r.Group("/a")
	for i := 0; i < 2; i++ {
		//openapi:public
		server.GET(hr, api, "/things", m.h)
		api = r.Group("/b")
	}`,
			dropWarn: pathDependentGroupPrefixMsg,
		},
		{
			name: "Add in a loop whose registrar the loop reassigns",
			body: `	api := r.Group("/a")
	for i := 0; i < 2; i++ {
		//openapi:public
		api.Add("GET", "/things", m.h)
		api = r.Group("/b")
	}`,
			dropWarn: pathDependentGroupPrefixMsg,
		},
		{
			name: "route in a closure on a registrar reassigned after it",
			body: `	api := r.Group("/a")
	register := func() {
		//openapi:errors 404
		server.GET(hr, api, "/things", m.h)
	}
	api = r.Group("/b")
	register()`,
			dropWarn: pathDependentGroupPrefixMsg,
		},
		{
			name: "Add in a closure on a registrar reassigned after it",
			body: `	api := r.Group("/a")
	register := func() {
		//openapi:errors 404
		api.Add("GET", "/things", m.h)
	}
	api = r.Group("/b")
	register()`,
			dropWarn: pathDependentGroupPrefixMsg,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes, warnings := analyzeDirectiveModule(t, tt.body)

			assert.Empty(t, routes)
			assert.Len(t, warningsContaining(warnings, tt.dropWarn), 1)
			assert.Empty(t, warningsContaining(warnings, detachedMsg), "a recognised registration attaches its directives even when dropped")
		})
	}
}

// TestDirectiveAboveUnrecognisedAddIsDetached verifies an .Add on a local that
// is not a group registrar at the call's position — one that is a group
// registrar only in a sibling block, or only from a later write in its own
// block — is not a registration, so the directive above it attaches to nothing
// and is reported as detached, while the other route keeps its prefix.
func TestDirectiveAboveUnrecognisedAddIsDetached(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		route        string
		detachedLine int
	}{
		{
			name: "group registrar only in a sibling block",
			body: `	if m != nil {
		f := r.Group("/f")
		server.GET(hr, f, "/things", m.h)
	} else {
		f := m.api
		//openapi:public
		f.Add("GET", "/things", m.h)
	}`,
			route:        "GET /f/things",
			detachedLine: 6,
		},
		{
			name: "group registrar only after a later write in the same block",
			body: `	late := m.api
	//openapi:public
	late.Add("GET", "/late", m.h)
	late = r.Group("/late-x")
	server.GET(hr, late, "/things", m.h)`,
			route:        "GET /late-x/things",
			detachedLine: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes, warnings := analyzeDirectiveModule(t, tt.body)

			require.Len(t, routes, 1)
			assert.Equal(t, tt.route, routes[0].key)
			assert.False(t, routes[0].public)
			assert.Equal(t, []string{
				directiveLoc(tt.detachedLine) + "directive public has no effect — it is not directly above an analyzed route registration",
			}, warnings)
		})
	}
}

// TestTrailingDirectiveNeverAttaches verifies a directive sharing its line with
// preceding code forms its own comment group that attaches to nothing — not
// even the registration on the next line, which it used to make public — and
// is reported as detached.
func TestTrailingDirectiveNeverAttaches(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	server.GET(hr, r, "/things", m.h) //openapi:public
	server.POST(hr, r, "/things", m.h)`)

	assert.False(t, routeByKey(t, routes, directiveGetThings).public)
	assert.False(t, routeByKey(t, routes, "POST /things").public, "a trailing directive must not attach to the next line")
	assert.Equal(t, []string{
		directiveLoc(1) + "directive public has no effect — it is not directly above an analyzed route registration",
	}, warnings)
}

// TestTrailingErrorsDirectiveNeverAttaches verifies the trailing rule does not
// depend on which directive the group holds: a trailing errors directive adds
// no status to the registration on the next line either.
func TestTrailingErrorsDirectiveNeverAttaches(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	server.POST(hr, r, "/things", m.h) //openapi:errors 404
	server.GET(hr, r, "/things", m.h)`)

	assert.Empty(t, routeByKey(t, routes, directiveGetThings).statuses, "a trailing directive must not attach to the next line")
	assert.Equal(t, []string{
		directiveLoc(1) + "directive errors has no effect — it is not directly above an analyzed route registration",
	}, warnings)
}

// TestTrailingHumanCommentKeepsNextGroupAttached verifies a trailing human
// comment on one registration does not stop the directive group on the next
// line from attaching to the registration below it.
func TestTrailingHumanCommentKeepsNextGroupAttached(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	server.GET(hr, r, "/things", m.h) // the list
	//openapi:public
	server.POST(hr, r, "/things", m.h)`)

	assert.True(t, routeByKey(t, routes, "POST /things").public)
	assert.Empty(t, warnings)
}

// TestDirectiveReachedTwiceAttachesOnce verifies a registration the walk
// reaches more than once (a helper called with two registrars) attaches its
// directive idempotently: it applies to both routes and never warns.
func TestDirectiveReachedTwiceAttachesOnce(t *testing.T) {
	routes, warnings := analyzeDirectiveModule(t, `	v1 := r.Group("/v1")
	v2 := r.Group("/v2")
	m.reg(hr, v1)
	m.reg(hr, v2)
}
func (m *Module) reg(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:public
	server.GET(hr, r, "/things", m.h)`)

	assert.True(t, routeByKey(t, routes, "GET /v1/things").public)
	assert.True(t, routeByKey(t, routes, "GET /v2/things").public)
	assert.Empty(t, warnings)
}

// TestDetachedDirectivesSortedAcrossFiles verifies detached warnings come out
// in (file, line) order whatever order the files were discovered and indexed
// in, cover registrations the walk never reaches (a function outside
// RegisterRoutes, a near-miss module), and are identical on a repeated run of
// the same analyzer.
func TestDetachedDirectivesSortedAcrossFiles(t *testing.T) {
	files := map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.25\n",
		filepath.Join("mod", "module.go"): directiveModuleHeader + `	//openapi:public

	server.GET(hr, r, "/things", m.h)
}
`,
		filepath.Join("mod", "extra.go"): `package mod

import "github.com/gaborage/go-bricks/server"

func unwalked(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:public
	server.GET(hr, r, "/unwalked", nil)

	//openapi:errors 404
	server.GET(hr, r, "/unwalked2", nil)
}
`,
		filepath.Join("api", "near.go"): `package api

import "github.com/gaborage/go-bricks/app"

type Module struct{}

func (m *Module) Name() string                      { return "api" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                   { return nil }
func (m *Module) RegisterRoutes(r any) {
	//openapi:public
	server.GET(nil, r, "/near", nil)
}
`,
	}
	a := analyzeDirectiveProject(t, files)
	require.Len(t, warningsContaining(a.Warnings(t.Context()), "an unrecognized RegisterRoutes signature"), 1,
		"api is a near-miss module, so its registration is never walked")
	first := warningsContaining(a.Warnings(t.Context()), detachedMsg)

	public := "directive public has no effect — it is not directly above an analyzed route registration"
	assert.Equal(t, []string{
		filepath.Join("api", "near.go") + ":11: " + public,
		filepath.Join("mod", "extra.go") + ":6: " + public,
		filepath.Join("mod", "extra.go") + ":9: directive errors has no effect — it is not directly above an analyzed route registration",
		directiveLoc(1) + public,
	}, first)

	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	assert.Equal(t, first, warningsContaining(a.Warnings(t.Context()), detachedMsg), "a repeated run reports the same detached warnings once")
}

// analyzeDirectiveProject writes files (project-relative path -> source) into
// a fresh project root and runs one analysis over it.
func analyzeDirectiveProject(t *testing.T, files map[string]string) *ProjectAnalyzer {
	t.Helper()
	dir := t.TempDir()
	for rel, src := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o600))
	}
	a := New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	return a
}

// TestDirectivesOutsideTheWalkedServiceNotDetached verifies a file parsed only
// to resolve an import — here a nested Go module, which discovery skips as
// another service — is never policed for detached directives: its correctly
// placed directive sits above a registration this service never walks, and
// reporting it would fail this service's --strict over another's annotations.
func TestDirectivesOutsideTheWalkedServiceNotDetached(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": "module github.com/example/app\n\ngo 1.25\n",
		filepath.Join("mod", "module.go"): `package mod

import (
	"github.com/example/app/billing/api"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }
func (m *Module) h(ctx server.HandlerContext) (server.Result[api.Invoice], server.IAPIError) {
	return server.NewResult(200, api.Invoice{}), nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/invoices", m.h)
}
`,
		filepath.Join("billing", "go.mod"): "module github.com/example/app/billing\n\ngo 1.25\n",
		filepath.Join("billing", "api", "api.go"): `package api

import "github.com/gaborage/go-bricks/server"

type Invoice struct {
	ID int64 ` + "`json:\"id\"`" + `
}

func RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:public
	server.GET(hr, r, "/invoices", nil)
}
`,
	})

	require.Contains(t, a.typeRegistry, "Invoice", "the nested module's package must be parsed for this test to mean anything")
	assert.Empty(t, a.Warnings(t.Context()))
}

// platformMethod is a registration helper declared in several build-tagged
// copies by TestDetachedDirectiveInBuildTaggedDuplicateIsDeterministic.
const platformMethod = `
func (m *Module) platform(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:public
	server.GET(hr, r, "/health", m.h)
}
`

// TestDetachedDirectiveInBuildTaggedDuplicateIsDeterministic verifies that a
// registration helper split across build-tagged files (build constraints are
// ignored) always resolves to the same copy — the calling file's own copy when
// it declares one, otherwise the copy in the first file by name — so the
// directive in the other copy is reported as detached at the same location on
// every run rather than wherever map iteration happened to land.
func TestDetachedDirectiveInBuildTaggedDuplicateIsDeterministic(t *testing.T) {
	platform := func(constraint string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\nimport \"github.com/gaborage/go-bricks/server\"\n" + platformMethod
	}
	calls := directiveModuleHeader + "\tm.platform(hr, r)\n}\n"
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "first file by name when the calling file declares none",
			files: map[string]string{
				filepath.Join("mod", "module.go"):         calls,
				filepath.Join("mod", "platform_linux.go"): platform("linux"),
				filepath.Join("mod", "platform_other.go"): platform("!linux"),
			},
			want: filepath.Join("mod", "platform_other.go"),
		},
		{
			// a_other.go sorts before module.go: a by-name-only lookup would
			// walk it and report the calling file's copy instead.
			name: "calling file's own copy wins over an earlier file by name",
			files: map[string]string{
				filepath.Join("mod", "module.go"):  calls + platformMethod,
				filepath.Join("mod", "a_other.go"): platform("!linux"),
			},
			want: filepath.Join("mod", "a_other.go"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.files["go.mod"] = "module example.com/app\n\ngo 1.25\n"
			want := []string{tt.want + ":8: directive public has no effect — it is not directly above an analyzed route registration"}
			for range 16 {
				assert.Equal(t, want, analyzeDirectiveProject(t, tt.files).Warnings(t.Context()))
			}
		})
	}
}
