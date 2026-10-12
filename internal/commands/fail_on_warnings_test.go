package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

const (
	failOnWarningsMsg = "spec written with warnings and --fail-on-warnings is set"
	strictWarnedMsg   = "spec generated with warnings and --strict is set"
	successLinePrefix = "✓ OpenAPI specification generated: "
	staleSpecContent  = "openapi: 3.0.1\n# stale\n"
)

// generateCaptured runs runGenerate with opts and returns its stdout and error.
func generateCaptured(t *testing.T, opts *GenerateOptions) (string, error) {
	t.Helper()
	var runErr error
	stdout := testutil.CaptureStdout(t, func() { runErr = runGenerate(t.Context(), opts) })
	return stdout, runErr
}

// seedStaleOutput writes a placeholder spec at out, standing in for the
// committed spec a CI drift gate regenerates over.
func seedStaleOutput(t *testing.T, out string) {
	t.Helper()
	require.NoError(t, os.WriteFile(out, []byte(staleSpecContent), 0o600))
}

// plainSpec returns the spec a run without --fail-on-warnings writes for dir:
// what a --fail-on-warnings run must leave on disk too.
func plainSpec(t *testing.T, dir string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), outputFileName)
	_, err := generateCaptured(t, &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	require.NoError(t, err, "a run without the flag must succeed despite the warning")
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	return content
}

// TestRunGenerateFailOnWarningsWritesThenFails pins the drift-gate contract on a
// project with one unresolved-route warning: the spec is written (replacing, not
// deleting, any existing output), the summary and success lines are printed as
// on a plain run, and only then does the run fail with the flag's error.
func TestRunGenerateFailOnWarningsWritesThenFails(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), pathDependentModSrc)
	want := plainSpec(t, dir)

	t.Run("no pre-existing file", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), outputFileName)

		stdout, err := generateCaptured(t, &GenerateOptions{ProjectRoot: dir, OutputFile: out, FailOnWarnings: true})

		require.EqualError(t, err, failOnWarningsMsg)
		assert.Contains(t, stdout, "Warnings: 1\n")
		assert.Contains(t, stdout, unresolvedCountLine+"\n")
		assert.Contains(t, stdout, successLinePrefix+out+"\n")
		got, readErr := os.ReadFile(out)
		require.NoError(t, readErr, "the spec must be written before the run fails")
		assert.Equal(t, string(want), string(got))
	})

	t.Run("pre-existing file is replaced, not deleted", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), outputFileName)
		seedStaleOutput(t, out)

		_, err := generateCaptured(t, &GenerateOptions{ProjectRoot: dir, OutputFile: out, FailOnWarnings: true})

		require.EqualError(t, err, failOnWarningsMsg)
		got, readErr := os.ReadFile(out)
		require.NoError(t, readErr, "the existing output must not be removed")
		assert.Equal(t, string(want), string(got), "the existing output must hold the fresh spec")
	})
}

// TestRunGenerateFailOnWarningsCleanRunUnchanged pins that the flag is inert on
// a warning-free project: the run succeeds, and stdout and the written file are
// byte-identical to a run without it. Both runs use one output path, since
// stdout names it.
func TestRunGenerateFailOnWarningsCleanRunUnchanged(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), warningSummaryModSrc)
	out := filepath.Join(t.TempDir(), outputFileName)

	plainStdout, err := generateCaptured(t, &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	require.NoError(t, err)
	plainContent, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NoError(t, os.Remove(out), "the flagged run must write the file afresh")

	flagStdout, err := generateCaptured(t, &GenerateOptions{ProjectRoot: dir, OutputFile: out, FailOnWarnings: true})
	require.NoError(t, err, "--fail-on-warnings must not fail a run that emitted no warning")
	flagContent, err := os.ReadFile(out)
	require.NoError(t, err)

	assert.Contains(t, flagStdout, "Warnings: 0\n")
	assert.Equal(t, plainStdout, flagStdout)
	assert.Equal(t, string(plainContent), string(flagContent))
}

// TestRunGenerateFailOnWarningsValidationFailure pins that a --validate failure
// keeps today's behavior under the flag: the validation error is returned, the
// existing output is removed, and the post-write gate is never reached. An
// undeclared server URL variable fails validation on any project.
func TestRunGenerateFailOnWarningsValidationFailure(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), pathDependentModSrc)
	out := filepath.Join(t.TempDir(), outputFileName)
	seedStaleOutput(t, out)

	stdout, err := generateCaptured(t, &GenerateOptions{
		ProjectRoot: dir, OutputFile: out, FailOnWarnings: true, Validate: true,
		Servers: []string{"{undefinedvar}"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated spec failed validation")
	assert.NotContains(t, err.Error(), failOnWarningsMsg)
	assert.NoFileExists(t, out, "a failed validation must not leave an artifact")
	assert.NotContains(t, stdout, successLinePrefix)
}

// TestRunGenerateStrictWinsOverFailOnWarnings pins the precedence when both
// flags are set: the strict gate runs before anything is rendered, so the run
// fails with the --strict error, removes the existing output and never prints
// the success line.
func TestRunGenerateStrictWinsOverFailOnWarnings(t *testing.T) {
	dir := writeProject(t, warningSummaryGoMod(), pathDependentModSrc)
	out := filepath.Join(t.TempDir(), outputFileName)
	seedStaleOutput(t, out)

	stdout, err := generateCaptured(t, &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true, FailOnWarnings: true})

	require.EqualError(t, err, strictWarnedMsg)
	assert.NoFileExists(t, out, "--strict must still leave no artifact when both flags are set")
	assert.NotContains(t, stdout, successLinePrefix)
}

// TestWriteSpecWriteFailures pins that a failed write returns its own error
// before the fail-on-warnings gate is reached.
func TestWriteSpecWriteFailures(t *testing.T) {
	t.Run("output directory cannot be created", func(t *testing.T) {
		notADir := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(notADir, nil, 0o600))
		opts := &GenerateOptions{OutputFile: filepath.Join(notADir, outputFileName), FailOnWarnings: true}

		err := writeSpec(opts, staleSpecContent, true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create output directory")
	})

	t.Run("output file cannot be written", func(t *testing.T) {
		opts := &GenerateOptions{OutputFile: t.TempDir(), FailOnWarnings: true} // a directory

		err := writeSpec(opts, staleSpecContent, true)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write output file")
	})
}

// TestNewGenerateCommandFailOnWarningsFlag pins the flag's registration: a bool
// that defaults to off, so a run without it behaves as before.
func TestNewGenerateCommandFailOnWarningsFlag(t *testing.T) {
	flag := NewGenerateCommand().Flags().Lookup("fail-on-warnings")
	require.NotNil(t, flag, "missing --fail-on-warnings flag")
	assert.Equal(t, "bool", flag.Value.Type())
	assert.Equal(t, "false", flag.DefValue)
}
