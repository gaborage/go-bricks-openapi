package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

const (
	modFileRel   = "mod/module.go"
	pathReasonGE = "path argument is not a string literal or a resolvable constant"
)

// TestUnresolvedRoutesRecordServerVerb pins the structured record the analyzer
// collects alongside the warning for a server.VERB registration whose path
// cannot be resolved: one entry, naming the registration form, the source
// location relative to the project root, and the reason.
func TestUnresolvedRoutesRecordServerVerb(t *testing.T) {
	a, routes := analyzeSingleModule(t, scopedModuleSrc("", "server.GET(hr, r, buildPath(), m.h)"))
	assert.Empty(t, routes)

	unresolved := a.UnresolvedRoutes(t.Context())
	require.Len(t, unresolved, 1)
	assert.Equal(t, "server.GET", unresolved[0].Form)
	assert.Equal(t, modFileRel, filepath.ToSlash(unresolved[0].File))
	assert.Positive(t, unresolved[0].Line)
	assert.Positive(t, unresolved[0].Col)
	assert.Contains(t, unresolved[0].Reason, pathReasonGE)
}

// TestUnresolvedRoutesRecordAddForm pins the same record for the raw
// <registrar>.Add registration form.
func TestUnresolvedRoutesRecordAddForm(t *testing.T) {
	dir := writeAnalyzerProject(t, "module.go", rawAddModuleSrc(
		"r.Add(http.MethodGet, buildPath(), m.ping)"))
	a := New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)

	unresolved := a.UnresolvedRoutes(t.Context())
	require.Len(t, unresolved, 1)
	assert.Equal(t, "r.Add", unresolved[0].Form)
	assert.Equal(t, "module.go", filepath.ToSlash(unresolved[0].File))
	assert.Positive(t, unresolved[0].Line)
	assert.Positive(t, unresolved[0].Col)
	assert.Contains(t, unresolved[0].Reason, pathReasonGE)
}

// TestUnresolvedRoutesEmptyForResolvedRoutes proves a project whose every route
// path resolves carries no unresolved-route records.
func TestUnresolvedRoutesEmptyForResolvedRoutes(t *testing.T) {
	a, routes := analyzeSingleModule(t, scopedModuleSrc(`const p = "/pkg"`,
		"const local = \"/local\"\n\tserver.GET(hr, r, local, m.h)\n\tserver.POST(hr, r, p+\"/x\", m.h)"))
	require.Len(t, routes, 2)
	assert.Empty(t, a.UnresolvedRoutes(t.Context()))
}

// TestUnresolvedRoutesOnePerDroppedGroupRoute proves a group whose own prefix
// cannot be resolved yields ONE record per dropped route (not one per group),
// each carrying the registrar name in its reason and its own line.
func TestUnresolvedRoutesOnePerDroppedGroupRoute(t *testing.T) {
	a, routes := analyzeSingleModule(t, scopedModuleSrc("",
		"api := r.Group(compute())\n\tserver.GET(hr, api, \"/widgets\", m.h)\n\tserver.POST(hr, api, \"/widgets\", m.h)"))
	assert.Empty(t, routes)

	unresolved := a.UnresolvedRoutes(t.Context())
	require.Len(t, unresolved, 2)
	assert.Equal(t, "server.GET", unresolved[0].Form)
	assert.Equal(t, "server.POST", unresolved[1].Form)
	assert.NotEqual(t, unresolved[0].Line, unresolved[1].Line)
	for i := range unresolved {
		assert.Contains(t, unresolved[i].Reason, `group prefix on registrar "api"`)
		assert.Equal(t, modFileRel, filepath.ToSlash(unresolved[i].File))
	}
}

// TestUnresolvedRoutesResetBetweenRuns proves the slot is per-run state: a
// second analysis on the same analyzer does not accumulate the first run's
// records.
func TestUnresolvedRoutesResetBetweenRuns(t *testing.T) {
	dir := writeAnalyzerProject(t, "module.go", rawAddModuleSrc(
		"r.Add(http.MethodGet, buildPath(), m.ping)"))
	a := New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	require.Len(t, a.UnresolvedRoutes(t.Context()), 1)

	_, err = a.AnalyzeProject()
	require.NoError(t, err)
	assert.Len(t, a.UnresolvedRoutes(t.Context()), 1, "records must be reset, not accumulated")
}

// TestUnresolvedRoutesWarningTextUnchanged proves the structured record is
// collected ALONGSIDE the existing warning, not in place of it.
func TestUnresolvedRoutesWarningTextUnchanged(t *testing.T) {
	a, _ := analyzeSingleModule(t, scopedModuleSrc("", "server.GET(hr, r, buildPath(), m.h)"))
	require.Len(t, a.UnresolvedRoutes(t.Context()), 1)
	assert.True(t, containsRawAddSubstr(a.Warnings(t.Context()),
		"unresolved route: skipping the server.GET registration, whose path argument is not a string literal or a resolvable constant"),
		"the warning text must be unchanged, got: %v", a.Warnings(t.Context()))
}

// TestUnresolvedRoutesClonedSlice proves the accessor hands back a copy, so a
// caller cannot mutate analyzer state (the Warnings contract).
func TestUnresolvedRoutesClonedSlice(t *testing.T) {
	a, _ := analyzeSingleModule(t, scopedModuleSrc("", "server.GET(hr, r, buildPath(), m.h)"))
	got := a.UnresolvedRoutes(t.Context())
	require.Len(t, got, 1)
	got[0].Form = "mutated"
	assert.Equal(t, "server.GET", a.UnresolvedRoutes(t.Context())[0].Form)
}

// TestUnresolvedRoutesSortedBySourcePosition proves the accessor orders entries
// by (File, Line, Col) rather than by the order the walk happened to record
// them, so the rendered CLI output never depends on file-discovery order.
func TestUnresolvedRoutesSortedBySourcePosition(t *testing.T) {
	a := New(t.TempDir())
	a.unresolvedRoutes = []models.UnresolvedRoute{
		{Form: "b.Add", File: "b/mod.go", Line: 10, Col: 2},
		{Form: "a.late", File: "a/mod.go", Line: 12, Col: 2},
		{Form: "a.samelinelatercol", File: "a/mod.go", Line: 12, Col: 40},
		{Form: "a.early", File: "a/mod.go", Line: 3, Col: 9},
	}

	sorted := a.UnresolvedRoutes(t.Context())
	forms := make([]string, 0, len(sorted))
	for _, r := range sorted {
		forms = append(forms, r.Form)
	}
	assert.Equal(t, []string{"a.early", "a.late", "a.samelinelatercol", "b.Add"}, forms)
}
