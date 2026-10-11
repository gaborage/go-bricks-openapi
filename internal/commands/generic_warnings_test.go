package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// genericPayloadModSrc is a module whose one route has a typed request and
// returns PAYLOAD, a generic instantiation. Before issue 138 it raised no
// warning at all, so --strict passed.
const genericPayloadModSrc = `package svc

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type PageReq struct {
	ID string ` + "`param:\"id\"`" + `
}

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/pages/:id", m.page, server.WithTags("svc"))
}

func (m *Module) page(req PageReq, ctx server.HandlerContext) (PAYLOAD, server.IAPIError) {
	var v PAYLOAD
	return v, nil
}
`

// TestRunGenerateGenericPayloadFailsStrict pins issue 138 through the CLI
// path: a typed-request route returning a generic payload raises the one
// generic payload warning, so --strict fails with no artifact, while a
// non-strict --validate run still emits a valid document.
func TestRunGenerateGenericPayloadFailsStrict(t *testing.T) {
	goMod := "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks " + minGoBricksVer + "\n"
	for _, payload := range []string{"server.Result[Page[Address]]", "Page[Address]", "server.Result[map[string]Page[Address]]"} {
		t.Run(payload, func(t *testing.T) {
			dir := writeProject(t, goMod, strings.ReplaceAll(genericPayloadModSrc, "PAYLOAD", payload))

			out := filepath.Join(t.TempDir(), outputFileName)
			var runErr error
			stdout := testutil.CaptureStdout(t, func() {
				runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true})
			})
			require.Error(t, runErr, "a generic payload must fail --strict")
			assert.Contains(t, stdout, "Warnings: 1\n", "the payload's one warning is the only diagnostic")
			_, statErr := os.Stat(out)
			assert.True(t, os.IsNotExist(statErr), "strict failure must not leave an artifact")

			out = filepath.Join(t.TempDir(), outputFileName)
			testutil.CaptureStdout(t, func() {
				runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Validate: true})
			})
			require.NoError(t, runErr)
		})
	}
}
