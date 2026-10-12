package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// unbindableParamModSrc is a module whose one route takes a uuid.UUID path
// parameter, an array go-bricks cannot bind.
const unbindableParamModSrc = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type GetReq struct {
	ID uuid.UUID ` + "`param:\"id\"`" + `
}

type Item struct {
	Name string ` + "`json:\"name\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/items/:id", m.get, server.WithTags("svc"))
}

func (m *Module) get(req GetReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.Result[Item]{}, nil
}
`

// TestRunGenerateUnbindableParamFailsStrict pins issue 121 through the CLI: a
// uuid.UUID path parameter raises one warning, so generate --strict fails
// with the strict error and no artifact, while a non-strict run documents the
// parameter as before (a uuid string), and doctor reports a caveat.
func TestRunGenerateUnbindableParamFailsStrict(t *testing.T) {
	dir := writeProject(t, "module github.com/example/svc\n\ngo 1.26.0\n\nrequire github.com/gaborage/go-bricks "+verifiedGoBricksVer+"\n",
		unbindableParamModSrc)
	a := analyzer.New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 1)
	assert.True(t, strings.HasSuffix(warnings[0], `is the path parameter "id" of type uuid.UUID, which go-bricks cannot bind `+
		"(an array), so a request that sends it gets a 400"), warnings[0])

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Validate: true})
	})
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 1", warningsSummary(t, stdout))
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	params, ok := digMap(t, spec.Paths, "/items/{id}", "get")["parameters"].([]any)
	require.True(t, ok)
	require.Len(t, params, 1)
	id, ok := params[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"type": "string", "format": "uuid"}, id["schema"])

	strictOut := filepath.Join(t.TempDir(), outputFileName)
	testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: strictOut, Strict: true})
	})
	require.EqualError(t, runErr, "spec generated with warnings and --strict is set")
	assert.NoFileExists(t, strictOut)

	doctorOut := testutil.CaptureStdout(t, func() {
		runErr = runDoctor(t.Context(), &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion})
	})
	require.NoError(t, runErr, "an unbindable parameter is a caveat, not a hard error")
	assert.Contains(t, doctorOut, "which go-bricks cannot bind (an array)")
	assert.Contains(t, doctorOut, "Ready with caveats")
	assert.NotContains(t, doctorOut, "All checks passed")
}
