package commands

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/specvalidate"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// TestRunGenerateMultiPointerRequestFailsStrict pins #132 end to end: a
// request written as **T raises the one non-struct request warning, so
// generate --strict exits non-zero and leaves no artifact, while a non-strict
// run still emits a valid document whose operation has no requestBody and no
// component for the request type. Doctor counts the request untyped; the
// route stays typed through its response.
func TestRunGenerateMultiPointerRequestFailsStrict(t *testing.T) {
	dir := writeNonStructRequestProject(t, "**Address", "Address")

	a := analyzer.New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	require.Len(t, a.Warnings(t.Context()), 1)
	assert.Contains(t, a.Warnings(t.Context())[0], "request type **Address is not a struct")

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	})
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 1", warningsSummary(t, stdout), "exactly one warning for the route")

	content, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NoError(t, specvalidate.Validate(t.Context(), content))
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	assert.NotContains(t, digMap(t, spec.Paths, "/items", "post"), "requestBody", "a **T request documents no body")
	schemas, ok := spec.Components["schemas"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"Address", "ErrorResponse"}, slices.Sorted(maps.Keys(schemas)), "no component is emitted for the request type")

	strictOut := filepath.Join(t.TempDir(), outputFileName)
	testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: strictOut, Strict: true})
	})
	require.Error(t, runErr, "--strict must fail on a **T request")
	assert.NoFileExists(t, strictOut, "a failed --strict run leaves no artifact")

	stats := nonStructRequestStats(t, dir)
	assert.Equal(t, 0, stats.TypedRequestRoutes)
	assert.Equal(t, 1, stats.TypedRoutes, "the route stays typed through its response")
	assert.Empty(t, stats.UntypedRoutes)
}

// TestRunDoctorMultiPointerRequestIsCaveat pins doctor's printed side of
// #132: before the fix a **T request was skipped silently and doctor said
// "All checks passed"; now it prints the warning and the caveat banner.
func TestRunDoctorMultiPointerRequestIsCaveat(t *testing.T) {
	dir := writeNonStructRequestProject(t, "**Address", "Address")

	var runErr error
	out := testutil.CaptureStdout(t, func() {
		runErr = runDoctor(t.Context(), &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion})
	})
	require.NoError(t, runErr, "a **T request is a caveat, not a hard error")
	assert.Contains(t, out, "request type **Address is not a struct")
	assert.Contains(t, out, "Ready with caveats")
	assert.NotContains(t, out, "All checks passed")
}
