package spectest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// anonRequestParityCorpus is the field lists
// TestAnonymousRequestMatchesNamedRequest writes both as a named request
// struct and as an anonymous one; ptr writes the request behind a pointer.
// Backquotes stand in for struct-tag backticks.
var anonRequestParityCorpus = []struct {
	fields string
	ptr    bool
}{
	{"ID int64 ´param:\"id\"´; Q int ´query:\"q\"´; H string ´header:\"X-Trace\"´; Body string ´json:\"body\" validate:\"required\"´", false},
	{"ID int64 ´param:\"id\"´; Q int ´query:\"q\"´", false},
	{"ID int64 ´param:\"id\"´; Body string ´json:\"body\"´", true},
	{"Home; Z int ´json:\"z\"´", false},
	{"Paging; Z int ´json:\"z\"´", false},
	{"X int ´json:\"x\" validate:\"required,min=1,max=9\"´; M string ´json:\"m\" validate:\"email\"´; S []string ´json:\"s\" validate:\"min=1,dive,max=3\"´", false},
	{"O Office ´json:\"o\"´; Os []*Office ´json:\"os\"´; Q string ´query:\"q\" validate:\"max=5\"´", true},
	{"time.Time; ID int64 ´param:\"id\"´", false},
	{"time.Time; ID int64 ´param:\"id\"´; Z int ´json:\"z\"´", true},
}

// writeAnonRequestParityProject writes a module with, per corpus row i, a
// route /ni/:id taking the named request Reqi and a route /li/:id taking the
// row's anonymous struct.
func writeAnonRequestParityProject(t *testing.T, dir string) {
	t.Helper()
	var decls, routes strings.Builder
	for i, row := range anonRequestParityCorpus {
		body := strings.ReplaceAll(row.fields, "´", "`")
		ptr := ""
		if row.ptr {
			ptr = "*"
		}
		fmt.Fprintf(&decls, "type Req%d struct{ %s }\n\n", i, body)
		fmt.Fprintf(&decls, "func (m *Module) n%d(req %sReq%d, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {\n\treturn server.Result[Out]{}, nil\n}\n\n", i, ptr, i)
		fmt.Fprintf(&decls, "func (m *Module) l%d(req %sstruct{ %s }, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {\n\treturn server.Result[Out]{}, nil\n}\n\n", i, ptr, body)
		fmt.Fprintf(&routes, "\tserver.POST(hr, r, \"/n%d/:id\", m.n%d)\n\tserver.POST(hr, r, \"/l%d/:id\", m.l%d)\n", i, i, i, i)
	}
	module := `package anonreqparity

import (
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "anonreqparity" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Out struct{ OK bool ` + "`json:\"ok\"`" + ` }
type Home struct{ City string ` + "`json:\"city\"`" + ` }
type Paging struct {
	Page int ` + "`query:\"page\"`" + `
	Size int ` + "`query:\"size\" validate:\"max=50\"`" + `
}
type Office struct{ Floor int ` + "`json:\"floor\"`" + ` }

var _ time.Time

` + decls.String() + `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
` + routes.String() + `}
`
	files := map[string]string{
		"go.mod":    "module github.com/example/anonreqparity\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks v0.53.0\n",
		"module.go": module,
	}
	for name, src := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600))
	}
}

// TestAnonymousRequestMatchesNamedRequest is issue 171's request-side parity
// oracle: an anonymous struct request emits exactly the parameters of a
// named request of the same fields, and an inline requestBody schema equal
// to the named one's component, or no requestBody when the named one has
// none.
func TestAnonymousRequestMatchesNamedRequest(t *testing.T) {
	dir := t.TempDir()
	writeAnonRequestParityProject(t, dir)

	spec, err := Generate(t.Context(), dir)
	require.NoError(t, err)
	require.NoError(t, Validate(t.Context(), []byte(spec)))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	schemas, ok := dig(t, doc, "components", "schemas").(map[string]any)
	require.True(t, ok)

	for i, row := range anonRequestParityCorpus {
		named, ok := dig(t, doc, "paths", fmt.Sprintf("/n%d/{id}", i), "post").(map[string]any)
		require.True(t, ok, row.fields)
		anon, ok := dig(t, doc, "paths", fmt.Sprintf("/l%d/{id}", i), "post").(map[string]any)
		require.True(t, ok, row.fields)

		assert.Equal(t, named["parameters"], anon["parameters"], "row %d parameters: %s", i, row.fields)
		_, namedBody := named["requestBody"]
		_, anonBody := anon["requestBody"]
		require.Equal(t, namedBody, anonBody, "row %d has a requestBody on one side only: %s", i, row.fields)
		if !namedBody {
			continue
		}
		want, ok := dig(t, named, "requestBody", "content", "application/json", "schema").(map[string]any)
		require.True(t, ok, row.fields)
		if ref, isRef := want["$ref"].(string); isRef {
			want, ok = schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
			require.True(t, ok, "%s: %s resolves", row.fields, ref)
		}
		assert.Equal(t, want, dig(t, anon, "requestBody", "content", "application/json", "schema"), "row %d body: %s", i, row.fields)
	}
}
