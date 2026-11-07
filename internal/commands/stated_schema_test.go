package commands

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// statedSchemaFixture is the golden fixture whose Marshaler types carry
// //openapi:schema directives (#117).
var statedSchemaFixture = filepath.Join("..", "spectest", "testdata", "stated_schema")

// statedModSrc is a module whose Money Marshaler type is a field, a payload
// and a request body; DIRECTIVE is its doc comment's directive line.
const statedModSrc = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Money is written as a decimal string.
//
DIRECTIVE
type Money struct{ Units int64 }

func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

type Wallet struct {
	Balance Money ` + "`json:\"balance\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/wallet", m.wallet)
	server.POST(hr, r, "/pay", m.pay)
}

func (m *Module) wallet(ctx server.HandlerContext) (server.Result[Wallet], server.IAPIError) {
	return server.Result[Wallet]{}, nil
}

func (m *Module) pay(req Money, ctx server.HandlerContext) (server.Result[Money], server.IAPIError) {
	return server.Result[Money]{}, nil
}
`

// TestRunGenerateStatedSchemaFixtureStrictClean pins that the stated_schema
// fixture passes --strict --validate: no Marshaler warning is raised for a
// type with a Stated schema.
func TestRunGenerateStatedSchemaFixtureStrictClean(t *testing.T) {
	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{
			ProjectRoot: statedSchemaFixture, OutputFile: out, Strict: true, Validate: true,
		})
	})
	require.NoError(t, runErr, stdout)
	assert.Contains(t, stdout, "Warnings: 0\n")
}

// TestRunGenerateStrictNeedsTheStatedSchema pins that --strict passes with
// the directive and fails without it, on the existing Marshaler warnings of
// the field, the payload and the request body.
func TestRunGenerateStrictNeedsTheStatedSchema(t *testing.T) {
	goMod := "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks " + minGoBricksVer + "\n"
	run := func(directive string) (string, error) {
		dir := writeProject(t, goMod, strings.Replace(statedModSrc, "DIRECTIVE", directive, 1))
		out := filepath.Join(t.TempDir(), outputFileName)
		var runErr error
		stdout := testutil.CaptureStdout(t, func() {
			runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true})
		})
		return stdout, runErr
	}

	stdout, err := run("//openapi:schema string decimal")
	require.NoError(t, err, stdout)
	assert.Contains(t, stdout, "Warnings: 0\n")

	stdout, err = run("// (no directive)")
	require.Error(t, err)
	assert.Contains(t, stdout, "Warnings: 4\n",
		"the field's, the payload's and the request's Marshaler warnings, and the untyped pay route")
}

// TestDoctorCountsStatedPayloadsTyped pins that doctor counts a route whose
// Result[T] payload or request body has a Stated schema as typed.
func TestDoctorCountsStatedPayloadsTyped(t *testing.T) {
	project, err := analyzer.New(statedSchemaFixture).AnalyzeProject()
	require.NoError(t, err)
	stats := calculateProjectStats(project)
	assert.Empty(t, stats.UntypedRoutes)
	assert.Equal(t, 7, stats.TypedRoutes)
	assert.Equal(t, 7, stats.TypedResponseRoutes)
}
