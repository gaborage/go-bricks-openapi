package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validOpenAPISpec is a minimal but structurally valid OpenAPI 3.0 document used
// to drive the validate command's happy path.
const validOpenAPISpec = `openapi: 3.0.1
info:
  title: Test API
  version: 1.0.0
paths: {}
`

// runValidateCmd writes content to a temp file (when non-empty), then runs the
// validate command against the given path, returning its stdout and error. A
// blank content means "do not create the file" so the missing-file arm can be
// exercised.
func runValidateCmd(t *testing.T, filename, content string, createFile bool) (string, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), filename)
	if createFile {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("failed to write spec file: %v", err)
		}
	}

	cmd := NewValidateCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{path})

	err := cmd.Execute()
	return out.String(), err
}

func TestRunValidate(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		content     string
		createFile  bool
		expectError bool
		wantStdout  string
	}{
		{
			name:       "valid spec",
			filename:   "openapi.yaml",
			content:    validOpenAPISpec,
			createFile: true,
			wantStdout: "valid OpenAPI 3.0 document",
		},
		{
			name:        "broken spec (not yaml)",
			filename:    "broken.yaml",
			content:     ":\n  not: [valid",
			createFile:  true,
			expectError: true,
		},
		{
			name:        "spec missing required fields",
			filename:    "incomplete.yaml",
			content:     "foo: bar\n",
			createFile:  true,
			expectError: true,
		},
		{
			name:        "missing file",
			filename:    "does-not-exist.yaml",
			createFile:  false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runValidateCmd(t, tt.filename, tt.content, tt.createFile)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected an error, got none (stdout: %q)", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, tt.wantStdout) {
				t.Errorf("expected stdout to contain %q, got: %q", tt.wantStdout, out)
			}
		})
	}
}

func TestNewValidateCommand(t *testing.T) {
	cmd := NewValidateCommand()

	if cmd == nil {
		t.Fatal("NewValidateCommand() returned nil")
	}
	if cmd.Use != "validate [spec]" {
		t.Errorf("Expected Use 'validate [spec]', got %s", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Command should have a short description")
	}
	if cmd.Long == "" {
		t.Error("Command should have a long description")
	}
	if cmd.RunE == nil {
		t.Error("Command should have a RunE function")
	}
}

// TestValidateDefaultsToGenerateOutput confirms that running validate with no
// argument targets the same default file the generate command writes to.
func TestValidateDefaultsToGenerateOutput(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, defaultOutputFile)
	if err := os.WriteFile(specPath, []byte(validOpenAPISpec), 0600); err != nil {
		t.Fatalf("failed to write default spec: %v", err)
	}

	// Run from the temp dir so the default relative path resolves to specPath.
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	cmd := NewValidateCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil) // no positional argument -> default path

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error validating default spec: %v", err)
	}
	if !strings.Contains(out.String(), defaultOutputFile) {
		t.Errorf("expected confirmation to reference %q, got: %q", defaultOutputFile, out.String())
	}
}
