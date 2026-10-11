package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildTaggedRefClashByDeclaringDir pins that build-tagged
// declarations clash on a ref name only when it reaches structs declared in
// different directories, wherever the name was written. One qualifier bound
// to two packages (type W []item.Item, item being dw in one file and lx in
// the other) clashes as two qualifiers do, so W is an untyped object with the
// disagree warning instead of one variant's Item picked by file order. One
// struct reached through two packages' own declarations (wa.Wrap, wb.Wrap,
// both []c.Item) is one struct, so the copies merge. A local alias or defined
// type over the qualified struct (type Item = item.Item, type W []Item) is
// judged by the struct it reaches, not by its own directory, so it clashes
// and merges the same way.
func TestBuildTaggedRefClashByDeclaringDir(t *testing.T) {
	variant := func(constraint, importPath, decl string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\nimport " + importPath + "\n\n" + decl + "\n"
	}
	item := func(name, field string) string {
		return "package " + name + "\n\ntype Item struct {\n\t" + field + "\n}\n"
	}
	wrap := func(name string) string {
		return "package " + name + "\n\nimport \"github.com/example/app/c\"\n\ntype Wrap []c.Item\n"
	}
	const (
		itemDW = `item "github.com/example/app/dw"`
		itemLX = `item "github.com/example/app/lx"`
		listW  = "type W []item.Item"
		aliasW = "type Item = item.Item\n\ntype W []Item"
		namedW = "type Item item.Item\n\ntype W []Item"
	)
	cases := []struct {
		name, darwinImport, darwinDecl, linuxImport, linuxDecl, want string
		warned                                                       bool
	}{
		{"one qualifier, two packages", itemDW, listW, itemLX, listW, "W", true},
		{"one qualifier, one package", itemDW, listW, itemDW, listW, "[]$Item", false},
		{"one struct through two packages", `"github.com/example/app/wa"`, "type W wa.Wrap", `"github.com/example/app/wb"`, "type W wb.Wrap", "[]$Item", false},
		{"local alias, two packages", itemDW, aliasW, itemLX, aliasW, "W", true},
		{"local alias, one package", itemDW, aliasW, itemDW, aliasW, "[]$Item", false},
		{"local defined type, two packages", itemDW, namedW, itemLX, namedW, "W", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                            resolveGoMod,
				filepath.Join("c", "c.go"):          item("c", "ID int `json:\"id\"`"),
				filepath.Join("dw", "dw.go"):        item("dw", "Darwin int `json:\"darwin\"`"),
				filepath.Join("lx", "lx.go"):        item("lx", "Linux string `json:\"linux\"`"),
				filepath.Join("wa", "wa.go"):        wrap("wa"),
				filepath.Join("wb", "wb.go"):        wrap("wb"),
				filepath.Join("mod", "module.go"):   rowsModule("", "", "\tW W `json:\"w\"`\n"),
				filepath.Join("mod", "w_darwin.go"): variant("darwin", c.darwinImport, c.darwinDecl),
				filepath.Join("mod", "w_linux.go"):  variant("linux", c.linuxImport, c.linuxDecl),
			})
			assert.Equal(t, c.want, renders(t, a.typeRegistry["Rows"])["w"])
			w := fieldWarnings(a, "W")
			if !c.warned {
				assert.Empty(t, w)
				return
			}
			if assert.Len(t, w, 1) {
				assert.Contains(t, w[0], "has type W: W's build-tagged declarations disagree on shape")
			}
		})
	}
}

// delegateModuleFile declares a module whose handler field h is a
// *handlers.H, with RegisterRoutes left to another file.
const delegateModuleFile = `package mod

import (
	"github.com/example/app/handlers"
	"github.com/gaborage/go-bricks/app"
)

type Module struct{ h *handlers.H }

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { m.h = &handlers.H{}; return nil }
func (m *Module) Shutdown() error                 { return nil }
`

// delegateRoutesFile is a RegisterRoutes delegating to m.h, in a file with
// the extra imports imports and declarations decls.
func delegateRoutesFile(imports, decls string) string {
	return "package mod\n\nimport (\n\t\"github.com/gaborage/go-bricks/server\"\n" + imports + ")\n\n" + decls +
		"func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n\tm.h.RegisterRoutes(hr, r)\n}\n"
}

// delegateHandlerPkg is a handlers package whose H registers GET path
// returning typeName, a struct of one string field with JSON name field.
func delegateHandlerPkg(path, typeName, field string) string {
	return `package handlers

import "github.com/gaborage/go-bricks/server"

type H struct{}

type ` + typeName + ` struct {
	` + exportName(field) + " string `json:\"" + field + "\"`" + `
}

func (h *H) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "` + path + `", h.get)
}

func (h *H) get(ctx server.HandlerContext) (server.Result[` + typeName + `], server.IAPIError) {
	return server.Result[` + typeName + `]{}, nil
}
`
}

// TestDelegateFieldResolvedInDeclaringFile pins that a module's handler
// field (Module{h *handlers.H} in module.go) resolves its qualifier through
// the imports of the file declaring the struct, not the file calling
// m.h.RegisterRoutes (routes.go): without the import there, the delegated
// routes are found and nothing warns; with another package imported there
// under the same name, its routes and schemas are not documented instead.
func TestDelegateFieldResolvedInDeclaringFile(t *testing.T) {
	cases := []struct {
		name, imports, decls string
	}{
		{"routes file does not import handlers", "", ""},
		{
			"routes file imports another handlers package",
			"\t\"github.com/example/app/legacy/handlers\"\n",
			"var _ handlers.Legacy\n\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, routes := analyzeProjectRoutes(t, map[string]string{
				"go.mod":                          resolveGoMod,
				filepath.Join("handlers", "h.go"): delegateHandlerPkg("/orders", "Order", "id"),
				filepath.Join("legacy", "handlers", "h.go"): delegateHandlerPkg("/legacy", "Legacy", "old"),
				filepath.Join("mod", "module.go"):           delegateModuleFile,
				filepath.Join("mod", "routes.go"):           delegateRoutesFile(c.imports, c.decls),
			})
			require.Len(t, routes, 1)
			assert.Equal(t, "/orders", routes[0].Path)
			require.NotNil(t, routes[0].Response)
			assert.Equal(t, "Order", routes[0].Response.Name)
			assert.Equal(t, map[string]string{"id": goTypeString}, renders(t, a.typeRegistry["Order"]))
			assert.NotContains(t, a.typeRegistry, "Legacy")
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}
