package commands

import (
	"os"
	"path/filepath"
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

// bareReturnModSrc is a module whose one route, GET /payload, returns RESULT
// and takes no request, so the route is typed only by its response: doctor's
// classification of the payload decides whether generate's untyped-route
// warning fires. OPTS is spliced into the registration (WithRawResponse).
const bareReturnModSrc = `package svc

import (
	"encoding/json"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Address struct {
	Street string ` + "`json:\"street\"`" + `
}

type Cents int64

type Tags []string

var (
	_ json.RawMessage
	_ time.Time
	_ uuid.UUID
	_ decimal.Decimal
)

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/payload", m.get, server.WithTags("svc")OPTS)
}

func (m *Module) get(ctx server.HandlerContext) (RESULT, server.IAPIError) {
	return out, nil
}
`

// payloadOutcome is everything a route's payload type decides that a user of
// generate or doctor can observe.
type payloadOutcome struct {
	Schema        any      // the success body: the data property, or the whole body for a raw route
	Components    []string // emitted component schema names
	Warnings      []string // analyzer warnings
	Summary       string   // generate's "Warnings: N" line
	StrictFails   bool     // generate --strict exits non-zero
	Valid         bool     // the emitted document validates as OpenAPI 3.0
	TypedResponse bool     // doctor counts the response typed
}

// bareReturnOutcome runs analyze, generate and generate --strict over
// bareReturnModSrc with the handler's first result set to result.
func bareReturnOutcome(t *testing.T, result string, raw bool) payloadOutcome {
	t.Helper()
	opts := ""
	if raw {
		opts = ", server.WithRawResponse()"
	}
	src := strings.NewReplacer("RESULT", result, "OPTS", opts).Replace(bareReturnModSrc)
	dir := writeProject(t, "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks "+minGoBricksVer+"\n", src)

	a := analyzer.New(dir)
	project, err := a.AnalyzeProject()
	require.NoError(t, err)
	outcome := payloadOutcome{
		Warnings:      a.Warnings(t.Context()),
		TypedResponse: calculateProjectStats(project).TypedResponseRoutes == 1,
	}

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: out})
	})
	require.NoError(t, runErr, stdout)
	outcome.Summary = warningsSummary(t, stdout)

	content, err := os.ReadFile(out)
	require.NoError(t, err)
	outcome.Valid = specvalidate.Validate(t.Context(), content) == nil
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))
	body := digMap(t, spec.Paths, "/payload", "get", "responses", "200", "content", "application/json", "schema")
	outcome.Schema = body
	if !raw {
		outcome.Schema = digMap(t, body, "properties", propData)
	}
	if schemas, ok := spec.Components["schemas"].(map[string]any); ok {
		for name := range schemas {
			outcome.Components = append(outcome.Components, name)
		}
		sort.Strings(outcome.Components)
	}

	testutil.CaptureStdout(t, func() {
		runErr = runGenerate(t.Context(), &GenerateOptions{ProjectRoot: dir, OutputFile: filepath.Join(t.TempDir(), outputFileName), Strict: true})
	})
	outcome.StrictFails = runErr != nil
	return outcome
}

const propData = "data"

// warningsSummary returns generate's machine-checkable "Warnings: N" line.
func warningsSummary(t *testing.T, stdout string) string {
	t.Helper()
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "Warnings: ") {
			return line
		}
	}
	t.Fatalf("no Warnings summary line in:\n%s", stdout)
	return ""
}

// TestRunGenerateBareReturnMatchesResult pins that a bare (non-wrapper) handler
// return of type T is documented exactly like server.Result[T] — the same
// schema, components, warnings, --strict outcome and doctor classification —
// for every builtin, well-known, unresolvable, named non-struct, slice,
// interface and map payload, and for the project-struct controls. go-bricks
// sends both under the same data key, or as the whole body with
// WithRawResponse(). typed is doctor's expected verdict on the bare return.
func TestRunGenerateBareReturnMatchesResult(t *testing.T) {
	cases := []struct {
		payload string
		raw     bool
		typed   bool
	}{
		{payload: "int64", typed: true},
		{payload: "*int64", typed: true},
		{payload: "string", typed: true},
		{payload: "bool", typed: true},
		{payload: "float64", typed: true},
		{payload: "any", typed: true},
		{payload: "time.Time", typed: true},
		{payload: "uuid.UUID", typed: true},
		{payload: "json.RawMessage", typed: true},
		{payload: "decimal.Decimal", typed: false},
		{payload: "Cents", typed: true},
		{payload: "Tags", typed: true},
		{payload: "[]Address", typed: true},
		{payload: "[]string", typed: true},
		{payload: "interface{}", typed: true},
		{payload: "map[string]Cents", typed: true},
		{payload: "Address", typed: true},
		{payload: "*Address", typed: true},
		{payload: "int64", raw: true, typed: true},
		{payload: "decimal.Decimal", raw: true, typed: false},
		{payload: "Address", raw: true, typed: true},
	}
	for _, c := range cases {
		name := c.payload
		if c.raw {
			name += "/raw"
		}
		t.Run(name, func(t *testing.T) {
			bare := bareReturnOutcome(t, c.payload, c.raw)
			wrapped := bareReturnOutcome(t, "server.Result["+c.payload+"]", c.raw)
			assert.Equal(t, wrapped, bare, "a bare %s return must be documented like server.Result[%s]", c.payload, c.payload)
			assert.True(t, bare.Valid, "a bare %s return must emit a valid document (no dangling $ref)", c.payload)
			assert.Equal(t, c.typed, bare.TypedResponse, "doctor's verdict on a bare %s return", c.payload)
			assert.Equal(t, !c.typed, bare.StrictFails, "an untyped bare %s return warns, so --strict fails", c.payload)
		})
	}
}

// TestRunGenerateBareRawInt64IsTopLevelInteger pins the WithRawResponse() arm:
// a bare int64 is the whole response body, so the top-level schema is an int64
// integer rather than a $ref to a component that is never emitted.
func TestRunGenerateBareRawInt64IsTopLevelInteger(t *testing.T) {
	got := bareReturnOutcome(t, "int64", true)
	assert.Equal(t, map[string]any{"type": "integer", "format": "int64"}, got.Schema)
	assert.True(t, got.Valid)
	assert.Equal(t, "Warnings: 0", got.Summary)
}
