package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// wellKnownNetModSrc uses os.FileMode, fs.FileMode, net.IP and the netip
// types in body fields, FileMode parameters and payloads: everything this
// tool documents from their encoding/json wire form, with no warning.
const wellKnownNetModSrc = `package svc

import (
	"io/fs"
	"net"
	"net/netip"
	"os"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Perm os.FileMode

type IPA = net.IP

type MyIP net.IP

type Host struct {
	IP   net.IP      ` + "`json:\"ip\" validate:\"required,ip\"`" + `
	Mode os.FileMode ` + "`json:\"mode\" validate:\"max=511\"`" + `
	Mine MyIP        ` + "`json:\"mine\"`" + `
}

type HostReq struct {
	Mode os.FileMode ` + "`query:\"mode\"`" + `
	Perm Perm        ` + "`header:\"X-Perm\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/host", m.host, server.WithTags("svc"))
	server.GET(hr, r, "/mode", m.mode, server.WithTags("svc"))
	server.GET(hr, r, "/perm", m.perm, server.WithTags("svc"))
	server.GET(hr, r, "/modes", m.modes, server.WithTags("svc"))
	server.GET(hr, r, "/ip", m.ip, server.WithTags("svc"))
	server.GET(hr, r, "/addr", m.addr, server.WithTags("svc"))
	server.GET(hr, r, "/prefixes", m.prefixes, server.WithTags("svc"))
	server.GET(hr, r, "/port", m.port, server.WithTags("svc"))
	server.GET(hr, r, "/ipa", m.ipa, server.WithTags("svc"))
	server.GET(hr, r, "/mine", m.mine, server.WithTags("svc"))
}

func (m *Module) host(req HostReq, ctx server.HandlerContext) (server.Result[Host], server.IAPIError) {
	return server.Result[Host]{}, nil
}

func (m *Module) mode(ctx server.HandlerContext) (server.Result[*os.FileMode], server.IAPIError) {
	return server.Result[*os.FileMode]{}, nil
}

func (m *Module) perm(ctx server.HandlerContext) (server.Result[Perm], server.IAPIError) {
	return server.Result[Perm]{}, nil
}

func (m *Module) modes(ctx server.HandlerContext) (server.Result[map[string]fs.FileMode], server.IAPIError) {
	return server.Result[map[string]fs.FileMode]{}, nil
}

func (m *Module) ip(ctx server.HandlerContext) (server.Result[net.IP], server.IAPIError) {
	return server.Result[net.IP]{}, nil
}

func (m *Module) addr(ctx server.HandlerContext) (server.Result[*netip.Addr], server.IAPIError) {
	return server.Result[*netip.Addr]{}, nil
}

func (m *Module) prefixes(ctx server.HandlerContext) (server.Result[[]netip.Prefix], server.IAPIError) {
	return server.Result[[]netip.Prefix]{}, nil
}

func (m *Module) port(ctx server.HandlerContext) (server.ResultWithMeta[netip.AddrPort], server.IAPIError) {
	return server.ResultWithMeta[netip.AddrPort]{}, nil
}

func (m *Module) ipa(ctx server.HandlerContext) (server.Result[IPA], server.IAPIError) {
	return server.Result[IPA]{}, nil
}

func (m *Module) mine(ctx server.HandlerContext) (server.Result[MyIP], server.IAPIError) {
	return server.Result[MyIP]{}, nil
}
`

// TestWellKnownNetAndFileModeAreTyped pins issue 139 at the CLI: FileMode
// fields, parameters and payloads are a uint32, the text types and an alias
// of one a string, and a defined type over net.IP a byte string, with no
// warning, so generate --strict --validate succeeds and doctor counts every
// route as typed.
func TestWellKnownNetAndFileModeAreTyped(t *testing.T) {
	goMod := "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks " + minGoBricksVer + "\n"
	dir := writeProject(t, goMod, wellKnownNetModSrc)

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Validate: true, Strict: true})
	})
	require.NoError(t, runErr, stdout)
	assert.Contains(t, stdout, "Warnings: 0\n")
	content, err := os.ReadFile(out)
	require.NoError(t, err)
	var spec OpenAPISpec
	require.NoError(t, yaml.Unmarshal(content, &spec))

	data := func(path string) map[string]any {
		return digMap(t, spec.Paths, path, "get", "responses", "200", "content", "application/json", "schema", "properties", "data")
	}
	mode := map[string]any{"type": "integer", "format": "int64", "minimum": 0}
	str := map[string]any{"type": "string"}
	assert.Equal(t, mode, data("/mode"), "a payload root is never nullable")
	assert.Equal(t, mode, data("/perm"))
	assert.Equal(t, map[string]any{"type": "object", "additionalProperties": mode}, data("/modes"))
	for _, path := range []string{"/ip", "/addr", "/port", "/ipa"} {
		assert.Equal(t, str, data(path), path)
	}
	assert.Equal(t, map[string]any{"type": "array", "items": str}, data("/prefixes"))
	assert.Equal(t, map[string]any{"type": "string", "format": "byte"}, data("/mine"))

	host := digMap(t, spec.Components, "schemas", "Host", "properties")
	assert.Equal(t, str, host["ip"])
	assert.Equal(t, map[string]any{"type": "integer", "format": "int64", "minimum": 0, "maximum": 511}, host["mode"])
	assert.Equal(t, map[string]any{"type": "string", "format": "byte"}, host["mine"])

	params, ok := digMap(t, spec.Paths, "/host", "get")["parameters"].([]any)
	require.True(t, ok, "the /host operation has parameters")
	schemas := map[string]any{}
	for _, p := range params {
		if pm, isMap := p.(map[string]any); isMap {
			name, _ := pm["name"].(string)
			schemas[name] = pm["schema"]
		}
	}
	assert.Equal(t, mode, schemas["mode"], "a FileMode query parameter binds by its uint32 kind")
	assert.Equal(t, mode, schemas["X-Perm"], "so does a Perm header")

	doctorOut := testutil.CaptureStdout(t, func() {
		runErr = runDoctor(t.Context(), &DoctorOptions{ProjectRoot: dir, GoVersion: minGoVersion})
	})
	require.NoError(t, runErr, doctorOut)
	assert.Contains(t, doctorOut, "Typed routes: 10/10")
	assert.NotContains(t, doctorOut, "Routes without type information")
}
