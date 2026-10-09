package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/testutil"
)

// qualifiedStrictModSrc is a module whose only non-local field types are
// named types from other packages of the module: b (also imported as money),
// b's import c, and kinds.
const qualifiedStrictModSrc = `package svc

import (
	"github.com/example/svc/b"
	"github.com/example/svc/kinds"
	money "github.com/example/svc/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "svc" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Code int32
type User struct {
	ID int ` + "`json:\"id\"`" + `
}
type Discount b.Cents
type Tags []b.Tags

type Body struct {
	b.Base
	Cents    b.Cents            ` + "`json:\"cents\" validate:\"min=1\"`" + `
	Money    money.Cents        ` + "`json:\"money\"`" + `
	Code     b.Code             ` + "`json:\"code\"`" + `
	Local    Code               ` + "`json:\"local\"`" + `
	Discount Discount           ` + "`json:\"discount\"`" + `
	ByKey    map[string]b.Cents ` + "`json:\"byKey\"`" + `
	Tags     Tags               ` + "`json:\"tags\"`" + `
	Labels   kinds.Labels       ` + "`json:\"labels\"`" + `
	Users    b.UserList         ` + "`json:\"users\"`" + `
	Owner    User               ` + "`json:\"owner\"`" + `
	Items    b.Items            ` + "`json:\"items\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/body", m.body, server.WithTags("svc"))
}

func (m *Module) body(ctx server.HandlerContext) (server.Result[Body], server.IAPIError) {
	return server.Result[Body]{}, nil
}
`

// TestRunGenerateQualifiedNamedTypesStrictClean pins that a project whose
// only non-local field types are resolvable named types from other packages
// of the module passes --strict --validate with no warning (#100).
func TestRunGenerateQualifiedNamedTypesStrictClean(t *testing.T) {
	goMod := "module github.com/example/svc\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks " + minGoBricksVer + "\n"
	dir := writeProject(t, goMod, qualifiedStrictModSrc)
	for rel, src := range map[string]string{
		filepath.Join("b", "b.go"): "package b\n\nimport \"github.com/example/svc/c\"\n\n" +
			"type Cents int64\ntype Code string\ntype Tags []string\ntype Grade c.Level\n" +
			"type Base struct {\n\tGrade Grade `json:\"grade\"`\n}\n" +
			"type User struct {\n\tName string `json:\"name\"`\n}\ntype UserList []User\ntype Items []c.Item\n",
		filepath.Join("c", "c.go"):         "package c\n\ntype Level int32\n\ntype Item struct {\n\tID int `json:\"id\"`\n}\n",
		filepath.Join("kinds", "kinds.go"): "package kinds\n\ntype Labels map[string]string\n",
	} {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o600))
	}

	out := filepath.Join(t.TempDir(), outputFileName)
	var runErr error
	stdout := testutil.CaptureStdout(t, func() {
		runErr = runGenerate(context.Background(), &GenerateOptions{ProjectRoot: dir, OutputFile: out, Strict: true, Validate: true})
	})
	require.NoError(t, runErr, stdout)
	assert.Contains(t, stdout, "Warnings: 0\n")
}
