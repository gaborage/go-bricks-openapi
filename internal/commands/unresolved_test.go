package commands

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// unresolvedCountLine is the summary line both commands render; it must appear
// verbatim in generate's stdout and inside doctor's diagnostics block.
const unresolvedCountLine = "Unresolved routes: 1"

func TestUnresolvedRouteCountLineEmptyWhenNone(t *testing.T) {
	assert.Empty(t, unresolvedRouteCountLine(nil))
	assert.Empty(t, unresolvedRouteCountLine([]models.UnresolvedRoute{}))
}

func TestUnresolvedRouteCountLineCountsEntries(t *testing.T) {
	routes := []models.UnresolvedRoute{
		{Form: "server.GET", File: "svc/mod.go", Line: 12, Reason: "path argument is not a string literal"},
		{Form: "api.Add", File: "svc/mod.go", Line: 14, Reason: "group prefix on registrar \"api\" is not a string literal"},
	}
	assert.Equal(t, "Unresolved routes: 2", unresolvedRouteCountLine(routes))
}

func TestUnresolvedRouteLocations(t *testing.T) {
	assert.Empty(t, unresolvedRouteLocations(nil))

	routes := []models.UnresolvedRoute{
		{Form: "server.GET", File: "svc/mod.go", Line: 12, Col: 2, Reason: "path argument is not a string literal"},
		{Form: "api.Add", File: "svc/mod.go", Line: 14, Col: 9, Reason: "group prefix is not a string literal"},
	}
	got := unresolvedRouteLocations(routes)
	require.Len(t, got, 2)
	assert.Equal(t, "server.GET at svc/mod.go:12:2 — path argument is not a string literal", got[0])
	assert.Equal(t, "api.Add at svc/mod.go:14:9 — group prefix is not a string literal", got[1])
}

// TestRunGenerateUnresolvedRoutesSummary proves generate prints the count line
// for a project with an Unresolved route.
func TestRunGenerateUnresolvedRoutesSummary(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), warningSummaryDynamicModSrc)
	out := filepath.Join(t.TempDir(), outputFileName)

	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	})

	require.NoError(t, runErr)
	assert.Contains(t, stdout, unresolvedCountLine+"\n")
}

// TestRunGenerateNoUnresolvedRoutesLine proves a clean project's stdout is
// unchanged: the count line is printed only when the count is greater than zero.
func TestRunGenerateNoUnresolvedRoutesLine(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), warningSummaryModSrc)
	out := filepath.Join(t.TempDir(), outputFileName)

	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	})

	require.NoError(t, runErr)
	assert.NotContains(t, stdout, "Unresolved routes:")
}

// TestRunDoctorUnresolvedRoutesBlock proves doctor prints the count line AND one
// located entry per Unresolved route.
func TestRunDoctorUnresolvedRoutesBlock(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), warningSummaryDynamicModSrc)
	opts := &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion}

	var runErr error
	out := testutil.CaptureStdout(t, func() { runErr = runDoctor(t.Context(), opts) })

	require.NoError(t, runErr, "an unresolved route is a caveat, not a hard error")
	assert.Contains(t, out, unresolvedCountLine)
	assert.Contains(t, out, "server.GET at "+testMainGoFile+":")
	assert.Contains(t, out, "path argument is not a string literal or a resolvable constant")
	assert.Contains(t, out, "Ready with caveats")
}

// TestRunDoctorNoUnresolvedRoutesBlock proves a clean project gets neither the
// count line nor any entry.
func TestRunDoctorNoUnresolvedRoutesBlock(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), warningSummaryModSrc)
	opts := &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion}

	var runErr error
	out := testutil.CaptureStdout(t, func() { runErr = runDoctor(t.Context(), opts) })

	require.NoError(t, runErr)
	assert.NotContains(t, out, "Unresolved routes:")
	assert.NotContains(t, out, " at "+testMainGoFile+":")
}

// pathDependentModSrc registers a route on a group whose registrar a nested
// block reassigns: the route after the block has a prefix that depends on
// control flow, so the analyzer drops it as an Unresolved route with a warning.
const pathDependentModSrc = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{ legacy bool }

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Ping struct {
	OK bool ` + "`json:\"ok\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	api := r.Group("/v1")
	server.GET(hr, api, "/ping", m.ping, server.WithTags("svc"))
	if m.legacy {
		api = r.Group("/v0")
	}
	server.GET(hr, api, "/status", m.ping, server.WithTags("svc"))
}

func (m *Module) ping(ctx server.HandlerContext) (server.Result[Ping], server.IAPIError) {
	return server.NewResult(200, Ping{}), nil
}
`

// TestPathDependentGroupPrefixFailsStrict proves a route whose group prefix
// depends on control flow is surfaced end to end: generate --strict fails
// without writing an artifact, and doctor lists the route with its reason.
func TestPathDependentGroupPrefixFailsStrict(t *testing.T) {
	t.Run("generate --strict fails", func(t *testing.T) {
		dir := writeProject(t, warningSummaryGoMod(), pathDependentModSrc)
		out := filepath.Join(t.TempDir(), outputFileName)

		var runErr error
		stdout := testutil.CaptureStdout(t, func() {
			runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true})
		})

		require.Error(t, runErr, "a path-dependent group prefix must fail --strict")
		assert.Contains(t, stdout, unresolvedCountLine+"\n")
		assert.NoFileExists(t, out, "a failed --strict run must not leave an artifact")
	})

	t.Run("doctor lists the reason", func(t *testing.T) {
		dir := writeProject(t, warningSummaryGoMod(), pathDependentModSrc)
		opts := &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion}

		var runErr error
		out := testutil.CaptureStdout(t, func() { runErr = runDoctor(t.Context(), opts) })

		require.NoError(t, runErr, "an unresolved route is a caveat, not a hard error")
		assert.Contains(t, out, unresolvedCountLine)
		assert.Contains(t, out, `group prefix on registrar "api" depends on control flow`)
	})
}
