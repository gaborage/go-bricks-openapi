package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// payloadFallbackModSrc is a module whose one route returns
// server.Result[PAYLOAD]; DECLS are extra type declarations. The route has a
// typed request, so no untyped-route warning fires: only the payload's own
// warning can fail --strict.
const payloadFallbackModSrc = `package svc

import (
	"time"
	t "time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type PriceReq struct {
	ID string ` + "`param:\"id\"`" + `
}

var _ decimal.Decimal
var _ t.Time
var _ time.Time

DECLS

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/price/:id", m.price, server.WithTags("svc"))
}

func (m *Module) price(req PriceReq, ctx server.HandlerContext) (server.Result[PAYLOAD], server.IAPIError) {
	return server.Result[PAYLOAD]{}, nil
}
`

// TestRunGenerateNamedPayloadsFixtureStrictClean pins that the named_payloads
// fixture, every resolved payload row, passes --strict --validate through the
// CLI path.
func TestRunGenerateNamedPayloadsFixtureStrictClean(t *testing.T) {
	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{
			ProjectRoot: filepath.Join("..", "spectest", "testdata", "named_payloads"),
			OutputFile:  out, Strict: true, Validate: true,
		})
	})
	require.NoError(t, runErr, stdout)
	assert.Contains(t, stdout, "Warnings: 0\n")
}

// TestRunGeneratePayloadFallbacksFailStrict pins that each payload ending in
// a fallback raises exactly one warning, so --strict fails with no artifact,
// while a non-strict --validate run still emits a valid document. uintptr and
// []uintptr were silent before #110.
func TestRunGeneratePayloadFallbacksFailStrict(t *testing.T) {
	goMod := "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks " + minGoBricksVer + "\n"
	tier := "type Tier int\nfunc (t Tier) MarshalText() ([]byte, error) { return nil, nil }"
	cases := []struct{ decls, payload string }{
		{tier, "Tier"},
		{tier, "*Tier"},
		{"type PTier int\nfunc (t *PTier) MarshalText() ([]byte, error) { return nil, nil }", "PTier"},
		{tier, "[]Tier"},
		{"type FlagV byte\nfunc (f FlagV) MarshalJSON() ([]byte, error) { return nil, nil }", "[]FlagV"},
		{tier, "map[string]Tier"},
		{"type Tree map[string]Tree", "Tree"},
		{"type Addr uintptr", "Addr"},
		{"", "map[string]uintptr"},
		{"", "uintptr"},
		{"", "[]uintptr"},
		{"", "decimal.Decimal"},
		{"", "t.Time"},
		{"", "*Missing"},
		{"", "map[string]decimal.Decimal"},
		{"type StampD time.Time", "StampD"},
		{"type StampD time.Time", "[]StampD"},
		{"type Cx complex128", "Cx"},
		{"", "complex128"},
	}
	for _, c := range cases {
		t.Run(c.payload, func(t *testing.T) {
			src := strings.NewReplacer("DECLS", c.decls, "PAYLOAD", c.payload).Replace(payloadFallbackModSrc)
			dir := writeProject(t, goMod, src)

			out := filepath.Join(t.TempDir(), outputFileName)
			var runErr error
			stdout := testutil.CaptureStdout(t, func() {
				runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true})
			})
			require.Error(t, runErr, "a fallback payload must fail --strict")
			assert.Contains(t, stdout, "Warnings: 1\n", "the payload's one warning is the only diagnostic")
			_, statErr := os.Stat(out)
			assert.True(t, os.IsNotExist(statErr), "strict failure must not leave an artifact")

			out = filepath.Join(t.TempDir(), outputFileName)
			testutil.CaptureStdout(t, func() {
				runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Validate: true})
			})
			require.NoError(t, runErr, "a non-strict run must emit a valid document")
		})
	}
}

// doctorPayloadsModSrc declares the payload types TestDoctorClassifiesPayloads
// classifies; its routes (ROUTES) and handlers (HANDLERS) take no request, so
// a route is typed only if its payload is.
const doctorPayloadsModSrc = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

var _ decimal.Decimal

type Cents int64
type UserList []User
type AnyD any
type Tier int

func (t Tier) MarshalText() ([]byte, error) { return nil, nil }

type Tree map[string]Tree
type Fn func()

type User struct {
	ID int64 ` + "`json:\"id\"`" + `
}

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
ROUTES}

HANDLERS`

// TestDoctorClassifiesPayloads pins doctor's verdict on payloads (and so
// generate's route count): a payload is typed exactly when its schema has no
// fallback leaf.
func TestDoctorClassifiesPayloads(t *testing.T) {
	typed := []string{"Cents", "UserList", "map[string]int64", "AnyD", "*[]string"}
	untyped := []string{"Tier", "Tree", "uintptr", "[]uintptr", "decimal.Decimal", "Fn", "complex128", "**Address", "map[string]Fn"}
	var routes, handlers strings.Builder
	names := map[string]string{}
	for i, payload := range append(append([]string{}, typed...), untyped...) {
		name := fmt.Sprintf("h%d", i)
		names[payload] = name
		fmt.Fprintf(&routes, "\tserver.GET(hr, r, \"/p%d\", m.%s)\n", i, name)
		fmt.Fprintf(&handlers, "func (m *Module) %s(ctx server.HandlerContext) (server.Result[%s], server.IAPIError) {\n\treturn server.Result[%s]{}, nil\n}\n\n",
			name, payload, payload)
	}
	src := strings.NewReplacer("ROUTES", routes.String(), "HANDLERS", handlers.String()).Replace(doctorPayloadsModSrc)
	dir := writeProject(t, "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks "+minGoBricksVer+"\n", src)
	project, err := analyzer.New(dir).AnalyzeProject()
	require.NoError(t, err)
	stats := calculateProjectStats(project)

	wantUntyped := make([]string, 0, len(untyped))
	for _, payload := range untyped {
		wantUntyped = append(wantUntyped, names[payload])
	}
	got := append([]string{}, stats.UntypedRoutes...)
	sort.Strings(got)
	sort.Strings(wantUntyped)
	assert.Equal(t, wantUntyped, got)
	assert.Equal(t, len(typed), stats.TypedResponseRoutes)
}
