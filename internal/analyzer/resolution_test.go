package analyzer

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

const resolveGoMod = "module github.com/example/app\n\ngo 1.25\n"

// resolveModuleHead is the go-bricks Module boilerplate the resolution tests
// put in front of their handlers.
const resolveModuleHead = `package mod

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }
`

// fieldByJSON returns ti's field with the given JSON name.
func fieldByJSON(t *testing.T, ti *models.TypeInfo, jsonName string) *models.FieldInfo {
	t.Helper()
	require.NotNil(t, ti)
	for i := range ti.Fields {
		if ti.Fields[i].JSONName == jsonName {
			return &ti.Fields[i]
		}
	}
	require.Failf(t, "field not found", "%s has no field %q", ti.Name, jsonName)
	return nil
}

// TestStructExtractedInDeclaringFile pins Step 0 of the named-type
// resolution: a struct declared in a sibling file of the handler (which alone
// does not import the field types' package) is extracted in its OWN file, so
// its qualified fields and embeds resolve against that file's imports. Resp is
// reached directly (registerTypeAt) and through an alias declared in the
// handler's file (resolveTypeSpecChain's base case).
func TestStructExtractedInDeclaringFile(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("models", "item.go"): "package models\n\n" +
			"type Item struct{ ID int `json:\"id\"` }\n\n" +
			"type Base struct{ Created string `json:\"created\"` }\n",
		filepath.Join("mod", "types.go"): "package mod\n\nimport m \"github.com/example/app/models\"\n\n" +
			"type Resp struct {\n\tm.Base\n\tItems []m.Item `json:\"items\"`\n\tOne m.Item `json:\"one\"`\n}\n",
		filepath.Join("mod", "module.go"): resolveModuleHead + `
type RespA = Resp

func (m *Module) one(ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) { return server.Result[Resp]{}, nil }
func (m *Module) two(ctx server.HandlerContext) (server.Result[RespA], server.IAPIError) {
	return server.Result[RespA]{}, nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/one", m.one)
	server.GET(hr, r, "/two", m.two)
}
`,
	})

	for _, name := range []string{"Resp", "RespA"} {
		ti := a.typeRegistry[name]
		require.NotNil(t, ti, "%s registered", name)
		assert.Equal(t, "[]$Item", renderShape(fieldByJSON(t, ti, "items").ResolvedShape()), "%s.items", name)
		assert.Equal(t, "$Item", renderShape(fieldByJSON(t, ti, "one").ResolvedShape()), "%s.one", name)
		fieldByJSON(t, ti, "created") // the qualified embed is promoted
	}
	assert.Contains(t, a.typeRegistry, "Item")
	assert.Empty(t, a.Warnings(t.Context()))
}

// rowsHandler is the handler and route set every rows module ends with: GET
// /rows returns Rows, taking Req when the module declares one.
const rowsHandler = `
func (m *Module) get(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.get)
}
`

// rowsModule builds a module file whose one route returns Rows. imports are
// extra import lines, decls the type and method declarations, and rows the
// Rows struct's body.
func rowsModule(imports, decls, rows string) string {
	return "package mod\n\nimport (\n\t\"github.com/gaborage/go-bricks/app\"\n\t\"github.com/gaborage/go-bricks/server\"\n" + imports + ")\n\n" +
		"type Module struct{}\n\n" +
		"func (m *Module) Name() string                    { return \"mod\" }\n" +
		"func (m *Module) Init(deps *app.ModuleDeps) error { return nil }\n" +
		"func (m *Module) Shutdown() error                 { return nil }\n\n" +
		decls + "\n\ntype Rows struct {\n" + rows + "}\n" + rowsHandler
}

// resolveRow is one Rows field: its JSON name (the Go name is the same,
// exported), its Go type and the expected rendered resolution.
type resolveRow struct {
	json, goType, want string
}

func exportName(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

// rowsBody renders rows as Rows struct fields.
func rowsBody(rows []resolveRow) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "\t%s %s `json:%q`\n", exportName(r.json), r.goType, r.json)
	}
	return b.String()
}

// analyzeRows analyzes a single-module project and returns the analyzer and
// the rendered resolution of each Rows field, by JSON name.
func analyzeRows(t *testing.T, imports, decls, rows string) (a *ProjectAnalyzer, got map[string]string) {
	t.Helper()
	a, _ = analyzeSingleModule(t, rowsModule(imports, decls, rows))
	return a, renders(t, a.typeRegistry["Rows"])
}

// renders maps each field of ti, by JSON name, to its rendered resolution
// ("<nil>" when the field has no Resolution).
func renders(t *testing.T, ti *models.TypeInfo) map[string]string {
	t.Helper()
	require.NotNil(t, ti, "type registered")
	out := map[string]string{}
	for i := range ti.Fields {
		f := &ti.Fields[i]
		key := f.JSONName
		if key == "" || key == jsonSkipValue {
			key = f.Name
		}
		if f.Resolution == nil {
			out[key] = "<nil>"
			continue
		}
		out[key] = renderShape(f.ResolvedShape())
	}
	return out
}

// fieldWarnings returns the warnings that name the field goName.
func fieldWarnings(a *ProjectAnalyzer, goName string) []string {
	var out []string
	for _, w := range a.warnings {
		if strings.Contains(w, "field "+goName+" at ") {
			out = append(out, w)
		}
	}
	return out
}

const rowsImports = "\t\"encoding/json\"\n\t\"time\"\n\t\"github.com/google/uuid\"\n\t\"github.com/shopspring/decimal\"\n"

// rowsDecls declares every resolution row type (R1-R14 of the plan).
const rowsDecls = `
type User struct{ Name string ` + "`json:\"name\"`" + ` }
type Address struct{ City string ` + "`json:\"city\"`" + ` }
type Cents int64
type Flag byte
type U8 = uint8

type Tags []string
type TagsA = []string
type Chain Tags
type Attrs map[string]string
type Stamps []time.Time
type Ids []Cents

type UserList []User
type PAddr *Address
type PC *int64
type FlagB bool
type AnyD any
type Shaper interface{ Area() float64 }

type Pair [2]int
type Key [4]byte
type Arr [3]byte

type Blob []byte
type BlobA = []byte
type B2 Blob
type Flags []Flag
type Raw json.RawMessage
type RawOfRaw Raw

type Stamp = time.Time
type IDA = uuid.UUID
type RawA = json.RawMessage
type Amount json.Number

type Status int
func (s Status) MarshalText() ([]byte, error) { return nil, nil }
type StatusA = Status
type StatusD Status
type Odd int
func (Odd) MarshalText() string { return "" }
type FlagU byte
func (f *FlagU) UnmarshalJSON(b []byte) error { return nil }
type TagsM []string
func (TagsM) MarshalJSON() ([]byte, error) { return nil, nil }
type FlagV byte
func (f *FlagV) MarshalJSON() ([]byte, error) { return nil, nil }

type Tree map[string]Tree
type List []List
type Left []Right
type Right map[string]Left

type StampD time.Time
type IDD uuid.UUID
type C complex128
type Addr uintptr
`

// TestResolveFieldRows pins the Resolution of every row type in its direct
// position (and a few extra positions).
func TestResolveFieldRows(t *testing.T) {
	rows := []resolveRow{
		{"tags", "Tags", "[]string"},
		{"tagsPtr", "*Tags", "*[]string"},
		{"tagsA", "TagsA", "[]string"},
		{"chain", "Chain", "[]string"},
		{"attrs", "Attrs", "map[string]string"},
		{"stamps", "Stamps", "[]time.Time"},
		{"ids", "Ids", "[]int64"},
		{"users", "UserList", "[]$User"},
		{"addr", "PAddr", "*$Address"},
		{"pc", "PC", "*int64"},
		{"flagB", "FlagB", "bool"},
		{"anyD", "AnyD", "any"},
		{"shaper", "Shaper", "interface{}"},
		{"pair", "Pair", "[N]int"},
		{"key", "Key", "[N]byte"},
		{"arr", "Arr", "[N]byte"},
		{"flags3", "[3]Flag", "[N]byte"},
		{"flagPtrs", "[]*Flag", "[]*byte"},
		{"grid", "[][]Cents", "[][]int64"},
		{"centsMap", "map[string]Cents", "map[string]int64"},
		{"usm", "[]map[string]User", "[]map[string]$User"},
		{"blob", "Blob", "[]byte"},
		{"blobA", "BlobA", "[]byte"},
		{"b2", "B2", "[]byte"},
		{"flags", "Flags", "[]byte"},
		{"raw", "Raw", "[]byte"},
		{"rawOfRaw", "RawOfRaw", "[]byte"},
		{"u8s", "[]U8", "[]uint8"},
		{"flagUs", "[]FlagU", "[]byte"},
		{"stamp", "Stamp", "time.Time"},
		{"ida", "IDA", "uuid.UUID"},
		{"rawA", "RawA", "json.RawMessage"},
		{"amount", "Amount", "string"},
		{"statusD", "StatusD", "int"},
		{"odd", "Odd", "int"},
		{"status", "Status", "marshal:Status(int)"},
		{"statusA", "StatusA", "marshal:Status(int)"},
		{"statusPtr", "*Status", "*marshal:Status(int)"},
		{"tagsM", "TagsM", "marshal:TagsM([]string)"},
		{"flagVs", "[]FlagV", "[]marshal:FlagV(byte)"},
		{"tree", "Tree", "map[string]cycle:Tree"},
		{"list", "List", "[]cycle:List"},
		{"left", "Left", "[]map[string]cycle:Left"},
		{"decimal", "decimal.Decimal", "decimal.Decimal"},
		{"stampD", "StampD", "StampD"},
		{"idd", "IDD", "IDD"},
		{"addr2", "Addr", "uintptr"},
		{"cents", "Cents", "int64"},
		{"duration", "time.Duration", "int64"},
		{"month", "time.Month", "int"},
	}
	a, got := analyzeRows(t, rowsImports, rowsDecls, rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], "%s (%s)", r.json, r.goType)
	}
	assert.Contains(t, a.typeRegistry, "User")
	assert.Contains(t, a.typeRegistry, "Address")
}

// TestResolveRawAndStampAliasesVersusDefined pins that an alias of a
// well-known type keeps it, while a defined type over it resolves to its
// kind-backed underlying (RawMessage) or to nothing (time.Time).
func TestResolveRawAndStampAliasesVersusDefined(t *testing.T) {
	rows := []resolveRow{
		{"raw", "Raw", "[]byte"},
		{"rawA", "RawA", "json.RawMessage"},
		{"stamp", "Stamp", "time.Time"},
		{"stampD", "StampD", "StampD"},
		{"direct", "json.RawMessage", "json.RawMessage"},
		{"number", "json.Number", "json.Number"},
		{"rawPtr", "*json.RawMessage", "*json.RawMessage"},
	}
	_, got := analyzeRows(t, rowsImports, rowsDecls, rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
}

// TestResolveChains pins defined chains reaching []byte, and that a fixed
// byte array never becomes a slice.
func TestResolveChains(t *testing.T) {
	rows := []resolveRow{
		{"b2", "B2", "[]byte"},
		{"rawOfRaw", "RawOfRaw", "[]byte"},
		{"x", "X", "[]byte"},
		{"arr", "Arr", "[N]byte"},
	}
	_, got := analyzeRows(t, rowsImports, rowsDecls+"\ntype X RawA\n", rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
}

// TestResolveRecursionTerminates pins that a named type met again inside
// itself, or a chain of more than maxNamedResolutionDepth distinct named
// types, is cut with one warning, while a chain of exactly
// maxNamedResolutionDepth resolves (resolveTypeSpecChain's nine hops). A true
// recursion names the type that contains itself; a depth-cap cut on a chain
// that never repeats says so, and never claims the cut type contains itself.
func TestResolveRecursionTerminates(t *testing.T) {
	var chain strings.Builder
	for i := range 9 {
		fmt.Fprintf(&chain, "type A%d []A%d\n", i, i+1)
	}
	chain.WriteString("type A9 int\n")
	rows := []resolveRow{
		{"tree", "Tree", "map[string]cycle:Tree"},
		{"list", "List", "[]cycle:List"},
		{"left", "Left", "[]map[string]cycle:Left"},
		{"f", "A0", "[][][][][][][][][]cycle:A9"},
	}
	warnings := map[string]string{
		"tree": "has type Tree: Tree contains itself",
		"list": "has type List: List contains itself",
		"left": "has type Left: Left contains itself",
		"f":    "has type A0: its named types nest more than 9 deep, so A9 is cut",
	}
	nine := resolveRow{"g", "A1", "[][][][][][][][]int"}
	a, got := analyzeRows(t, rowsImports, rowsDecls+chain.String(), rowsBody(append(rows, nine)))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
		w := fieldWarnings(a, exportName(r.json))
		if assert.Len(t, w, 1, r.json) {
			assert.Contains(t, w[0], warnings[r.json], r.json)
		}
	}
	if w := fieldWarnings(a, "F"); assert.Len(t, w, 1) {
		assert.NotContains(t, w[0], "contains itself", "a depth-cap cut is not a recursion")
	}
	assert.Equal(t, nine.want, got[nine.json], "a nine-type chain resolves")
	assert.Empty(t, fieldWarnings(a, exportName(nine.json)), "a nine-type chain does not warn")
}

// marshalerMethodDecls is each of the seven methods, declared on recv.
var marshalerMethodDecls = map[string]string{
	methodMarshalJSON:       "func (%s) MarshalJSON() ([]byte, error) { return nil, nil }",
	methodMarshalText:       "func (%s) MarshalText() ([]byte, error) { return nil, nil }",
	methodMarshalJSONTo:     "func (%s) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }",
	methodAppendText:        "func (%s) AppendText(b []byte) ([]byte, error) { return b, nil }",
	methodUnmarshalJSON:     "func (%s) UnmarshalJSON(b []byte) error { return nil }",
	methodUnmarshalText:     "func (%s) UnmarshalText(b []byte) error { return nil }",
	methodUnmarshalJSONFrom: "func (%s) UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }",
}

// TestMarshalerGuardPositions pins the guard for each method on T and *T: a
// direct or aliased use is a Marshaler leaf with one warning, a defined type
// over it is not, and a param or a json:"-" field is exempt.
func TestMarshalerGuardPositions(t *testing.T) {
	for method, decl := range marshalerMethodDecls {
		for _, recv := range []string{"T", "*T"} {
			t.Run(method+"/"+recv, func(t *testing.T) {
				decls := "type T int\n" + fmt.Sprintf(decl, "t "+recv) +
					"\ntype TA = T\ntype TD T\ntype Odd int\nfunc (Odd) MarshalText() string { return \"\" }\n"
				rows := "\tF T `json:\"f\"`\n\tFA TA `json:\"fa\"`\n\tFD TD `json:\"fd\"`\n\tOdd Odd `json:\"odd\"`\n" +
					"\tQ T `query:\"q\"`\n\tX T `json:\"-\"`\n"
				a, got := analyzeRows(t, "\t\"encoding/json/jsontext\"\n", decls, rows)
				assert.Equal(t, "marshal:T(int)", got["f"])
				assert.Equal(t, "marshal:T(int)", got["fa"])
				assert.Equal(t, "int", got["fd"])
				assert.Equal(t, "int", got["odd"])
				assert.Equal(t, "int", got["Q"])
				assert.Equal(t, "<nil>", got["X"])
				for _, name := range []string{"F", "FA"} {
					if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
						assert.Contains(t, w[0], method)
						assert.Contains(t, w[0], "has type "+map[string]string{"F": "T", "FA": "TA"}[name]+": T has its own "+method+" method")
					}
				}
				for _, name := range []string{"FD", "Odd", "Q", "X"} {
					assert.Empty(t, fieldWarnings(a, name), name)
				}
			})
		}
	}
}

// TestByteSliceElementRule pins #97: a slice of a byte-kind element is base64
// unless the element has an encode-side method; arrays and pointer elements
// never take the rule.
func TestByteSliceElementRule(t *testing.T) {
	decls := `
type FlagU byte
func (f *FlagU) UnmarshalJSON(b []byte) error { return nil }
type FlagUA = FlagU
type FlagT byte
func (f *FlagT) UnmarshalText(b []byte) error { return nil }
type FlagV byte
func (f *FlagV) MarshalJSON() ([]byte, error) { return nil, nil }
type FlagA byte
func (f FlagA) AppendText(b []byte) ([]byte, error) { return b, nil }
type FlagM byte
func (f FlagM) MarshalText() ([]byte, error) { return nil, nil }
type FlagJ byte
func (f FlagJ) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }
`
	rows := []resolveRow{
		{"us", "[]FlagU", "[]byte"},
		{"ts", "[]FlagT", "[]byte"},
		{"uas", "[]FlagUA", "[]byte"},
		{"vs", "[]FlagV", "[]marshal:FlagV(byte)"},
		{"as", "[]FlagA", "[]marshal:FlagA(byte)"},
		{"ms", "[]FlagM", "[]marshal:FlagM(byte)"},
		{"js", "[]FlagJ", "[]marshal:FlagJ(byte)"},
		{"arr", "[3]FlagU", "[N]marshal:FlagU(byte)"},
		{"ptrs", "[]*FlagU", "[]*marshal:FlagU(byte)"},
	}
	a, got := analyzeRows(t, "\t\"encoding/json/jsontext\"\n", decls, rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
		warned := strings.Contains(r.want, "marshal:")
		assert.Equal(t, warned, len(fieldWarnings(a, exportName(r.json))) == 1, "%s warns: %v", r.json, a.warnings)
	}
}

// TestResolveRegistersRefLeavesAtDepth pins that a struct leaf at any depth
// is registered and carries its final (collision-qualified) component name.
func TestResolveRegistersRefLeavesAtDepth(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		rows := []resolveRow{
			{"grid", "[][]User", "[][]$User"},
			{"gridPtr", "*[][]User", "*[][]$User"},
			{"usm", "[]map[string]User", "[]map[string]$User"},
			{"msg", "map[string][][]User", "map[string][][]$User"},
			{"list", "UserList", "[]$User"},
			{"p", "PAddr", "*$Address"},
		}
		a, got := analyzeRows(t, rowsImports, rowsDecls, rowsBody(rows))
		for _, r := range rows {
			assert.Equal(t, r.want, got[r.json], r.json)
		}
		assert.Contains(t, a.typeRegistry, "User")
		assert.Contains(t, a.typeRegistry, "Address")
		assert.Empty(t, a.Warnings(t.Context()))
	})

	t.Run("collision", func(t *testing.T) {
		a := analyzeDirectiveProject(t, map[string]string{
			"go.mod":                           resolveGoMod,
			filepath.Join("orders", "user.go"): "package orders\n\ntype User struct{ X int `json:\"x\"` }\n",
			filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/orders\"\n",
				"type User struct{ Y int `json:\"y\"` }\n",
				"\tMine []User `json:\"mine\"`\n\tTheirs [][]orders.User `json:\"theirs\"`\n"),
		})
		got := renders(t, a.typeRegistry["Rows"])
		assert.Equal(t, "[]$User", got["mine"])
		assert.Equal(t, "[][]$OrdersUser", got["theirs"])
		fieldByJSON(t, a.typeRegistry["OrdersUser"], "x")
		fieldByJSON(t, a.typeRegistry["User"], "y")
	})

	siblingFiles := func(apiImports string) map[string]string {
		return map[string]string{
			"go.mod": resolveGoMod,
			filepath.Join("domain", "domain.go"): "package domain\n\n" +
				"type User struct{ Name string `json:\"name\"` }\n\ntype Address struct{ City string `json:\"city\"` }\n",
			filepath.Join("other", "other.go"): "package other\n\ntype User struct{ Z bool `json:\"z\"` }\n",
			filepath.Join("mod", "types.go"): "package mod\n\nimport \"github.com/example/app/domain\"\n\n" +
				"type Users []domain.User\ntype PAddr2 *domain.Address\ntype UserMap map[string]domain.User\n",
			filepath.Join("mod", "api.go"): "package mod\n\n" + apiImports +
				"type Rows struct {\n\tUsers Users `json:\"users\"`\n\tP PAddr2 `json:\"p\"`\n\tM UserMap `json:\"m\"`\n}\n",
			filepath.Join("mod", "module.go"): resolveModuleHead + rowsHandler,
		}
	}

	t.Run("sibling file", func(t *testing.T) {
		a := analyzeDirectiveProject(t, siblingFiles(""))
		got := renders(t, a.typeRegistry["Rows"])
		assert.Equal(t, "[]$User", got["users"])
		assert.Equal(t, "*$Address", got["p"])
		assert.Equal(t, "map[string]$User", got["m"])
		fieldByJSON(t, a.typeRegistry["User"], "name")
		assert.Contains(t, a.typeRegistry, "Address")
		assert.Empty(t, a.Warnings(t.Context()))
	})

	t.Run("alias shadow", func(t *testing.T) {
		a := analyzeDirectiveProject(t, siblingFiles("import domain \"github.com/example/app/other\"\n\nvar _ domain.User\n\n"))
		got := renders(t, a.typeRegistry["Rows"])
		assert.Equal(t, "[]$User", got["users"])
		fieldByJSON(t, a.typeRegistry["User"], "name") // types.go's domain, not api.go's
		assert.Empty(t, a.Warnings(t.Context()))
	})
}

// TestSiblingFileRefSites pins that ref leaves register in the file their name
// was written in beyond direct sibling declarations: a promoted cross-package
// struct's refs, and an embedded sibling-file struct's qualified refs.
func TestSiblingFileRefSites(t *testing.T) {
	t.Run("promoted cross-package embed", func(t *testing.T) {
		a := analyzeDirectiveProject(t, map[string]string{
			"go.mod": resolveGoMod,
			filepath.Join("other", "base.go"): "package other\n\n" +
				"type Base struct {\n\tIn Inner `json:\"in\"`\n\tIns []Inner `json:\"ins\"`\n}\n\ntype Inner struct{ X int `json:\"x\"` }\n",
			filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/other\"\n", "", "\tother.Base\n"),
		})
		got := renders(t, a.typeRegistry["Rows"])
		assert.Equal(t, "$Inner", got["in"])
		assert.Equal(t, "[]$Inner", got["ins"])
		fieldByJSON(t, a.typeRegistry["Inner"], "x")
		assert.Empty(t, a.Warnings(t.Context()))
	})

	t.Run("embedded sibling-file struct", func(t *testing.T) {
		a := analyzeDirectiveProject(t, map[string]string{
			"go.mod":                           resolveGoMod,
			filepath.Join("models", "item.go"): "package models\n\ntype Item struct{ ID int `json:\"id\"` }\n",
			filepath.Join("mod", "types.go"): "package mod\n\nimport m \"github.com/example/app/models\"\n\n" +
				"type Resp struct {\n\tItems []m.Item `json:\"items\"`\n}\n",
			filepath.Join("mod", "module.go"): rowsModule("", "", "\tResp\n"),
		})
		got := renders(t, a.typeRegistry["Rows"])
		assert.Equal(t, "[]$Item", got["items"])
		assert.Contains(t, a.typeRegistry, "Item")
		assert.Empty(t, a.Warnings(t.Context()))
	})
}

// TestRegisterRefLeafDemotion pins that a ShapeRef leaf that cannot register
// is demoted to a named (object) leaf with one warning, unless the depth cap
// truncated it (which has its own warning). The analyzer never runs
// AnalyzeProject, which also pins that New allocates the dedupe maps.
func TestRegisterRefLeafDemotion(t *testing.T) {
	missing := func() *models.FieldInfo {
		leaf := models.TypeShape{Kind: models.ShapeRef, Name: "Missing"}
		res := models.TypeShape{Kind: models.ShapeSlice, Elem: &leaf}
		return &models.FieldInfo{Name: "Items", Resolution: &res}
	}
	parsed := func(t *testing.T) (*ProjectAnalyzer, *ast.File, string) {
		t.Helper()
		dir := t.TempDir()
		a := New(dir)
		file, path := parseInDir(t, a, dir, "x.go", "package x\n")
		return a, file, path
	}
	unregistered := func(a *ProjectAnalyzer) []string {
		var out []string
		for _, w := range a.warnings {
			if strings.Contains(w, "could not be registered") {
				out = append(out, w)
			}
		}
		return out
	}

	t.Run("with a site", func(t *testing.T) {
		a, file, path := parsed(t)
		f := missing()
		a.fieldSites[f.Resolution] = fieldSite{loc: "x.go:3:2", declared: "[]Missing"}
		a.registerFieldRefAt(f, file, path, 1)
		assert.Equal(t, models.ShapeNamed, f.Resolution.Elem.Kind)
		assert.Equal(t, "Missing", f.Resolution.Elem.Name)
		w := unregistered(a)
		require.Len(t, w, 1)
		assert.Contains(t, w[0], "field Items at x.go:3:2 has type []Missing: struct Missing could not be registered")

		g := missing()
		a.fieldSites[g.Resolution] = fieldSite{loc: "x.go:3:2"}
		a.registerFieldRefAt(g, file, path, 1)
		assert.Len(t, unregistered(a), 1, "a second field at the same position does not warn again")
	})

	t.Run("without a site", func(t *testing.T) {
		a, file, path := parsed(t)
		f := missing()
		a.registerFieldRefAt(f, file, path, 1)
		assert.Equal(t, models.ShapeNamed, f.Resolution.Elem.Kind)
		w := unregistered(a)
		require.Len(t, w, 1)
		assert.Contains(t, w[0], "unknown position")
	})

	t.Run("past the depth cap", func(t *testing.T) {
		a, file, path := parsed(t)
		f := missing()
		a.fieldSites[f.Resolution] = fieldSite{loc: "x.go:3:2"}
		a.registerFieldRefAt(f, file, path, maxTypeRegistrationDepth+1)
		assert.Equal(t, models.ShapeNamed, f.Resolution.Elem.Kind)
		assert.Empty(t, unregistered(a))
		assert.True(t, a.depthWarned)
	})
}

// TestMarshalerUnderlyingNotRegistered pins that a struct reachable only
// under a Marshaler type is never registered (no orphan component).
func TestMarshalerUnderlyingNotRegistered(t *testing.T) {
	decls := "type User struct{ N int `json:\"n\"` }\ntype Users []User\nfunc (Users) MarshalJSON() ([]byte, error) { return nil, nil }\n"
	a, got := analyzeRows(t, "", decls, "\tU Users `json:\"u\"`\n")
	assert.Equal(t, "marshal:Users([]$User)", got["u"])
	assert.NotContains(t, a.typeRegistry, "User")
}

// TestFieldFallbackWarnsOncePerDeclaration pins the dedupe: a declaration
// reached from several routes and embeddings warns once, and its warning
// names the type and the method.
func TestFieldFallbackWarnsOncePerDeclaration(t *testing.T) {
	src := "package mod\n\nimport (\n\t\"github.com/gaborage/go-bricks/app\"\n\t\"github.com/gaborage/go-bricks/server\"\n)\n\n" +
		"type Module struct{}\n\n" +
		"func (m *Module) Name() string                    { return \"mod\" }\n" +
		"func (m *Module) Init(deps *app.ModuleDeps) error { return nil }\n" +
		"func (m *Module) Shutdown() error                 { return nil }\n" + `
type Status int
func (s Status) MarshalText() ([]byte, error) { return nil, nil }

type Inner struct {
	A, B Status
	C    Status
}
type E1 struct{ Inner }
type E2 struct{ Inner }

func (m *Module) e1(ctx server.HandlerContext) (server.Result[E1], server.IAPIError) { return server.Result[E1]{}, nil }
func (m *Module) e2(ctx server.HandlerContext) (server.Result[E2], server.IAPIError) { return server.Result[E2]{}, nil }
func (m *Module) in(ctx server.HandlerContext) (server.Result[Inner], server.IAPIError) {
	return server.Result[Inner]{}, nil
}
func (m *Module) in2(ctx server.HandlerContext) (server.Result[Inner], server.IAPIError) {
	return server.Result[Inner]{}, nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/e1", m.e1)
	server.GET(hr, r, "/e2", m.e2)
	server.GET(hr, r, "/in", m.in)
	server.GET(hr, r, "/in2", m.in2)
}
`
	a, _ := analyzeSingleModule(t, src)
	require.Len(t, a.warnings, 2, "one per declaration: %v", a.warnings)
	for _, w := range a.warnings {
		assert.Contains(t, w, "has type Status: Status has its own")
		assert.Contains(t, w, methodMarshalText)
	}
	assert.Len(t, fieldWarnings(a, "A"), 1)
	assert.Len(t, fieldWarnings(a, "C"), 1)
}

// TestShadowedPromotedFieldNeverWarns pins that field warnings wait for the
// merge: a promoted field shadowed by an outer one never reaches the document,
// so it raises no warning (and cannot fail --strict); a promoted field that
// survives still warns, and so does the embedded struct's own field when that
// struct is also a component of its own.
func TestShadowedPromotedFieldNeverWarns(t *testing.T) {
	decls := "type Status string\nfunc (Status) MarshalText() ([]byte, error) { return nil, nil }\n" +
		"type Base struct {\n\tAmount decimal.Decimal `json:\"amount\"`\n\tState Status `json:\"state\"`\n" +
		"\tAddr uintptr `json:\"addr\"`\n\tKeep decimal.Decimal `json:\"keep\"`\n}\n"
	shadowed := "\tBase\n\tAmount string `json:\"amount\"`\n\tState string `json:\"state\"`\n\tAddr string `json:\"addr\"`\n"

	a, got := analyzeRows(t, "\t\"github.com/shopspring/decimal\"\n", decls, shadowed)
	assert.Equal(t, "string", got["amount"])
	for _, name := range []string{"Amount", "State", "Addr"} {
		assert.Empty(t, fieldWarnings(a, name), name)
	}
	assert.Len(t, fieldWarnings(a, "Keep"), 1, "a surviving promoted field still warns")
	assert.Len(t, a.warnings, 1, "%v", a.warnings)

	a, _ = analyzeRows(t, "\t\"github.com/shopspring/decimal\"\n", decls, shadowed+"\tB Base `json:\"b\"`\n")
	for _, name := range []string{"Amount", "State", "Addr", "Keep"} {
		assert.Len(t, fieldWarnings(a, name), 1, "Base as its own component warns: %s", name)
	}
	assert.Len(t, a.warnings, 4, "%v", a.warnings)
}

// TestUintptrOutranksFallback pins the warning precedence: a Resolution that
// holds a uintptr leaf gets only the uintptr warning, whatever fallback was
// noted; a uintptr under a Marshaler leaf is not a uintptr leaf.
func TestUintptrOutranksFallback(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	file, _ := parseInDir(t, a, dir, "x.go", "package x\n\ntype S struct {\n\tP uintptr\n}\n")
	field := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType).Fields.List[0]
	leaf := models.TypeShape{Kind: models.ShapePrimitive, Name: goTypeUintptr}
	f := &models.FieldInfo{Name: "P", Resolution: &models.TypeShape{Kind: models.ShapeMap, Elem: &leaf}}
	a.fieldSites[f.Resolution] = fieldSite{loc: a.fieldLoc(field), first: fieldFallback{kind: fallbackMarshaler, typeName: "Status", detail: methodMarshalText}}
	a.warnResolvedField(f)
	a.warnResolvedField(f)                                               // the same declaration again: deduped
	a.warnResolvedField(&models.FieldInfo{Name: "Q"})                    // no Resolution, no site: never warns
	a.warnResolvedField(&models.FieldInfo{Name: "R", Resolution: &leaf}) // a hand-built field with no site: never warns
	require.Len(t, a.warnings, 1)
	assert.Contains(t, a.warnings[0], "holds a uintptr")

	_, got := analyzeRows(t, "", "type P uintptr\nfunc (P) MarshalJSON() ([]byte, error) { return nil, nil }\n", "\tP P `json:\"p\"`\n")
	assert.Equal(t, "marshal:P(uintptr)", got["p"])
}

// TestUintptrWrapperAndMapValueWarn pins that the uintptr warning covers a
// named wrapper and a map value, and that no fallback warning joins it.
func TestUintptrWrapperAndMapValueWarn(t *testing.T) {
	rows := []resolveRow{
		{"a", "Addr", "uintptr"},
		{"m", "map[string]uintptr", "map[string]uintptr"},
		{"ma", "map[string]Addr", "map[string]uintptr"},
		{"sa", "[]Addr", "[]uintptr"},
	}
	a, got := analyzeRows(t, "", "type Addr uintptr\n", rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
		w := fieldWarnings(a, exportName(r.json))
		if assert.Len(t, w, 1, r.json) {
			assert.Contains(t, w[0], "holds a uintptr")
		}
	}
	assert.Len(t, a.warnings, len(rows))
}

// TestUnresolvableFieldsWarn pins the unresolvable and untyped-builtin
// warnings, and that ShapeUnknown fields never warn.
func TestUnresolvableFieldsWarn(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("types", "cents.go"): "package types\n\ntype Cents int64\n",
		filepath.Join("mod", "module.go"): rowsModule(
			"\tt \"time\"\n\tj \"encoding/json\"\n\t\"github.com/google/uuid\"\n\t\"github.com/shopspring/decimal\"\n\t\"github.com/example/app/types\"\n",
			"type StampD t.Time\ntype IDD uuid.UUID\ntype C complex128\n",
			"\tDec decimal.Decimal `json:\"dec\"`\n\tAliasedTime t.Time `json:\"aliasedTime\"`\n\tAliasedRaw j.RawMessage `json:\"aliasedRaw\"`\n"+
				"\tCents types.Cents `json:\"cents\"`\n\tStampD StampD `json:\"stampD\"`\n\tIDD IDD `json:\"idd\"`\n\tMissing Missing `json:\"missing\"`\n"+
				"\tQ decimal.Decimal `query:\"q\"`\n"+
				"\tCh chan int `json:\"ch\"`\n\tAnon struct{ X int } `json:\"anon\"`\n\tPage Page[int] `json:\"page\"`\n"+
				"\tErr error `json:\"err\"`\n\tC64 complex64 `json:\"c64\"`\n\tC128 complex128 `json:\"c128\"`\n\tCW C `json:\"cw\"`\n\tCM map[string]complex128 `json:\"cm\"`\n"),
	})
	for _, name := range []string{"Dec", "AliasedTime", "AliasedRaw", "Cents", "StampD", "IDD", "Missing", "Q"} {
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], "resolves to no schema", name)
		}
	}
	for _, name := range []string{"Ch", "Anon", "Page"} {
		assert.Empty(t, fieldWarnings(a, name), name)
	}
	builtins := map[string][2]string{
		"Err": {frameworkTypeError, frameworkTypeError}, "C64": {goTypeComplex64, goTypeComplex64},
		"C128": {goTypeComplex128, goTypeComplex128}, "CW": {"C", goTypeComplex128}, "CM": {"map[string]complex128", goTypeComplex128},
	}
	for name, tb := range builtins {
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], "has type "+tb[0]+",", name)
			assert.Contains(t, w[0], "holds the builtin "+tb[1]+",", name)
		}
	}
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "error", got["err"])
	assert.Equal(t, goTypeComplex64, got["c64"])
	assert.Equal(t, goTypeComplex128, got["c128"])
	assert.Equal(t, goTypeComplex128, got["cw"])
	assert.Equal(t, "map[string]complex128", got["cm"])
	assert.Equal(t, "types.Cents", got["cents"])
}

// TestWarnedRowsInEveryPosition pins each warned row in all seven positions:
// the render is the position around the direct render, and exactly one
// warning of the row's kind names the field.
func TestWarnedRowsInEveryPosition(t *testing.T) {
	reps := []struct{ goType, direct, warning string }{
		{"Status", "marshal:Status(int)", "encoding/json uses instead"},
		{"TagsM", "marshal:TagsM([]string)", "encoding/json uses instead"},
		{"Tree", "map[string]cycle:Tree", "contains itself"},
		{"decimal.Decimal", "decimal.Decimal", "resolves to no schema"},
		{"t.Time", "t.Time", "resolves to no schema"},
		{"StampD", "StampD", "resolves to no schema"},
		{"C", goTypeComplex128, "holds the builtin"},
		{"Addr", "uintptr", "holds a uintptr"},
	}
	positions := []string{"%s", "*%s", "[]%s", "map[string]%s", "[][]%s", "map[string][]%s", "[]map[string]%s"}
	var body strings.Builder
	for i, r := range reps {
		for j, p := range positions {
			fmt.Fprintf(&body, "\tF%dP%d %s\n", i, j, fmt.Sprintf(p, r.goType))
		}
	}
	decls := "type Status int\nfunc (s Status) MarshalText() ([]byte, error) { return nil, nil }\n" +
		"type TagsM []string\nfunc (TagsM) MarshalJSON() ([]byte, error) { return nil, nil }\n" +
		"type Tree map[string]Tree\ntype StampD time.Time\ntype C complex128\ntype Addr uintptr\n"
	a, got := analyzeRows(t, "\t\"time\"\n\tt \"time\"\n\t\"github.com/shopspring/decimal\"\n", decls, body.String())
	for i, r := range reps {
		for j, p := range positions {
			name := fmt.Sprintf("F%dP%d", i, j)
			assert.Equal(t, fmt.Sprintf(p, r.direct), got[name], name)
			if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
				assert.Contains(t, w[0], r.warning, name)
			}
		}
	}
}

func prim(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePrimitive, Name: name}
}

func wrap(kind models.ShapeKind, elem models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: kind, Elem: &elem}
}

// TestMergeShapes pins the build-tagged declaration merge.
func TestMergeShapes(t *testing.T) {
	ref := models.TypeShape{Kind: models.ShapeRef, Name: "User"}
	cases := []struct {
		name string
		x, y models.TypeShape
		want string // "" = no merge
	}{
		{"equal", prim(goTypeInt64), prim(goTypeInt64), "int64"},
		{"byte/uint8", prim(goTypeByte), prim(goTypeUint8), "byte"},
		{"rune/int32", prim(goTypeRune), prim(goTypeInt32), "rune"},
		{"widths", prim(goTypeInt64), prim(goTypeInt32), "kind:integer"},
		{"kind-only vs primitive", models.TypeShape{Kind: models.ShapeKindOnly, Name: kindInteger}, prim(goTypeInt), "kind:integer"},
		{"at depth", wrap(models.ShapeSlice, prim(goTypeInt64)), wrap(models.ShapeSlice, prim(goTypeInt32)), "[]kind:integer"},
		{"container mismatch", wrap(models.ShapeSlice, prim(goTypeInt64)), wrap(models.ShapeArray, prim(goTypeInt64)), ""},
		{"kinds differ", prim(goTypeInt64), prim(goTypeString), ""},
		{"leaf vs container", prim(goTypeInt64), wrap(models.ShapeSlice, prim(goTypeInt64)), ""},
		{"same ref", ref, ref, "$User"},
		{"refs differ", ref, models.TypeShape{Kind: models.ShapeRef, Name: "Other"}, ""},
		{"nil elems", models.TypeShape{Kind: models.ShapeSlice}, models.TypeShape{Kind: models.ShapeSlice}, "[]unknown"},
		{"one nil elem", models.TypeShape{Kind: models.ShapeSlice}, wrap(models.ShapeSlice, prim(goTypeInt64)), ""},
	}
	for _, tc := range cases {
		got, ok := mergeShapes(tc.x, tc.y)
		if tc.want == "" {
			assert.False(t, ok, tc.name)
			continue
		}
		if assert.True(t, ok, tc.name) {
			assert.Equal(t, tc.want, renderShape(got), tc.name)
		}
	}
}

// TestKindBackedUnderlying pins the well-known names a defined chain may end
// in: the kind-backed ones resolve, the struct-backed ones do not.
func TestKindBackedUnderlying(t *testing.T) {
	cases := map[string]string{
		models.WellKnownTimeDuration: "int64",
		models.WellKnownTimeMonth:    "int",
		models.WellKnownTimeWeekday:  "int",
		models.WellKnownRawMessage:   "[]byte",
		models.WellKnownJSONNumber:   "string",
	}
	for name, want := range cases {
		got, ok := kindBackedUnderlying(name)
		if assert.True(t, ok, name) {
			assert.Equal(t, want, renderShape(got), name)
		}
	}
	for _, name := range []string{models.WellKnownTimeTime, models.WellKnownUUID, "decimal.Decimal"} {
		_, ok := kindBackedUnderlying(name)
		assert.False(t, ok, name)
	}
}

// TestPromotedFieldIgnoresEmbeddingPackageNames pins #101: a field promoted
// from another package resolves and registers in that package, so a struct in
// the embedding package that shares its type's name is never captured.
func TestPromotedFieldIgnoresEmbeddingPackageNames(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("other", "base.go"): "package other\n\ntype Code string\n\ntype Inner struct{ X int `json:\"x\"` }\n\n" +
			"type Base struct {\n\tC Code `json:\"c\"`\n\tIn Inner `json:\"in\"`\n}\n",
		filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/other\"\n",
			"type Code struct{ V int `json:\"v\"` }\n\ntype Inner struct{ Z bool `json:\"z\"` }\n",
			"\tother.Base\n"),
	})
	rows := a.typeRegistry["Rows"]
	assert.Equal(t, "string", renderShape(fieldByJSON(t, rows, "c").ResolvedShape()))
	in := fieldByJSON(t, rows, "in").ResolvedShape()
	require.Equal(t, models.ShapeRef, in.Kind)
	inner := a.typeRegistry[in.Name]
	require.NotNil(t, inner)
	require.Len(t, inner.Fields, 1)
	assert.Equal(t, "x", inner.Fields[0].JSONName)
	for name, ti := range a.typeRegistry {
		for i := range ti.Fields {
			assert.NotContains(t, []string{"v", "z"}, ti.Fields[i].JSONName, "%s registers the embedding package's struct", name)
		}
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestResolveDegenerateShapes pins the resolver on inputs no decoder emits but
// a hand-built shape can carry: a container with no element stays as is, and
// a nil Resolution holds no uintptr.
func TestResolveDegenerateShapes(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "x.go", "package x\n")
	for _, kind := range []models.ShapeKind{models.ShapePointer, models.ShapeSlice, models.ShapeArray, models.ShapeMap} {
		got := a.resolveShape(models.TypeShape{Kind: kind}, newResolveCtx(file, path, false))
		assert.Nil(t, got.Elem, kind)
	}
	assert.False(t, holdsUintptr(nil))
	assert.False(t, holdsUintptr(&models.TypeShape{Kind: models.ShapeSlice}))
}

// TestResolveDeclsWithNoCommonShape pins the last merge fallback: build-tagged
// declarations that neither merge nor include a scalar leave the type
// unresolvable, noted as a disagreement.
func TestResolveDeclsWithNoCommonShape(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "w_a.go"), []byte("package x\n\ntype W []int\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "w_b.go"), []byte("package x\n\ntype W map[string]int\n"), 0o600))
	a := New(dir)
	file, path := parseInDir(t, a, dir, "x.go", "package x\n")
	c := newResolveCtx(file, path, false)
	got := a.resolveLocal(models.TypeShape{Kind: models.ShapeNamed, Name: "W"}, c, identityMode)
	assert.Equal(t, "W", renderShape(got))
	assert.Equal(t, fieldFallback{kind: fallbackDeclsDisagree, typeName: "W"}, c.st.first)

	_, ok := mergeShapes(wrap(models.ShapeSlice, prim(goTypeInt64)), wrap(models.ShapeSlice, prim(goTypeString)))
	assert.False(t, ok, "elements of one container kind that do not merge")
}

// TestFieldWarningsNameDeclaredType pins that a fallback warning names the
// type the field declares, as written, then the named type where resolution
// stopped: a defined type over an alias of a well-known struct is named, not
// the alias it passed through, and a wrapper position shows in the declared
// type.
func TestFieldWarningsNameDeclaredType(t *testing.T) {
	decls := "type TA = time.Time\ntype D TA\ntype Tree map[string]Tree\n"
	rows := "\tD D `json:\"d\"`\n\tDS []D `json:\"ds\"`\n\tTS []Tree `json:\"ts\"`\n" +
		"\tDec *decimal.Decimal `json:\"dec\"`\n\tMissing map[string]Missing `json:\"missing\"`\n"
	a, got := analyzeRows(t, "\t\"time\"\n\t\"github.com/shopspring/decimal\"\n", decls, rows)
	assert.Equal(t, "TA", got["d"], "the chain stops at the alias; the leaf emits an untyped object")
	assert.Equal(t, "[]TA", got["ds"])
	want := map[string]string{
		"D":       "has type D: D is a defined type over time.Time, which resolves to no schema",
		"DS":      "has type []D: D is a defined type over time.Time, which resolves to no schema",
		"TS":      "has type []Tree: Tree contains itself",
		"Dec":     "has type *decimal.Decimal: decimal.Decimal resolves to no schema",
		"Missing": "has type map[string]Missing: Missing resolves to no schema",
	}
	for name, sub := range want {
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], sub, name)
		}
	}
	for _, name := range []string{"D", "DS"} {
		assert.NotContains(t, fieldWarnings(a, name)[0], "TA", "the alias passed through is not named")
	}
}

// TestBuildTaggedDisagreementWarns pins the warning for build-tagged
// declarations that disagree on shape (w_linux.go's []string against
// w_darwin.go's []int): it gives that reason, not the generic unresolvable
// one, and names the field's declared type.
func TestBuildTaggedDisagreementWarns(t *testing.T) {
	variant := func(constraint, decl string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\n" + decl + "\n"
	}
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                            resolveGoMod,
		filepath.Join("mod", "module.go"):   rowsModule("", "", "\tW W `json:\"w\"`\n\tWS []W `json:\"ws\"`\n"),
		filepath.Join("mod", "w_linux.go"):  variant("linux", "type W []string"),
		filepath.Join("mod", "w_darwin.go"): variant("darwin", "type W []int"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "W", got["w"])
	assert.Equal(t, "[]W", got["ws"])
	for name, declared := range map[string]string{"W": "W", "WS": "[]W"} {
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], "has type "+declared+": W's build-tagged declarations disagree on shape", name)
			assert.NotContains(t, w[0], "third-party", name)
		}
	}
}
