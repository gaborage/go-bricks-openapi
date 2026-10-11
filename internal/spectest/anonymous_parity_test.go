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

// anonParityCorpus is the field lists TestAnonymousStructMatchesNamedStruct
// writes both as a named struct and as an anonymous one. Backquotes stand in
// for struct-tag backticks.
//
// Pointer rows stay out on purpose: *Named emits allOf: [$ref] plus nullable
// (refProperty), while *struct{…} puts nullable on the inline object itself;
// both are correct. So do doc: tags on the holder's fields: a $ref carries no
// description, while an inline object does.
var anonParityCorpus = []string{
	"A int ´json:\"a\"´; B string ´json:\"b,omitempty\"´; C int ´json:\"-\"´; d int; E bool",
	"Home; Z int ´json:\"z\"´",
	"Home ´json:\"home\"´; Z int ´json:\"z\"´",
	"Cents; Z int ´json:\"z\"´",
	"Home; City string ´json:\"city\"´",
	"X int ´json:\"x\" validate:\"required,min=1,max=9\"´; M string ´json:\"m\" validate:\"email\"´; S []string ´json:\"s\" validate:\"min=1,dive,max=3\"´",
	"ID int64 ´param:\"id\"´; Q string ´query:\"q\"´; X int ´json:\"x\"´",
	"time.Time; Z int",
	"uuid.UUID",
	"T time.Time ´json:\"t\"´; U uuid.UUID ´json:\"u\"´; P *int ´json:\"p\"´",
	"Tags Tags ´json:\"tags\"´; C Cents ´json:\"c\" validate:\"max=5\"´",
	"O Office ´json:\"o\"´; Os []*Office ´json:\"os\"´",
	"In struct{ Y int ´json:\"y\"´ } ´json:\"in\"´",
	"",
}

// writeAnonParityProject writes a module whose Holder holds, per corpus row
// i, a field Li of the row's anonymous struct and a field Ni of the named
// struct Namedi declaring the same fields.
func writeAnonParityProject(t *testing.T, dir string) {
	t.Helper()
	var named, holder strings.Builder
	for i, row := range anonParityCorpus {
		body := strings.ReplaceAll(row, "´", "`")
		fmt.Fprintf(&named, "type Named%d struct{ %s }\n", i, body)
		fmt.Fprintf(&holder, "\tL%d struct{ %s } `json:\"l%d\"`\n\tN%d Named%d `json:\"n%d\"`\n", i, body, i, i, i, i)
	}
	module := `package anonparity

import (
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
)

var _ time.Time
var _ uuid.UUID

type Module struct{}

func (m *Module) Name() string                    { return "anonparity" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Home struct{ City string ` + "`json:\"city\"`" + ` }
type Office struct{ Floor int ` + "`json:\"floor\"`" + ` }
type Cents int64
type Tags []string

` + named.String() + `
type Holder struct {
` + holder.String() + `}

func (m *Module) get(ctx server.HandlerContext) (server.Result[Holder], server.IAPIError) {
	return server.Result[Holder]{}, nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/holder", m.get)
}
`
	files := map[string]string{
		"go.mod":    "module github.com/example/anonparity\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks v0.53.0\n",
		"module.go": module,
	}
	for name, src := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600))
	}
}

// TestAnonymousStructMatchesNamedStruct is issue 118's "must match a named
// struct" oracle: an anonymous struct field emits exactly the schema of a
// named struct of the same fields, its component's when that is a $ref.
func TestAnonymousStructMatchesNamedStruct(t *testing.T) {
	dir := t.TempDir()
	writeAnonParityProject(t, dir)

	spec, err := Generate(t.Context(), dir)
	require.NoError(t, err)
	require.NoError(t, Validate(t.Context(), []byte(spec)))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	schemas, ok := dig(t, doc, "components", "schemas").(map[string]any)
	require.True(t, ok)
	props, ok := dig(t, schemas, "Holder", "properties").(map[string]any)
	require.True(t, ok)

	for i, row := range anonParityCorpus {
		want, ok := props[fmt.Sprintf("n%d", i)].(map[string]any)
		require.True(t, ok, row)
		if ref, isRef := want["$ref"].(string); isRef {
			want, ok = schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
			require.True(t, ok, "%s: %s resolves", row, ref)
		}
		assert.Equal(t, want, props[fmt.Sprintf("l%d", i)], "row %d: struct{ %s }", i, row)
	}
}
