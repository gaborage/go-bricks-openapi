package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/generator"
	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// metaModHead is a module whose handlers cover every result form the
// `//openapi:meta` Directive (issue 69) decides on; a test appends the
// RegisterRoutes body (metaModule).
const metaModHead = `package mod

import (
	"net/http"

	"github.com/example/app/pagination"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Item struct {
	ID int64 ` + "`json:\"id\"`" + `
}

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}

type Sealed struct {
	_  struct{} ` + "`jose:\"decrypt=k1,verify=k2\"`" + `
	ID int64    ` + "`json:\"id\"`" + `
}

type PageMeta struct {
	Total  int64 ` + "`json:\"total\"`" + `
	Limit  int   ` + "`json:\"limit\"`" + `
	Offset int   ` + "`json:\"offset\"`" + `
}

type OtherMeta struct {
	Cursor string ` + "`json:\"cursor\"`" + `
}

type Meta = pagination.Meta

type NotStruct int

type MarshalMeta struct {
	N int ` + "`json:\"n\"`" + `
}

func (m MarshalMeta) MarshalJSON() ([]byte, error) { return nil, nil }

type StampMeta struct {
	Stamp string ` + "`json:\"timestamp\"`" + `
}

type traced struct {
	Trace string ` + "`json:\"traceId\"`" + `
}

type EmbedMeta struct {
	traced
	N int ` + "`json:\"n\"`" + `
}

type EmptyMeta struct{}

type PM = pagination.Meta

type DM pagination.Meta

type PA = PageMeta

type TagMeta struct {
	A string ` + "`json:\"a\" validate:\"required`" + `
}

type TagStampMeta struct {
	A string ` + "`json:\"a\" validate:\"required`" + `
	S string ` + "`json:\"timestamp\"`" + `
}

func (m *Module) withMeta(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) {
	return server.ResultWithMeta[[]Item]{}, nil
}

func (m *Module) ptrMeta(ctx server.HandlerContext) (*server.ResultWithMeta[Item], server.IAPIError) {
	return nil, nil
}

func (m *Module) ptrPtrMeta(ctx server.HandlerContext) (**server.ResultWithMeta[Item], server.IAPIError) {
	return nil, nil
}

func (m *Module) pageMeta(ctx server.HandlerContext) (server.ResultWithMeta[Page[Item]], server.IAPIError) {
	return server.ResultWithMeta[Page[Item]]{}, nil
}

func (m *Module) plain(ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.Result[Item]{}, nil
}

func (m *Module) noContent(ctx server.HandlerContext) (server.NoContentResult, server.IAPIError) {
	return server.NoContent(), nil
}

func (m *Module) bare(ctx server.HandlerContext) (Item, server.IAPIError) { return Item{}, nil }

func (m *Module) sealed(ctx server.HandlerContext) (server.ResultWithMeta[Sealed], server.IAPIError) {
	return server.ResultWithMeta[Sealed]{}, nil
}

func (m *Module) metaNoContent(ctx server.HandlerContext) (server.ResultWithMeta[Item], server.IAPIError) {
	return server.NewResultWithMeta(http.StatusNoContent, Item{}, nil), nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
`

// metaPagination is the module's other package, imported by metaModHead.
const metaPagination = `package pagination

type Meta struct {
	Next string ` + "`json:\"next\"`" + `
}
`

const (
	metaModRel   = "mod/module.go"
	metaItemsReg = `	server.GET(hr, r, "/items", m.withMeta)` + "\n"
)

// metaRun is one analysis of a metaModHead module.
type metaRun struct {
	a       *ProjectAnalyzer
	project *models.Project
	routes  []models.Route
	src     string
}

// metaModule analyzes metaModHead+body+"}" with the pagination package.
func metaModule(t *testing.T, body string) metaRun {
	t.Helper()
	src := metaModHead + body + "}\n"
	dir := t.TempDir()
	for rel, content := range map[string]string{
		"go.mod":             resolveGoMod,
		metaModRel:           src,
		"pagination/meta.go": metaPagination,
	} {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}
	a := New(dir)
	project, err := a.AnalyzeProject()
	require.NoError(t, err)
	var routes []models.Route
	for i := range project.Modules {
		routes = append(routes, project.Modules[i].Routes...)
	}
	return metaRun{a: a, project: project, routes: routes, src: src}
}

// route returns the run's route at method+" "+path.
func (r metaRun) route(t *testing.T, key string) models.Route {
	t.Helper()
	for i := range r.routes {
		if r.routes[i].Method+" "+r.routes[i].Path == key {
			return r.routes[i]
		}
	}
	t.Fatalf("route %q not found", key)
	return models.Route{}
}

// loc is the "mod/module.go:<line>: " prefix of the first source line
// holding marker.
func (r metaRun) loc(t *testing.T, marker string) string {
	t.Helper()
	return fileLoc(t, filepath.Join("mod", "module.go"), r.src, marker)
}

// warnings returns the run's warnings.
func (r metaRun) warnings(t *testing.T) []string {
	t.Helper()
	return r.a.Warnings(t.Context())
}

// metaWarnings returns the run's meta directive warnings.
func (r metaRun) metaWarnings(t *testing.T) []string {
	t.Helper()
	return warningsContaining(r.warnings(t), "directive meta")
}

// TestMetaDirectiveApplies verifies a meta directive above a ResultWithMeta
// route names its meta type, registers the component, and raises nothing.
func TestMetaDirectiveApplies(t *testing.T) {
	tests := []struct {
		name, body, key, want string
	}{
		{name: "local struct", body: "\t//openapi:meta PageMeta\n" + metaItemsReg, key: "GET /items", want: "PageMeta"},
		{name: "struct of another package", body: "\t//openapi:meta pagination.Meta\n" + metaItemsReg, key: "GET /items", want: "Meta"},
		{name: "pointer to ResultWithMeta", body: "\t//openapi:meta PageMeta\n\tserver.GET(hr, r, \"/ptr\", m.ptrMeta)\n", key: "GET /ptr", want: "PageMeta"},
		{name: "unresolved payload", body: "\t//openapi:meta PageMeta\n\tserver.GET(hr, r, \"/page\", m.pageMeta)\n", key: "GET /page", want: "PageMeta"},
		{name: "beside public and errors", body: "\t//openapi:public\n\t//openapi:meta PageMeta\n\t//openapi:errors 404\n" + metaItemsReg, key: "GET /items", want: "PageMeta"},
		{name: "trailing human comment", body: "\t//openapi:meta PageMeta // paging\n" + metaItemsReg, key: "GET /items", want: "PageMeta"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := metaModule(t, tt.body)
			route := run.route(t, tt.key)
			assert.Equal(t, models.ResultWrapperResultWithMeta, route.ResultWrapper)
			require.NotNil(t, route.Meta)
			assert.Equal(t, tt.want, route.Meta.Name)
			assert.Contains(t, run.a.typeRegistry, tt.want)
			// The unresolved payload raises its own generic-payload warning.
			assert.Empty(t, run.metaWarnings(t))
		})
	}
}

// TestMetaDirectiveRefused pins every refusal's one warning, located at the
// directive: the route documents no meta type and the run warns, so
// `generate --strict` fails.
func TestMetaDirectiveRefused(t *testing.T) {
	const directive = "\t//openapi:meta "
	tests := []struct {
		name, body, marker, want string
	}{
		{name: "no argument", body: "\t//openapi:meta\n" + metaItemsReg, want: metaNoArgsMsg},
		{name: "generic", body: directive + "Page[Item]\n" + metaItemsReg, want: fmt.Sprintf(metaGenericMsg, "Page[Item]")},
		{name: "two tokens", body: directive + "PageMeta Extra\n" + metaItemsReg, want: fmt.Sprintf(metaBadArgMsg, "PageMeta Extra")},
		{name: "two dots", body: directive + "a.b.C\n" + metaItemsReg, want: fmt.Sprintf(metaBadArgMsg, "a.b.C")},
		{name: "pointer", body: directive + "*PageMeta\n" + metaItemsReg, want: fmt.Sprintf(metaBadArgMsg, "*PageMeta")},
		{name: "undeclared", body: directive + "Missing\n" + metaItemsReg, want: fmt.Sprintf(metaUnresolvedMsg, "Missing")},
		{name: "non-struct", body: directive + "NotStruct\n" + metaItemsReg, want: fmt.Sprintf(metaUnresolvedMsg, "NotStruct")},
		{name: "unknown package", body: directive + "billing.Meta\n" + metaItemsReg, want: fmt.Sprintf(metaUnresolvedMsg, "billing.Meta")},
		{name: "stdlib type", body: directive + "http.Header\n" + metaItemsReg, want: fmt.Sprintf(metaUnresolvedMsg, "http.Header")},
		{name: "generic declaration", body: directive + "Page\n" + metaItemsReg, want: fmt.Sprintf(metaGenericDeclMsg, "Page")},
		{name: "marshaler", body: directive + "MarshalMeta\n" + metaItemsReg, want: fmt.Sprintf(metaMarshalerMsg, "MarshalMeta")},
		{name: "Result", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.plain)\n", want: metaNoWrapperMsg},
		{name: "NoContentResult", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.noContent)\n", want: metaNoWrapperMsg},
		{name: "bare payload", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.bare)\n", want: metaNoWrapperMsg},
		{name: "two pointers", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.ptrPtrMeta)\n", want: metaNoWrapperMsg},
		{name: "unresolvable handler", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.missing)\n", want: metaNoWrapperMsg},
		{name: "raw Add", body: directive + "PageMeta\n\tr.Add(http.MethodGet, \"/items\", m.withMeta)\n", want: metaNoWrapperMsg},
		{name: "WithRawResponse", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.withMeta, server.WithRawResponse())\n", want: metaRawMsg},
		{name: "JOSE", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.sealed)\n", want: metaJOSEMsg},
		{name: "204", body: directive + "PageMeta\n\tserver.GET(hr, r, \"/items\", m.metaNoContent)\n", want: metaNoBodyMsg},
		{name: "reserved key", body: directive + "StampMeta\n" + metaItemsReg, want: fmt.Sprintf(metaReservedMsg, "StampMeta", "timestamp")},
		{name: "promoted reserved key", body: directive + "EmbedMeta\n" + metaItemsReg, want: fmt.Sprintf(metaReservedMsg, "EmbedMeta", "traceId")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := metaModule(t, tt.body)
			assert.Equal(t, []string{run.loc(t, "//openapi:meta") + tt.want}, run.metaWarnings(t))
			// A double pointer also raises its own payload warning.
			if tt.name != "two pointers" {
				assert.Len(t, run.warnings(t), 1)
			}
			route := run.route(t, "GET /items")
			assert.Nil(t, route.Meta, "a refused directive documents no meta type")
		})
	}
}

// TestMetaDirectiveRepeatedLines locks the single-valued rule: lines naming
// one declaration collapse, two types warn once at the later line and
// neither applies, and a line failing on its own warns without stopping a
// valid one.
func TestMetaDirectiveRepeatedLines(t *testing.T) {
	t.Run("one declaration twice", func(t *testing.T) {
		run := metaModule(t, "\t//openapi:meta Meta\n\t//openapi:meta pagination.Meta\n"+metaItemsReg)
		assert.Empty(t, run.warnings(t))
		route := run.route(t, "GET /items")
		require.NotNil(t, route.Meta)
		assert.Equal(t, "Meta", route.Meta.Name)
	})
	t.Run("identical lines", func(t *testing.T) {
		run := metaModule(t, "\t//openapi:meta PageMeta\n\t//openapi:meta PageMeta\n"+metaItemsReg)
		assert.Empty(t, run.warnings(t))
		require.NotNil(t, run.route(t, "GET /items").Meta)
	})
	t.Run("two types", func(t *testing.T) {
		run := metaModule(t, "\t//openapi:meta PageMeta\n\t//openapi:meta OtherMeta\n"+metaItemsReg)
		want := run.loc(t, "//openapi:meta OtherMeta") + fmt.Sprintf(metaConflictMsg, "PageMeta", "OtherMeta")
		assert.Equal(t, []string{want}, run.warnings(t))
		assert.Nil(t, run.route(t, "GET /items").Meta)
		assert.NotContains(t, run.a.typeRegistry, "OtherMeta")
	})
	t.Run("malformed beside valid", func(t *testing.T) {
		run := metaModule(t, "\t//openapi:meta\n\t//openapi:meta PageMeta\n"+metaItemsReg)
		assert.Equal(t, []string{run.loc(t, "//openapi:meta\n") + metaNoArgsMsg}, run.warnings(t))
		require.NotNil(t, run.route(t, "GET /items").Meta)
	})
	t.Run("unresolvable beside valid", func(t *testing.T) {
		run := metaModule(t, "\t//openapi:meta Missing\n\t//openapi:meta PageMeta\n"+metaItemsReg)
		assert.Equal(t, []string{run.loc(t, "//openapi:meta Missing") + fmt.Sprintf(metaUnresolvedMsg, "Missing")}, run.warnings(t))
		require.NotNil(t, run.route(t, "GET /items").Meta)
	})
}

// TestMetaDirectivePlacement verifies placement is judged per name: a meta
// line in a type's doc comment is Detached, and in a group above a
// registration meta applies while a schema line beside it is Detached.
func TestMetaDirectivePlacement(t *testing.T) {
	t.Run("type doc comment", func(t *testing.T) {
		body := metaItemsReg + "}\n\n//openapi:meta PageMeta\ntype Doc struct{}\n\nfunc (m *Module) unused() {\n"
		run := metaModule(t, body)
		want := run.loc(t, "//openapi:meta PageMeta") + "directive meta has no effect — it is " + detachedMsg
		assert.Equal(t, []string{want}, run.warnings(t))
		assert.Nil(t, run.route(t, "GET /items").Meta)
	})
	t.Run("beside schema above a registration", func(t *testing.T) {
		run := metaModule(t, "\t//openapi:schema string\n\t//openapi:meta PageMeta\n"+metaItemsReg)
		want := run.loc(t, "//openapi:schema") + "directive schema has no effect — it is not in the doc comment of a package-level type declaration"
		assert.Equal(t, []string{want}, run.warnings(t))
		require.NotNil(t, run.route(t, "GET /items").Meta)
	})
}

// TestMetaDirectiveWarnsOncePerGroup verifies a registration a helper
// reaches under two prefixes applies one verdict: one warning when refused,
// the meta type on both routes when applied.
func TestMetaDirectiveWarnsOncePerGroup(t *testing.T) {
	helper := func(typ string) string {
		return "\ta := r.Group(\"/a\")\n\tm.helper(hr, a)\n\tb := r.Group(\"/b\")\n\tm.helper(hr, b)\n}\n\n" +
			"func (m *Module) helper(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" +
			"\t//openapi:meta " + typ + "\n" + metaItemsReg
	}
	run := metaModule(t, helper("Missing"))
	assert.Equal(t, []string{run.loc(t, "//openapi:meta") + fmt.Sprintf(metaUnresolvedMsg, "Missing")}, run.warnings(t))

	run = metaModule(t, helper("PageMeta"))
	assert.Empty(t, run.warnings(t))
	for _, key := range []string{"GET /a/items", "GET /b/items"} {
		route := run.route(t, key)
		require.NotNil(t, route.Meta, key)
		assert.Equal(t, "PageMeta", route.Meta.Name, key)
	}
}

// TestMetaDirectiveResolvesInRegistrationFile verifies a directive resolves
// its name against the file holding the registration — a delegate in a
// sibling package — not the module's.
func TestMetaDirectiveResolvesInRegistrationFile(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		"mod/module.go": `package mod

import (
	"github.com/example/app/users"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{ h *users.Handler }

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.h.RegisterRoutes(hr, r)
}
`,
		"users/handler.go": `package users

import "github.com/gaborage/go-bricks/server"

type Handler struct{}

type User struct {
	ID int64 ` + "`json:\"id\"`" + `
}

type UsersMeta struct {
	Total int64 ` + "`json:\"total\"`" + `
}

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:meta UsersMeta
	server.GET(hr, r, "/users", h.list)
}

func (h *Handler) list(ctx server.HandlerContext) (server.ResultWithMeta[[]User], server.IAPIError) {
	return server.ResultWithMeta[[]User]{}, nil
}
`,
	})
	assert.Empty(t, a.Warnings(t.Context()))
	assert.Contains(t, a.typeRegistry, "UsersMeta")
}

// TestMetaDirectiveRefusedAddsNoComponent verifies a refused directive
// leaves the generated document exactly as it is with the line deleted,
// even when the refusal comes after resolution (applicability, reserved
// keys) and the type is referenced nowhere else.
func TestMetaDirectiveRefusedAddsNoComponent(t *testing.T) {
	tests := []struct{ name, line, reg string }{
		{name: "Result route", line: "\t//openapi:meta PageMeta\n", reg: "\tserver.GET(hr, r, \"/items\", m.plain)\n"},
		{name: "reserved key", line: "\t//openapi:meta StampMeta\n", reg: metaItemsReg},
		{name: "conflict", line: "\t//openapi:meta PageMeta\n\t//openapi:meta OtherMeta\n", reg: metaItemsReg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			with := metaDocument(t, tt.line+tt.reg)
			without := metaDocument(t, tt.reg)
			assert.Equal(t, without, with)
			assert.NotContains(t, with, "PageMeta")
			assert.NotContains(t, with, "StampMeta")
		})
	}
}

// metaDocument generates the OpenAPI document of a metaModHead module.
func metaDocument(t *testing.T, body string) string {
	t.Helper()
	spec, err := generator.New("", "", "").Generate(metaModule(t, body).project)
	require.NoError(t, err)
	return spec
}

// TestMetaDirectiveDocument verifies the applied directive's document: the
// envelope meta is allOf the framework block and the type's $ref, a
// field-less meta type is emitted as an object, and a route without the
// directive keeps the framework block alone.
func TestMetaDirectiveDocument(t *testing.T) {
	spec := metaDocument(t, "\t//openapi:meta EmptyMeta\n"+metaItemsReg+"\tserver.GET(hr, r, \"/plain\", m.withMeta)\n")
	assert.Contains(t, spec, "$ref: '#/components/schemas/EmptyMeta'")
	assert.Contains(t, spec, "    EmptyMeta:\n      type: object\n")
	assert.Equal(t, 1, strings.Count(spec, "allOf:"), "only the directive's route has the allOf meta")
}

// TestBodyPropertyKey pins the property key the reserved-key check reads,
// the generator's propertyName rule: a JSON name, else the Go name, and
// nothing for a parameter or a json:"-" field.
func TestBodyPropertyKey(t *testing.T) {
	tests := []struct {
		name  string
		field models.FieldInfo
		want  string
	}{
		{name: "json name", field: models.FieldInfo{Name: "Stamp", JSONName: "timestamp"}, want: "timestamp"},
		{name: "go name", field: models.FieldInfo{Name: "Total"}, want: "Total"},
		{name: "parameter", field: models.FieldInfo{Name: "TraceID", JSONName: "traceId", ParamType: "query"}, want: ""},
		{name: "json dash", field: models.FieldInfo{Name: "Timestamp", JSONName: "timestamp", JSONSkip: true}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, bodyPropertyKey(&tt.field))
		})
	}
}

// TestMetaDirectiveAliasesCollapse verifies lines naming one type through an
// alias collapse whatever its name (a Go alias is the type it names), the
// first line's spelling registering, while a defined type over the same
// struct is a type of its own and conflicts.
func TestMetaDirectiveAliasesCollapse(t *testing.T) {
	tests := []struct{ name, lines, want string }{
		{name: "renamed alias first", lines: "\t//openapi:meta PM\n\t//openapi:meta pagination.Meta\n", want: "PM"},
		{name: "target first", lines: "\t//openapi:meta pagination.Meta\n\t//openapi:meta PM\n", want: "Meta"},
		{name: "local alias", lines: "\t//openapi:meta PA\n\t//openapi:meta PageMeta\n", want: "PA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := metaModule(t, tt.lines+metaItemsReg)
			assert.Empty(t, run.warnings(t))
			route := run.route(t, "GET /items")
			require.NotNil(t, route.Meta)
			assert.Equal(t, tt.want, route.Meta.Name)
		})
	}

	run := metaModule(t, "\t//openapi:meta DM\n\t//openapi:meta pagination.Meta\n"+metaItemsReg)
	want := run.loc(t, "//openapi:meta pagination.Meta") + fmt.Sprintf(metaConflictMsg, "DM", "pagination.Meta")
	assert.Equal(t, []string{want}, run.warnings(t))
	assert.Nil(t, run.route(t, "GET /items").Meta)
}

// TestMetaDirectiveProbeRaisesNoFieldWarning verifies the reserved-key probe
// raises none of the type's own field warnings: a refused type, never
// emitted, warns only for the directive, and an accepted one still warns
// once, when it is registered.
func TestMetaDirectiveProbeRaisesNoFieldWarning(t *testing.T) {
	refused := metaModule(t, "\t//openapi:meta TagStampMeta\n"+metaItemsReg)
	want := refused.loc(t, "//openapi:meta") + fmt.Sprintf(metaReservedMsg, "TagStampMeta", "timestamp")
	assert.Equal(t, []string{want}, refused.warnings(t))

	accepted := metaModule(t, "\t//openapi:meta TagMeta\n"+metaItemsReg)
	assert.Len(t, warningsContaining(accepted.warnings(t), "is not readable by reflect.StructTag"), 1)
	assert.Len(t, accepted.warnings(t), 1)
	require.NotNil(t, accepted.route(t, "GET /items").Meta)
}

// metaDupModule is a module package pkg registering GET /x, its directive
// lines (extra) above the registration.
func metaDupModule(pkg, extra string) string {
	return "package " + pkg + `

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "` + pkg + `" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Item struct {
	ID int64 ` + "`json:\"id\"`" + `
}

type Nest struct {
	Q string ` + "`json:\"q\"`" + `
}

type LoserMeta struct {
	N Nest ` + "`json:\"n\"`" + `
}

func (m *Module) h(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) {
	return server.ResultWithMeta[[]Item]{}, nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
` + extra + `	server.GET(hr, r, "/x", m.h)
}
`
}

// TestMetaDirectiveCrossModuleLoserRegistersNothing verifies a route another
// module already claimed registers no meta component: the meta type is
// registered only after the duplicate-route pass drops the loser.
func TestMetaDirectiveCrossModuleLoserRegistersNothing(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		"a/a.go": metaDupModule("a", ""),
		"b/b.go": metaDupModule("b", "\t//openapi:meta LoserMeta\n"),
	})
	assert.Len(t, warningsContaining(a.Warnings(t.Context()), "is the same route node"), 1)
	assert.Empty(t, warningsContaining(a.Warnings(t.Context()), "directive meta"))
	assert.NotContains(t, a.typeRegistry, "LoserMeta")
	assert.NotContains(t, a.typeRegistry, "Nest")
}

// TestMetaDirectiveInterfaceCopiesDisagree verifies a meta type whose
// build-tagged copies (or a name on its chain) disagree on an in-module
// interface's marshal methods (issue 179) is refused at the directive: no
// single declaration can be selected, so it registers no component, and
// the route keeps the framework meta with one located warning rather than
// losing the directive silently.
func TestMetaDirectiveInterfaceCopiesDisagree(t *testing.T) {
	for _, typ := range []string{"BT", "DBT"} {
		t.Run(typ, func(t *testing.T) {
			head := resolveModuleHead + "\ntype Item struct {\n\tID int64 `json:\"id\"`\n}\n\ntype DBT BT\n\n" +
				"func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" +
				"\t//openapi:meta " + typ + "\n" +
				"\tserver.GET(hr, r, \"/items\", m.items)\n}\n\n" +
				"func (m *Module) items(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) { panic(0) }\n"
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod": resolveGoMod,
				filepath.Join("mod", "bt_linux.go"): "//go:build linux\n\npackage mod\n\n" +
					"type BT interface{ MarshalJSON() ([]byte, error) }\n",
				filepath.Join("mod", "bt_other.go"): "//go:build !linux\n\npackage mod\n\n" +
					"type BT struct {\n\tX int `json:\"x\"`\n}\n",
				filepath.Join("mod", "module.go"): head,
			})
			want := fileLoc(t, filepath.Join("mod", "module.go"), head, "//openapi:meta") + fmt.Sprintf(metaUnresolvedMsg, typ)
			assert.Equal(t, []string{want}, a.Warnings(t.Context()))
			assert.NotContains(t, a.typeRegistry, typ)
			assert.NotContains(t, a.typeRegistry, "BT")
		})
	}
}
