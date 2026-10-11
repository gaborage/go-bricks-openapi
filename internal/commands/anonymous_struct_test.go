package commands

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/specvalidate"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// Issue 118, end to end: an anonymous struct payload is typed, and
// an anonymous struct request warns.

// anonPayloadModSrc is a module whose request-less routes return anonymous
// struct payloads, and whose /create route takes an anonymous struct
// request.
const anonPayloadModSrc = `package svc

import (
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

var _ time.Time

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
ROUTES}

func (m *Module) item(ctx server.HandlerContext) (server.Result[struct{ A int ` + "`json:\"a\"`" + ` }], server.IAPIError) {
	return server.Result[struct{ A int ` + "`json:\"a\"`" + ` }]{}, nil
}

func (m *Module) list(ctx server.HandlerContext) ([]struct{ B string ` + "`json:\"b\"`" + ` }, server.IAPIError) {
	return nil, nil
}

func (m *Module) stamp(ctx server.HandlerContext) (server.Result[struct {
	time.Time
	Z int
}], server.IAPIError) {
	return server.Result[struct {
		time.Time
		Z int
	}]{}, nil
}

func (m *Module) create(req struct{ ID int64 ` + "`param:\"id\"`" + ` }, ctx server.HandlerContext) (server.Result[int64], server.IAPIError) {
	return server.Result[int64]{}, nil
}
`

// writeAnonPayloadProject writes anonPayloadModSrc registering routes as a
// project and returns its root.
func writeAnonPayloadProject(t *testing.T, routes string) string {
	t.Helper()
	src := strings.Replace(anonPayloadModSrc, "ROUTES", routes, 1)
	return writeProject(t, "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks "+minGoBricksVer+"\n", src)
}

// TestRunGenerateAnonymousPayloadsPassStrict pins that request-less routes
// returning anonymous struct payloads are typed, so no untyped-route warning
// fires and generate --strict --validate succeeds, documenting each payload
// inline.
func TestRunGenerateAnonymousPayloadsPassStrict(t *testing.T) {
	dir := writeAnonPayloadProject(t, "\tserver.GET(hr, r, \"/item\", m.item)\n\tserver.GET(hr, r, \"/list\", m.list)\n")
	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true, Validate: true})
	})
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 0", warningsSummary(t, stdout))

	content, err := os.ReadFile(out)
	require.NoError(t, err)
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	data := func(path string) map[string]any {
		return digMap(t, spec.Paths, path, "get", "responses", "200", "content", "application/json", "schema", "properties", "data")
	}
	item := data("/item")
	assert.Equal(t, "object", item["type"])
	assert.NotContains(t, item, "description", "an inline object is not the untyped fallback")
	assert.Contains(t, digMap(t, item, "properties"), "a")
	list := data("/list")
	assert.Equal(t, "array", list["type"])
	assert.Contains(t, digMap(t, list, "items", "properties"), "b")
}

// TestRunGenerateAnonymousRequestFailsStrict pins that an anonymous struct
// request raises one warning, so generate --strict fails with no artifact,
// while a non-strict run emits a valid document with no requestBody and no
// component for the request.
func TestRunGenerateAnonymousRequestFailsStrict(t *testing.T) {
	dir := writeNonStructRequestProject(t, "struct{ Street string `json:\"street\"` }", "Address")
	a := analyzer.New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	require.Len(t, a.Warnings(t.Context()), 1)
	assert.True(t, strings.HasPrefix(a.Warnings(t.Context())[0], "request type struct{Street string} is an anonymous struct"),
		a.Warnings(t.Context())[0])

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	})
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 1", warningsSummary(t, stdout))
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NoError(t, specvalidate.Validate(t.Context(), content))
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	assert.NotContains(t, digMap(t, spec.Paths, "/items", "post"), "requestBody")
	schemas, ok := spec.Components["schemas"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"Address", "ErrorResponse"}, slices.Sorted(maps.Keys(schemas)))

	strictOut := filepath.Join(t.TempDir(), outputFileName)
	testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: strictOut, Strict: true})
	})
	require.Error(t, runErr, "--strict must fail on an anonymous struct request")
	assert.NoFileExists(t, strictOut)
}

// TestRunEmptyAnonymousRequestPassesStrict pins that a struct{} request,
// go-bricks' request type for a route that takes no input, raises no
// warning: generate --strict --validate succeeds with no requestBody, and
// doctor prints "All checks passed".
func TestRunEmptyAnonymousRequestPassesStrict(t *testing.T) {
	dir := writeNonStructRequestProject(t, "struct{}", "Address")
	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true, Validate: true})
	})
	require.NoError(t, runErr, stdout)
	assert.Equal(t, "Warnings: 0", warningsSummary(t, stdout))
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	assert.NotContains(t, digMap(t, spec.Paths, "/items", "post"), "requestBody")

	doctorOut := testutil.CaptureStdout(t, func() {
		runErr = runDoctor(t.Context(), &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion})
	})
	require.NoError(t, runErr, doctorOut)
	assert.Contains(t, doctorOut, "All checks passed")
	assert.NotContains(t, doctorOut, "anonymous struct")
}

// TestDoctorClassifiesAnonymousStructs pins doctor's verdict: an anonymous
// struct payload is a typed response, one that embeds a Marshaler type other
// than a text one is not, and an anonymous struct request is an untyped
// request.
func TestDoctorClassifiesAnonymousStructs(t *testing.T) {
	dir := writeAnonPayloadProject(t, "\tserver.GET(hr, r, \"/item\", m.item)\n\tserver.GET(hr, r, \"/list\", m.list)\n"+
		"\tserver.GET(hr, r, \"/stamp\", m.stamp)\n\tserver.POST(hr, r, \"/create/:id\", m.create)\n")
	project, err := analyzer.New(dir).AnalyzeProject()
	require.NoError(t, err)
	stats := calculateProjectStats(project)
	assert.Equal(t, 3, stats.TypedResponseRoutes, "item, list and create's int64")
	assert.Equal(t, 0, stats.TypedRequestRoutes)
	untyped := append([]string{}, stats.UntypedRoutes...)
	sort.Strings(untyped)
	assert.Equal(t, []string{"stamp"}, untyped)
}
