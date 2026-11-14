package commands

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// duplicateRoutesFixture is the golden fixture whose modules register pairs
// of routes on one go-bricks route node (issue 178).
var duplicateRoutesFixture = filepath.Join("..", "spectest", "testdata", "duplicate_routes")

// duplicateRouteMarker is a substring of every duplicate-route warning.
const duplicateRouteMarker = "documenting the first; the second is dropped from the spec"

// TestRunGenerateDuplicateRoutesStrict pins that the duplicate-route warnings
// reach generate: --strict fails on them, while the spec itself validates.
func TestRunGenerateDuplicateRoutesStrict(t *testing.T) {
	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{
			ProjectRoot: duplicateRoutesFixture, OutputFile: out, Strict: true, Validate: true,
		})
	})
	require.Error(t, runErr)
	assert.Contains(t, stdout, "Warnings: 7\n") // the warning lines themselves go to stderr
	warnings := analyzerWarnings(t, duplicateRoutesFixture)
	require.Len(t, warnings, 7)
	for _, w := range warnings {
		assert.Contains(t, w, duplicateRouteMarker)
	}

	stdout = testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{
			ProjectRoot: duplicateRoutesFixture, OutputFile: out, Validate: true,
		})
	})
	require.NoError(t, runErr, stdout)
}

// TestRunDoctorDuplicateRoutes pins that doctor surfaces the same warnings
// and counts only the routes kept.
func TestRunDoctorDuplicateRoutes(t *testing.T) {
	stdout := testutil.CaptureStdout(t, func() {
		_ = runDoctor(t.Context(), &DoctorOptions{ProjectRoot: duplicateRoutesFixture})
	})
	assert.Contains(t, stdout, "Routes discovered: 7\n")
	assert.Equal(t, 7, strings.Count(stdout, duplicateRouteMarker), stdout)
	assert.NotContains(t, stdout, "All checks passed")
}
