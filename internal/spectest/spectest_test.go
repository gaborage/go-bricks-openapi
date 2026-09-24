package spectest

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// update regenerates the golden files instead of comparing against them.
// Run: go test ./internal/spectest -update
var update = flag.Bool("update", false, "update golden expected.yaml files")

const (
	testdataDir = "testdata"
	goldenFile  = "expected.yaml"
)

// TestGoldenFixtures runs every testdata/<case>/ project through the full
// analyze->generate pipeline, validates the emitted document as OpenAPI 3.0,
// and compares it (line-ending-normalized) against the checked-in golden file.
// This is the regression net every later generator change relies on: a fidelity
// fix shows up as a reviewable golden diff, and a structural regression fails
// Validate.
func TestGoldenFixtures(t *testing.T) {
	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)

	ran := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ran++

		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(testdataDir, name)

			spec, genErr := Generate(t.Context(), dir)
			require.NoError(t, genErr)

			// Primary gate: the emitted document must be valid OpenAPI 3.0.
			require.NoError(t, Validate(t.Context(), []byte(spec)), "emitted spec is not valid OpenAPI 3.0")

			goldenPath := filepath.Join(dir, goldenFile)
			if *update {
				require.NoError(t, os.WriteFile(goldenPath, []byte(spec), 0o600))
				return
			}

			want, readErr := os.ReadFile(goldenPath)
			require.NoError(t, readErr, "missing golden file; run `go test ./internal/spectest -update`")
			// Compare with line endings normalized: the generator always emits LF,
			// but git may check the golden out with CRLF on Windows (autocrlf).
			assert.Equal(t, normalizeEOL(string(want)), normalizeEOL(spec),
				"generated spec drifted from golden; run `go test ./internal/spectest -update` to refresh")
		})
	}

	require.Positive(t, ran, "no fixtures found under testdata/")
}

// normalizeEOL collapses CRLF to LF so golden comparisons are stable across
// platforms. The generator emits LF; only the checked-out golden can carry CRLF.
func normalizeEOL(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// TestValidateRejectsMalformedSpec locks the negative arm of Validate so the
// harness cannot silently degrade into a no-op gate that passes everything.
func TestValidateRejectsMalformedSpec(t *testing.T) {
	t.Run("not_yaml", func(t *testing.T) {
		require.Error(t, Validate(t.Context(), []byte(":\n  not: [valid")))
	})
	t.Run("missing_required_fields", func(t *testing.T) {
		// Parses as YAML but is not a valid OpenAPI document (no openapi/info/paths).
		require.Error(t, Validate(t.Context(), []byte("foo: bar\n")))
	})
}

// TestGenerateEmptyProject exercises Generate on a directory with no modules so
// the helper's success path is covered independently of the golden fixtures. A
// project with zero discovered modules is not an error at the analyzer layer
// (the CLI surfaces that separately); the helper must still return a document.
func TestGenerateEmptyProject(t *testing.T) {
	spec, err := Generate(t.Context(), t.TempDir())
	require.NoError(t, err)
	require.NotEmpty(t, spec)
}

// widthsModule is the module file of TestNamedScalarBuildTaggedWidths; each
// case declares Word and UWord itself, in build-tagged sibling files.
const widthsModule = `package words

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "words" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Packet struct {
	Size  Word   ` + "`json:\"size\"`" + `
	Sizes []Word ` + "`json:\"sizes\"`" + `
	Count UWord  ` + "`json:\"count\"`" + `
}

func (m *Module) get(ctx server.HandlerContext) (server.Result[Packet], server.IAPIError) {
	return server.NewResult(http.StatusOK, Packet{}), nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/packets", m.get)
}
`

// TestNamedScalarBuildTaggedWidths runs the real analyze -> generate ->
// validate pipeline over a project whose named scalars are declared in
// build-tagged files. Build constraints are not evaluated, so when the
// variants disagree on width the schema carries the type alone — no format,
// no unsigned floor — rather than one target's width. When they agree, the
// builtin's format and floor are kept.
func TestNamedScalarBuildTaggedWidths(t *testing.T) {
	integer := map[string]any{"type": "integer"}
	cases := []struct {
		name                    string
		amd64, i386             string // declarations in words_amd64.go / words_386.go
		size, count, sizesItems map[string]any
	}{
		{
			name:  "widths disagree",
			amd64: "type Word int64\ntype UWord uint64\n",
			i386:  "type Word int32\ntype UWord uint32\n",
			size:  integer, count: integer, sizesItems: integer,
		},
		{
			name:       "widths agree",
			amd64:      "type Word int64\ntype UWord uint32\n",
			i386:       "type Word int64\ntype UWord uint32\n",
			size:       map[string]any{"type": "integer", "format": "int64"},
			count:      map[string]any{"type": "integer", "format": "int32", "minimum": 0},
			sizesItems: map[string]any{"type": "integer", "format": "int64"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, src := range map[string]string{
				"go.mod":         "module github.com/example/words\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks v0.53.0\n",
				"module.go":      widthsModule,
				"words_amd64.go": "//go:build amd64\n\npackage words\n\n" + c.amd64,
				"words_386.go":   "//go:build 386\n\npackage words\n\n" + c.i386,
			} {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600))
			}

			spec, err := Generate(t.Context(), dir)
			require.NoError(t, err)
			require.NoError(t, Validate(t.Context(), []byte(spec)))

			var doc struct {
				Components struct {
					Schemas map[string]struct {
						Properties map[string]map[string]any `yaml:"properties"`
					} `yaml:"schemas"`
				} `yaml:"components"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
			props := doc.Components.Schemas["Packet"].Properties
			require.NotNil(t, props, "Packet component missing:\n%s", spec)
			assert.Equal(t, c.size, props["size"], "size")
			assert.Equal(t, c.count, props["count"], "count")
			assert.Equal(t, map[string]any{"type": "array", "items": c.sizesItems}, props["sizes"], "sizes")
		})
	}
}
