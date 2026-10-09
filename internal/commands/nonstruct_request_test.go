package commands

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/specvalidate"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// nonStructRequestModSrc is a module whose one route, POST /items, takes a
// request of type REQ and returns RESULT. Address is the struct control and
// response; the imports back the well-known and third-party request rows.
const nonStructRequestModSrc = `package svc

import (
	"encoding/json"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Address struct {
	Street string ` + "`json:\"street\"`" + `
}

type Status string

var (
	_ json.RawMessage
	_ decimal.Decimal
)

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/items", m.create, server.WithTags("svc"))
}

func (m *Module) create(req REQ, ctx server.HandlerContext) (RESULT, server.IAPIError) {
	return out, nil
}
`

// writeNonStructRequestProject writes nonStructRequestModSrc with the given
// request type and handler result as a project and returns its root.
func writeNonStructRequestProject(t *testing.T, req, result string) string {
	t.Helper()
	src := strings.NewReplacer("REQ", req, "RESULT", result).Replace(nonStructRequestModSrc)
	return writeProject(t, "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks "+minGoBricksVer+"\n", src)
}

// nonStructRequestStats analyzes the project and returns doctor's statistics
// for it.
func nonStructRequestStats(t *testing.T, dir string) ProjectStats {
	t.Helper()
	project, err := analyzer.New(dir).AnalyzeProject()
	require.NoError(t, err)
	return calculateProjectStats(project)
}

// TestRunGenerateNonStructRequestFailsStrict pins #102 end to end: a non-struct
// request type warns, so generate --strict exits non-zero and leaves no
// artifact, while a non-strict run still emits a valid document whose
// operation has no requestBody and no orphaned component for the request type.
func TestRunGenerateNonStructRequestFailsStrict(t *testing.T) {
	for _, req := range []string{"string", "*string", "json.RawMessage", "decimal.Decimal", "Status", "[]Address", "map[string]string", "interface{}"} {
		t.Run(req, func(t *testing.T) {
			dir := writeNonStructRequestProject(t, req, "Address")
			a := analyzer.New(dir)
			_, err := a.AnalyzeProject()
			require.NoError(t, err)
			require.Len(t, a.Warnings(t.Context()), 1)
			assert.True(t, strings.HasPrefix(a.Warnings(t.Context())[0], "request type "+req+" is not a struct"), a.Warnings(t.Context())[0])

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
			op := digMap(t, spec.Paths, "/items", "post")
			assert.NotContains(t, op, "requestBody", "a non-struct request documents no body")
			schemas, ok := spec.Components["schemas"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, []string{"Address", "ErrorResponse"}, slices.Sorted(maps.Keys(schemas)), "no component is emitted for the request type")

			strictOut := filepath.Join(t.TempDir(), outputFileName)
			testutil.CaptureStdout(t, func() {
				runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: strictOut, Strict: true})
			})
			require.Error(t, runErr, "--strict must fail on a non-struct request")
			assert.NoFileExists(t, strictOut, "a failed --strict run leaves no artifact")
		})
	}
}

// TestRunGenerateStructRequestPassesStrict is the control: a struct request
// raises no warning, so generate --strict succeeds and documents its body.
func TestRunGenerateStructRequestPassesStrict(t *testing.T) {
	dir := writeNonStructRequestProject(t, "Address", "Address")
	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true})
	})
	require.NoError(t, runErr, stdout)
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	assert.Contains(t, digMap(t, spec.Paths, "/items", "post"), "requestBody")
}

// TestDoctorCountsNonStructRequestUntyped pins doctor's side of #102: a
// non-struct request is an untyped request, so `req string` with an untyped
// response leaves the whole route untyped; a struct request stays typed; and
// the response is classified exactly as before.
func TestDoctorCountsNonStructRequestUntyped(t *testing.T) {
	t.Run("non_struct_request_untyped_response", func(t *testing.T) {
		stats := nonStructRequestStats(t, writeNonStructRequestProject(t, "string", "map[string]chan int"))
		assert.Equal(t, 0, stats.TypedRequestRoutes)
		assert.Equal(t, 0, stats.TypedRoutes)
		assert.Equal(t, []string{"create"}, stats.UntypedRoutes)
	})
	t.Run("struct_request_stays_typed", func(t *testing.T) {
		stats := nonStructRequestStats(t, writeNonStructRequestProject(t, "Address", "map[string]string"))
		assert.Equal(t, 1, stats.TypedRequestRoutes)
		assert.Equal(t, 1, stats.TypedRoutes)
		assert.Empty(t, stats.UntypedRoutes)
	})
	t.Run("response_classification_unchanged", func(t *testing.T) {
		stats := nonStructRequestStats(t, writeNonStructRequestProject(t, "string", "server.Result[int64]"))
		assert.Equal(t, 0, stats.TypedRequestRoutes)
		assert.Equal(t, 1, stats.TypedResponseRoutes, "an inline-scalar response stays typed")
		assert.Equal(t, 1, stats.TypedRoutes)
		assert.Empty(t, stats.UntypedRoutes)
	})
}
