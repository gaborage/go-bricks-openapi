package analyzer

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// funcChanModule declares issue 158's DReq request and EResp payload, plus a
// GET route whose request binds a func-typed query parameter.
const funcChanModule = resolveModuleHead + "\n" + `type F func()
type Ch chan int
type fn func()

type DReq struct {
	F    func()            ` + "`json:\"f\"`" + `
	C    chan int          ` + "`json:\"c\"`" + `
	RO   <-chan string     ` + "`json:\"ro\"`" + `
	FS   []func()          ` + "`json:\"fs\"`" + `
	FM   map[string]func() ` + "`json:\"fm\"`" + `
	NF   F                 ` + "`json:\"nf\"`" + `
	NC   Ch                ` + "`json:\"nc\"`" + `
	PA   *[2]Ch            ` + "`json:\"pa\"`" + `
	Anon struct {
		In func(int) ` + "`json:\"in\"`" + `
	} ` + "`json:\"anon\"`" + `
	Skip func()            ` + "`json:\"-\"`" + `
	Name string            ` + "`json:\"name\"`" + `
}

type EResp struct {
	G func(int) error ` + "`json:\"g\"`" + `
	F
	Ch
	fn
	X int ` + "`json:\"x\"`" + `
}

type QReq struct {
	CB func() ` + "`query:\"cb\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/d", m.d)
	server.GET(hr, r, "/q", m.q)
}

func (m *Module) d(req DReq, ctx server.HandlerContext) (server.Result[EResp], server.IAPIError) {
	return server.Result[EResp]{}, nil
}

func (m *Module) q(req QReq, ctx server.HandlerContext) (server.Result[EResp], server.IAPIError) {
	return server.Result[EResp]{}, nil
}
`

// TestFuncChanFieldsWarn pins issue 158: a body field holding a func or chan
// (literal or named, any chan direction, at any depth through slices,
// arrays, maps, pointers and named types, inside an anonymous struct, or as
// an exported embed of a named func or chan type) keeps its untyped-object
// schema and raises exactly one warning, naming its declared type and what
// it holds. json:"-" and an unexported embed stay silent, and a func-typed
// query parameter keeps only its binding warning.
func TestFuncChanFieldsWarn(t *testing.T) {
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("mod", "module.go"): funcChanModule,
	})
	rows := []struct{ goName, declared, held string }{
		{"F", "func()", models.UnknownFunc},
		{"C", "chan int", models.UnknownChan},
		{"RO", "<-chan string", models.UnknownChan},
		{"FS", "[]func()", models.UnknownFunc},
		{"FM", "map[string]func()", models.UnknownFunc},
		{"NF", "F", models.UnknownFunc},
		{"NC", "Ch", models.UnknownChan},
		{"PA", "*[2]Ch", models.UnknownChan},
		{"In", "func(int)", models.UnknownFunc},
		{"G", "func(int) error", models.UnknownFunc},
		{"Ch", "Ch", models.UnknownChan},
	}
	for _, r := range rows {
		w := fieldWarnings(a, r.goName)
		if r.goName == "F" {
			// DReq's F field and EResp's F embed, one warning each.
			require.Len(t, w, 2, "%v", w)
			assert.Len(t, matching(w, "has type F, which holds a func, a Go type encoding/json cannot marshal"), 1, "the F embed")
		} else if !assert.Len(t, w, 1, r.goName) {
			continue
		}
		assert.Contains(t, w[0], fmt.Sprintf("has type %s, which holds a %s, a Go type encoding/json cannot marshal", r.declared, r.held), r.goName)
	}
	for _, name := range []string{"Skip", "fn", "Name", "X"} {
		assert.Empty(t, fieldWarnings(a, name), name)
	}
	if w := fieldWarnings(a, "CB"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], `is the query parameter "cb" of type func()`)
	}
	assert.Len(t, a.Warnings(t.Context()), 13, "one per field row, two for F, one for CB: %v", a.Warnings(t.Context()))

	d := renders(t, a.typeRegistry["DReq"])
	for key, want := range map[string]string{
		"f": "unknown", "c": "unknown", "ro": "unknown", "fs": "[]unknown", "fm": "map[string]unknown",
		"nf": "unknown", "nc": "unknown", "pa": "*[N]unknown", "anon": "struct{in unknown}",
	} {
		assert.Equal(t, want, d[key], key)
	}
	e := renders(t, a.typeRegistry["EResp"])
	for _, key := range []string{"g", "F", "Ch"} {
		assert.Equal(t, "unknown", e[key], key)
	}
}

// TestFuncChanPayloadNoWarning pins that warnPayload raises nothing for a
// func-or-chan fallback: a payload over a func or chan is a README known
// limitation, warned (or not) where it was before issue 158.
func TestFuncChanPayloadNoWarning(t *testing.T) {
	a := New(t.TempDir())
	r := models.TypeShape{Kind: models.ShapeMap, Elem: &models.TypeShape{Kind: models.ShapeUnknown, Name: models.UnknownFunc}}
	fb := fieldFallback{kind: fallbackFuncChan, typeName: models.UnknownFunc, detail: models.UnknownFunc}
	assert.False(t, a.warnPayload("map[string]Fn", &r, fb))
	assert.Empty(t, a.warnings)
}

// TestTypeShapeFuncChan pins the Names typeShape gives func and chan nodes,
// through parentheses, so their fields can warn, while another unmodelled
// node keeps an empty Name.
func TestTypeShapeFuncChan(t *testing.T) {
	a := New(t.TempDir())
	for src, want := range map[string]string{
		"func()": models.UnknownFunc, "func(int) error": models.UnknownFunc, "(func())": models.UnknownFunc,
		"chan int": models.UnknownChan, "<-chan string": models.UnknownChan, "chan<- int": models.UnknownChan,
	} {
		s := a.typeShape(mustParse(t, src))
		assert.Equal(t, models.TypeShape{Kind: models.ShapeUnknown, Name: want}, s, src)
	}
	assert.Equal(t, models.TypeShape{Kind: models.ShapeUnknown}, a.typeShape(&ast.Ellipsis{}))
}

// funcChanChainModule renders a module whose Resp struct holds respBody,
// importing the module's types package, with decls appended.
func funcChanChainModule(respBody, decls string) string {
	src := btagModule(respBody, [][2]string{{"/resp", "server.Result[Resp]"}}, "")
	src = strings.Replace(src, "\"github.com/gaborage/go-bricks/server\"\n",
		"\"github.com/gaborage/go-bricks/server\"\n\t\"github.com/example/app/types\"\n", 1)
	return src + decls
}

// TestFuncChanNamedChains pins issue 158 through named types: a func type of
// another package of the module, an alias of a local one, a defined type
// over one, and build-tagged copies that agree each raise the func-or-chan
// warning once. A func type with its own MarshalJSON keeps the Marshaler
// warning, since encoding/json uses that method instead.
func TestFuncChanNamedChains(t *testing.T) {
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("types", "types.go"): "package types\n\ntype F func()\n",
		filepath.Join("mod", "t_linux.go"): "//go:build linux\n\npackage mod\n\ntype T func()\n",
		filepath.Join("mod", "t_other.go"): "//go:build !linux\n\npackage mod\n\ntype T func()\n",
		filepath.Join("mod", "module.go"): funcChanChainModule(
			"\tQF types.F `json:\"qf\"`\n\tAF A `json:\"af\"`\n\tGF G `json:\"gf\"`\n\tTF T `json:\"tf\"`\n\tMF MF `json:\"mf\"`\n",
			"type L func()\n\ntype A = L\n\ntype G L\n\ntype MF func()\n\nfunc (MF) MarshalJSON() ([]byte, error) { return nil, nil }\n"),
	})
	for name, declared := range map[string]string{"QF": "types.F", "AF": "A", "GF": "G", "TF": "T"} {
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], "has type "+declared+", which holds a func, a Go type encoding/json cannot marshal", name)
		}
	}
	if w := fieldWarnings(a, "MF"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "has its own MarshalJSON method")
	}
}

// TestFuncChanBuildTagDisagree pins build-tagged copies declared as a func in
// one file and a chan in another: the first copy's func is noted before the
// copies are compared, so they raise the func-or-chan warning once, not the
// build-tagged one. Before issue 158 they merged silently.
func TestFuncChanBuildTagDisagree(t *testing.T) {
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("mod", "d_linux.go"): "//go:build linux\n\npackage mod\n\ntype D func()\n",
		filepath.Join("mod", "d_other.go"): "//go:build !linux\n\npackage mod\n\ntype D chan int\n",
		filepath.Join("mod", "module.go"):  btagModule("\tDF D `json:\"df\"`\n", [][2]string{{"/resp", "server.Result[Resp]"}}, ""),
	})
	w := fieldWarnings(a, "DF")
	require.Len(t, w, 1, "%v", w)
	assert.Contains(t, w[0], "has type D, which holds a func, a Go type encoding/json cannot marshal")
}
