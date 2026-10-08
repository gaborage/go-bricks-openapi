package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// fieldFallbackModSrc is a module whose one route returns Body, a struct with
// the single field F of type FIELD; DECLS are extra type declarations.
const fieldFallbackModSrc = `package svc

import (
	"time"
	t "time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

DECLS

type Body struct {
	F FIELD ` + "`json:\"f\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/body", m.body, server.WithTags("svc"))
}

func (m *Module) body(ctx server.HandlerContext) (server.Result[Body], server.IAPIError) {
	return server.Result[Body]{}, nil
}
`

// TestRunGenerateFieldFallbacksFailStrict pins that each warned field row
// raises exactly one warning, so --strict fails with no artifact, while a
// non-strict --validate run emits the row's fallback schema.
func TestRunGenerateFieldFallbacksFailStrict(t *testing.T) {
	object := map[string]any{"type": "object"}
	untyped := map[string]any{}
	goMod := "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks " + minGoBricksVer + "\n"
	cases := []struct {
		name, decls, field string
		want               map[string]any
	}{
		{"marshaler Status", "type Status int\nfunc (s Status) MarshalText() ([]byte, error) { return nil, nil }", "Status", untyped},
		{"marshaler TagsM", "type TagsM []string\nfunc (TagsM) MarshalJSON() ([]byte, error) { return nil, nil }", "TagsM", untyped},
		{"marshaler FlagV", "type FlagV byte\nfunc (f *FlagV) MarshalJSON() ([]byte, error) { return nil, nil }", "[]FlagV",
			map[string]any{"type": "array", "items": untyped}},
		{"recursive Tree", "type Tree map[string]Tree", "Tree", map[string]any{"type": "object", "additionalProperties": untyped}},
		{"third-party", "", "decimal.Decimal", object},
		{"aliased import", "", "t.Time", object},
		{"defined over time.Time", "type StampD time.Time", "StampD", object},
		{"defined over uuid.UUID", "type IDD uuid.UUID", "IDD", object},
		{"error", "", "error", object},
		{"complex wrapper", "type C complex128", "C", object},
		{"uintptr wrapper", "type Addr uintptr", "Addr", object},
		{"uintptr map value", "", "map[string]uintptr", map[string]any{"type": "object", "additionalProperties": object}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := strings.NewReplacer("DECLS", c.decls, "FIELD", c.field).Replace(fieldFallbackModSrc)
			dir := writeProject(t, goMod, src)

			out := filepath.Join(t.TempDir(), outputFileName)
			var runErr error
			stdout := testutil.CaptureStdout(t, func() {
				runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true})
			})
			require.Error(t, runErr, "a fallback field must fail --strict")
			assert.Contains(t, stdout, "Warnings: 1\n", "the field's one warning is the only diagnostic")
			_, statErr := os.Stat(out)
			assert.True(t, os.IsNotExist(statErr), "strict failure must not leave an artifact")

			out = filepath.Join(t.TempDir(), outputFileName)
			testutil.CaptureStdout(t, func() {
				runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Validate: true})
			})
			require.NoError(t, runErr, "a non-strict run must emit a valid document")
			content, err := os.ReadFile(out)
			require.NoError(t, err)
			var spec OpenAPISpec
			require.NoError(t, yaml.Unmarshal(content, &spec))
			props := digMap(t, spec.Components, "schemas", "Body", "properties")
			assert.Equal(t, c.want, props["f"])
		})
	}
}

// TestRunGenerateNamedResolutionFixtureStrictClean pins that the
// named_resolution fixture, every resolvable row, passes --strict --validate
// through the CLI path (which, unlike spectest, also runs the go-bricks
// version check).
func TestRunGenerateNamedResolutionFixtureStrictClean(t *testing.T) {
	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{
			ProjectRoot: filepath.Join("..", "spectest", "testdata", "named_resolution"),
			OutputFile:  out, Strict: true, Validate: true,
		})
	})
	require.NoError(t, runErr, stdout)
	assert.Contains(t, stdout, "Warnings: 0\n")
}

// TestRunDoctorMarshalerFieldIsCaveat pins that a project whose only issue is
// a Marshaler body field is ready with caveats, and doctor prints the warning.
func TestRunDoctorMarshalerFieldIsCaveat(t *testing.T) {
	src := strings.NewReplacer(
		"DECLS", "type Status int\nfunc (s Status) MarshalText() ([]byte, error) { return nil, nil }",
		"FIELD", "Status",
	).Replace(fieldFallbackModSrc)
	dir := writeProject(t, "module test\n\ngo 1.26.0\n\nrequire github.com/gaborage/go-bricks "+verifiedGoBricksVer+"\n", src)

	var runErr error
	out := testutil.CaptureStdout(t, func() {
		runErr = runDoctor(t.Context(), &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion})
	})
	require.NoError(t, runErr, "a Marshaler field is a caveat, not a hard error")
	assert.Contains(t, out, "Status has its own MarshalText method, which encoding/json uses instead")
	assert.Contains(t, out, "Ready with caveats")
	assert.NotContains(t, out, "All checks passed")
}
