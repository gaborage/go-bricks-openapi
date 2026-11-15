package analyzer

import (
	"go/ast"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Issue 114: an Instantiation's type arguments are substituted into its
// generic declaration, and each distinct struct Instantiation is one
// component named by ADR 0003's rules.

// instBillingSrc is the in-module package billing: a named non-struct, a
// struct Marshaler, an alias of time.Time and a second generic Page.
const instBillingSrc = `package billing

import "time"

type Cents int64

type Money struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

type Timestamp = time.Time

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}
`

// instModSrc is a module whose one route returns Rows, one field per naming,
// sharing and identity row of ADR 0003.
const instModSrc = `package mod

import (
	t "time"
	"time"

	"github.com/example/app/billing"
	ma "github.com/example/app/modelsa"
	mb "github.com/example/app/modelsb"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

type User struct {
	Name string ` + "`json:\"name\"`" + `
}

type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
	Total int ` + "`json:\"total\"`" + `
}

type Pair[K comparable, V any] struct {
	Key K ` + "`json:\"key\"`" + `
	Val V ` + "`json:\"val\"`" + `
}

type Node[T any] struct {
	Val  T        ` + "`json:\"val\"`" + `
	Next *Node[T] ` + "`json:\"next\"`" + `
}

type Wrap[T any] struct {
	Inner []Box[T]               ` + "`json:\"inner\"`" + `
	Anon  struct{ W T ` + "`json:\"w\"`" + ` } ` + "`json:\"anon\"`" + `
	Page[T]
}

type AliasPage = Page[Address]
type AddrPage Page[Address]

type T struct {
	Z int ` + "`json:\"z\"`" + `
}

type TList []T

type Shadow[T any] struct {
	L TList ` + "`json:\"l\"`" + `
	V T     ` + "`json:\"v\"`" + `
}

type Rows struct {
	P1  Page[Address]                                    ` + "`json:\"p1\"`" + `
	P2  Page[Address]                                    ` + "`json:\"p2\"`" + `
	PU  Page[User]                                       ` + "`json:\"pu\"`" + `
	PR  Pair[string, int]                                ` + "`json:\"pr\"`" + `
	PB  Page[Box[Address]]                               ` + "`json:\"pb\"`" + `
	BP  Box[*Address]                                    ` + "`json:\"bp\"`" + `
	BS  Box[[]Address]                                   ` + "`json:\"bs\"`" + `
	BM  Box[map[string]int]                              ` + "`json:\"bm\"`" + `
	BT  Box[time.Time]                                   ` + "`json:\"bt\"`" + `
	BTA Box[t.Time]                                      ` + "`json:\"bta\"`" + `
	BTS Box[billing.Timestamp]                           ` + "`json:\"bts\"`" + `
	BU  Box[uuid.UUID]                                   ` + "`json:\"bu\"`" + `
	BD  Box[decimal.Decimal]                             ` + "`json:\"bd\"`" + `
	BC  Box[billing.Cents]                               ` + "`json:\"bc\"`" + `
	BMo Box[billing.Money]                               ` + "`json:\"bmo\"`" + `
	BA  Box[any]                                         ` + "`json:\"ba\"`" + `
	BI  Box[interface{}]                                 ` + "`json:\"bi\"`" + `
	B3  Box[[3]int]                                      ` + "`json:\"b3\"`" + `
	B5  Box[[5]int]                                      ` + "`json:\"b5\"`" + `
	BX  Box[struct{ X int ` + "`json:\"x\"`" + ` }] ` + "`json:\"bx\"`" + `
	IDA Box[ma.ID]                                       ` + "`json:\"ida\"`" + `
	IDB Box[mb.ID]                                       ` + "`json:\"idb\"`" + `
	BPG billing.Page[Address]                            ` + "`json:\"bpg\"`" + `
	AP  AliasPage                                        ` + "`json:\"ap\"`" + `
	DP  AddrPage                                         ` + "`json:\"dp\"`" + `
	N   Node[int]                                        ` + "`json:\"n\"`" + `
	W   Wrap[User]                                       ` + "`json:\"w\"`" + `
	SH  Shadow[int]                                      ` + "`json:\"sh\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}
`

// analyzeInstRows analyzes instModSrc with its in-module packages.
func analyzeInstRows(t *testing.T) *ProjectAnalyzer {
	t.Helper()
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                              resolveGoMod,
		filepath.Join("billing", "b.go"):      instBillingSrc,
		filepath.Join("modelsa", "models.go"): "package models\n\ntype ID string\n",
		filepath.Join("modelsb", "models.go"): "package models\n\ntype ID int64\n",
		filepath.Join("mod", "module.go"):     instModSrc,
	})
	require.Len(t, routes, 1)
	return a
}

// TestInstantiationComponentNames pins ADR 0003's spelling rules, row by row.
func TestInstantiationComponentNames(t *testing.T) {
	a := analyzeInstRows(t)
	rows := renders(t, a.typeRegistry["Rows"])
	for json, want := range map[string]string{
		"p1":  "$Page-Address",
		"pu":  "$Page-User",
		"pr":  "$Pair-string-int",
		"pb":  "$Page-Box-Address",
		"bp":  "$Box-ptr-Address",
		"bs":  "$Box-slice-Address",
		"bm":  "$Box-map-string-int",
		"bt":  "$Box-time.Time",
		"bu":  "$Box-uuid.UUID",
		"bd":  "$Box-decimal.Decimal",
		"bc":  "$Box-billing.Cents",
		"bmo": "$Box-billing.Money",
		"ba":  "$Box-any",
		"b3":  "$Box-array-int",
		"bx":  "struct{v struct{x int}}",
		"ida": "$Box-models.ID",
		"idb": "$ModBox-models.ID",
		"bpg": "$BillingPage-Address",
		"ap":  "$Page-Address",
		"dp":  "$AddrPage",
		"n":   "$Node-int",
		"w":   "$Wrap-User",
	} {
		assert.Equal(t, want, rows[json], json)
	}
	for _, name := range []string{"Page", "Box", "Pair", "AliasPage", "Node", "Wrap"} {
		assert.NotContains(t, a.typeRegistry, name, "no component under a generic base's bare name, nor an alias of an Instantiation")
	}
}

// TestInstantiationSharing pins identity: identical arguments share one
// component, whatever their spelling (an aliased import, an alias of the
// type, any for interface{}, any array length), and distinct ones never do.
func TestInstantiationSharing(t *testing.T) {
	a := analyzeInstRows(t)
	rows := renders(t, a.typeRegistry["Rows"])
	for _, same := range [][]string{{"p1", "p2", "ap"}, {"bt", "bta", "bts"}, {"ba", "bi"}, {"b3", "b5"}} {
		for _, json := range same[1:] {
			assert.Equal(t, rows[same[0]], rows[json], "%s and %s are one Instantiation", same[0], json)
		}
	}
	assert.NotEqual(t, rows["p1"], rows["pu"])
	assert.NotContains(t, a.typeRegistry, "Box-Time")
}

// TestInstantiationSubstitutedSchemas pins the substituted fields: a
// well-known argument resolves through its import even under an alias, an
// unresolvable one falls back in its fields only (with the field warning),
// any is {} with no warning, a struct Marshaler argument is a Marshaler leaf,
// the colliding ids keep their own kinds, a recursive generic struct refs
// its own component, and a generic struct's anonymous struct, its embedded
// generic and its nested Instantiation all resolve under its bindings.
func TestInstantiationSubstitutedSchemas(t *testing.T) {
	a := analyzeInstRows(t)
	v := func(component string) string { return renders(t, a.typeRegistry[component])["v"] }
	assert.Equal(t, models.WellKnownTimeTime, v("Box-time.Time"))
	assert.Equal(t, "decimal.Decimal", v("Box-decimal.Decimal"))
	assert.Equal(t, "any", v("Box-any"))
	assert.Equal(t, "marshal:Money(unknown)", v("Box-billing.Money"))
	assert.Equal(t, goTypeInt64, v("Box-billing.Cents"))
	assert.Equal(t, goTypeString, v("Box-models.ID"))
	assert.Equal(t, goTypeInt64, v("ModBox-models.ID"))
	assert.Equal(t, "*$Address", v("Box-ptr-Address"))
	assert.Equal(t, "[N]int", v("Box-array-int"))
	assert.Equal(t, map[string]string{"items": "[]$Box-Address", "total": goTypeInt}, renders(t, a.typeRegistry["Page-Box-Address"]))
	assert.Equal(t, map[string]string{"items": "[]$Address", "total": goTypeInt}, renders(t, a.typeRegistry["AddrPage"]))
	assert.Equal(t, map[string]string{"val": goTypeInt, "next": "*$Node-int"}, renders(t, a.typeRegistry["Node-int"]))
	assert.Equal(t, map[string]string{"inner": "[]$Box-User", "anon": "struct{w $User}", "items": "[]$User", "total": goTypeInt},
		renders(t, a.typeRegistry["Wrap-User"]))
	// A type parameter is bound only inside its own declaration: TList's T is
	// the module's type T, not Shadow's parameter.
	assert.Equal(t, map[string]string{"l": "[]$T", "v": goTypeInt}, renders(t, a.typeRegistry["Shadow-int"]))

	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 1, "%v", warnings)
	assert.Len(t, matching(warnings, "has type T: decimal.Decimal resolves to no schema"), 1, "%v", warnings)
	assert.Len(t, matching(warnings, "has type T: billing.Money has its own MarshalJSON method"), 0,
		"a generic struct's field warns once per declaration, the first Instantiation's: %v", warnings)
	assert.Len(t, matching(warnings, "struct{ X int"), 0, "%v", warnings)
}

// TestInstantiationIdentity pins argIdentity: what makes two arguments one
// type, independent of spelling.
func TestInstantiationIdentity(t *testing.T) {
	a := analyzeInstRows(t)
	file, path := instModFile(t, a)
	at := pkgFile{file: file, path: path}
	id := func(src string) string {
		return a.argIdentity(a.typeShape(mustParse(t, src)), at, nil, 0)
	}
	assert.Equal(t, id("time.Time"), id("t.Time"))
	assert.Equal(t, id("time.Time"), id("billing.Timestamp"))
	assert.Equal(t, "any", id("interface{}"))
	assert.Equal(t, id("[3]int"), id("[5]int"))
	assert.NotEqual(t, id("ma.ID"), id("mb.ID"))
	assert.NotEqual(t, id("*Address"), id("[]Address"))
	assert.Equal(t, "map[string]int", id("map[string]int"))
	assert.Equal(t, "Undeclared[int]", id("Undeclared[int]")[len(id("Undeclared[int]"))-len("Undeclared[int]"):])
	assert.Equal(t, "q.Unknown", id("q.Unknown"), "a qualifier no import binds keeps its spelling")
	assert.Equal(t, identityCap, a.argIdentity(a.typeShape(mustParse(t, "int")), at, nil, maxNamedResolutionDepth+1))
	assert.Equal(t, "unknown:func", id("func()"))
	bound := typeEnv{"T": {shape: a.typeShape(mustParse(t, "t.Time")), site: at}}
	assert.Equal(t, id("time.Time"), a.argIdentity(a.typeShape(mustParse(t, "T")), at, bound, 0), "a type parameter is its argument")
}

// TestInstantiationSpelling pins spellArg's rows that the component names do
// not reach: an unknown qualifier, an undeclared name, an unmodelled leaf
// and a non-struct Instantiation, spelled through its generic type.
func TestInstantiationSpelling(t *testing.T) {
	a := analyzeInstRows(t)
	file, path := instModFile(t, a)
	at := pkgFile{file: file, path: path}
	spell := func(src string) string { return a.spellArg(a.typeShape(mustParse(t, src)), at, nil, 0) }
	assert.Equal(t, "q.Unknown", spell("q.Unknown"))
	assert.Equal(t, "mod.Missing", spell("Missing"))
	assert.Equal(t, "unknown", spell("func()"))
	assert.Equal(t, "Gone-int", spell("Gone[int]"))
	assert.Equal(t, "Undeclared-slice-int", spell("q.Undeclared[[]int]"))
	assert.Equal(t, "map-string-time.Time", spell("map[string]t.Time"))
}

// instModFile returns the parsed module file of an analyzeInstRows project.
func instModFile(t *testing.T, a *ProjectAnalyzer) (file *ast.File, path string) {
	t.Helper()
	for dir, files := range a.pkgCache {
		if filepath.Base(dir) != "mod" {
			continue
		}
		for p, f := range files {
			if strings.HasSuffix(p, "module.go") {
				return f, p
			}
		}
	}
	require.Fail(t, "module file not parsed")
	return nil, ""
}

// TestGenericBaseNameQualified pins genericBaseName: the first generic type
// of a name keeps it, another directory's is package-qualified, then
// numbered, and each keeps its name on every later call.
func TestGenericBaseNameQualified(t *testing.T) {
	a := New("")
	decl := func(dir, pkg string) genericDecl {
		return genericDecl{spec: &ast.TypeSpec{Name: ast.NewIdent("Page")}, site: pkgFile{file: &ast.File{Name: ast.NewIdent(pkg)}, path: filepath.Join(dir, "x.go")}}
	}
	assert.Equal(t, "Page", a.genericBaseName(decl("/a", "pg")))
	assert.Equal(t, "PgPage", a.genericBaseName(decl("/b", "pg")))
	assert.Equal(t, "PgPage2", a.genericBaseName(decl("/c", "pg")))
	assert.Equal(t, "PgPage", a.genericBaseName(decl("/b", "pg")))
}

// TestRegisterInstantiationDepthCap pins that an instance past
// maxTypeRegistrationDepth does not register, and its ref leaf is demoted to
// a named leaf spelled as written.
func TestRegisterInstantiationDepthCap(t *testing.T) {
	a := New("")
	inst := &instance{written: "Box[int]"}
	a.instances["ref"] = inst
	assert.Nil(t, a.registerInstantiation(inst, maxTypeRegistrationDepth+1))
	leaf := models.TypeShape{Kind: models.ShapeRef, Name: "ref"}
	assert.False(t, a.registerLeafAt(&leaf, pkgFile{}, maxTypeRegistrationDepth+1))
	assert.Equal(t, models.TypeShape{Kind: models.ShapeNamed, Name: "Box[int]"}, leaf)
	assert.True(t, a.depthWarned)
}

// TestBoundShapeFollowsParameters pins boundShape and holdsAnonymousArg:
// an argument naming an outer type parameter is followed to what was
// written for it.
func TestBoundShapeFollowsParameters(t *testing.T) {
	a := New("")
	lit := a.typeShape(mustParse(t, "struct{ X int }"))
	outer := typeEnv{"U": {shape: lit}}
	inner := typeEnv{"T": {shape: models.TypeShape{Kind: models.ShapeNamed, Name: "U"}, env: outer}}
	arg := inner["T"]
	assert.Equal(t, models.ShapeStruct, boundShape(&arg).Kind)
	assert.True(t, holdsAnonymousArg(inner))
	assert.False(t, holdsAnonymousArg(typeEnv{"T": {shape: models.TypeShape{Kind: models.ShapeNamed, Name: "U"}}}))
	assert.Equal(t, []string{"K", "V"}, typeParamNames(&ast.TypeSpec{TypeParams: &ast.FieldList{List: []*ast.Field{
		{Names: []*ast.Ident{ast.NewIdent("K"), ast.NewIdent("V")}},
	}}}))
	assert.Nil(t, typeParamNames(&ast.TypeSpec{}))
	assert.Nil(t, a.instanceOf(models.TypeShape{Kind: models.ShapeNamed, Name: "x"}))
}

// TestGenericMarshalerRequest pins a request whose generic struct holds a
// marshal method on its generic receiver: it is documented as a struct
// Marshaler request (#111), its body the leaf, {} with the request warning
// or a quiet string, and no component.
func TestGenericMarshalerRequest(t *testing.T) {
	run := analyzeGenericRoute(t, "req JBox[Address], ctx server.HandlerContext", "server.Result[Receipt]")
	assert.Equal(t, []string{"request type JBox[Address]: JBox has its own MarshalJSON method, which encoding/json uses instead of its " +
		"fields — emitting an untyped requestBody schema ({}); its path, query and header fields stay parameters"}, run.warnings)
	require.NotNil(t, run.route.Request)
	assert.Empty(t, run.route.Request.Name)
	assert.Equal(t, "marshal:JBox[Address](unknown)", renderShapePtr(run.route.Request.Resolution))

	run = analyzeGenericRoute(t, "req *TBox[Address], ctx server.HandlerContext", "server.Result[Receipt]")
	assert.Empty(t, run.warnings)
	require.NotNil(t, run.route.Request)
	assert.Equal(t, "text:TBox[Address]", renderShapePtr(run.route.Request.Resolution))
	assertNoGenericComponent(t, run.a)
}

// instDefinedSrc holds defined types over generic Marshaler types, and
// Instantiations whose arguments hold a func or chan.
const instDefinedSrc = `package mod

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type MJ[T any] []T

func (MJ[T]) MarshalText() ([]byte, error) { return nil, nil }
func (*MJ[T]) UnmarshalText([]byte) error  { return nil }

type TBox[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

func (TBox[T]) MarshalText() ([]byte, error) { return nil, nil }
func (*TBox[T]) UnmarshalText([]byte) error  { return nil }

type JBox[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

func (JBox[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

type DM MJ[int]
type GM[T any] MJ[T]
type AM = MJ[int]
type DA AM
type GAM[T any] = MJ[T]
type DX GAM[int]
type TD[T any] TBox[T]
type GD[T any] JBox[T]

type ED struct {
	DM
	K int ` + "`json:\"k\"`" + `
}

type Rows struct {
	MJ MJ[int]            ` + "`json:\"mj\"`" + `
	DM DM                 ` + "`json:\"dm\"`" + `
	GM GM[int]            ` + "`json:\"gm\"`" + `
	DA DA                 ` + "`json:\"da\"`" + `
	DX DX                 ` + "`json:\"dx\"`" + `
	SM []MJ[int]          ` + "`json:\"sm\"`" + `
	TD TD[int]            ` + "`json:\"td\"`" + `
	GD GD[int]            ` + "`json:\"gd\"`" + `
	ED ED                 ` + "`json:\"ed\"`" + `
	BC Box[chan int]      ` + "`json:\"bc\"`" + `
	BF Box[[]func()]      ` + "`json:\"bf\"`" + `
	PC Box[Box[chan int]] ` + "`json:\"pc\"`" + `
	JC JBox[chan int]     ` + "`json:\"jc\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}
`

// analyzeInstDefined analyzes instDefinedSrc.
func analyzeInstDefined(t *testing.T) *ProjectAnalyzer {
	t.Helper()
	a, routes := analyzeProjectRoutes(t, map[string]string{"go.mod": resolveGoMod, filepath.Join("mod", "module.go"): instDefinedSrc})
	require.Len(t, routes, 1)
	return a
}

// TestInstantiationDefinedOverMarshaler pins that a defined type over a
// generic Marshaler type's Instantiation drops its methods, as the Go spec
// says, directly (DM), generic (GM, TD, GD), through an alias (DA, DX) or
// embedded (ED), while the Instantiation itself, or one inside a container
// (SM), keeps them.
func TestInstantiationDefinedOverMarshaler(t *testing.T) {
	a := analyzeInstDefined(t)
	rows := renders(t, a.typeRegistry["Rows"])
	for json, want := range map[string]string{
		"mj": "text:MJ[int]",
		"dm": "[]int",
		"gm": "[]int",
		"da": "[]int",
		"dx": "[]int",
		"sm": "[]text:MJ[int]",
		"td": "$TBox-int",
		"gd": "$JBox-int",
		"ed": "$ED",
	} {
		assert.Equal(t, want, rows[json], json)
	}
	assert.Equal(t, map[string]string{"v": goTypeInt}, renders(t, a.typeRegistry["TBox-int"]))
	assert.Equal(t, map[string]string{"v": goTypeInt}, renders(t, a.typeRegistry["JBox-int"]))
	assert.Equal(t, map[string]string{"DM": "[]int", "k": goTypeInt}, renders(t, a.typeRegistry["ED"]))
	assert.Len(t, matching(a.Warnings(t.Context()), "MarshalJSON"), 1, "only JC's: %v", a.warnings)
}

// TestInstantiationFuncChanArgRefused pins that an Instantiation whose type
// arguments hold a func or chan, at any depth, is refused: no component,
// the field's func or chan warning, an untyped schema. A Marshaler generic
// type writes itself, so JBox[chan int] is its Marshaler leaf as before.
func TestInstantiationFuncChanArgRefused(t *testing.T) {
	a := analyzeInstDefined(t)
	rows := renders(t, a.typeRegistry["Rows"])
	for json, want := range map[string]string{"bc": "unknown", "bf": "unknown", "pc": "unknown"} {
		assert.Equal(t, want, rows[json], json)
	}
	for name := range a.typeRegistry {
		assert.NotContains(t, name, "unknown", "no component spells a func or chan argument")
		assert.NotContains(t, name, "Box-Box", "no component for an Instantiation holding a refused one")
	}
	warnings := a.Warnings(t.Context())
	for _, want := range []string{
		"has type Box[chan int], which holds a chan",
		"has type Box[[]func()], which holds a func",
		"has type Box[Box[chan int]], which holds a chan",
		"has type JBox[chan int]: JBox has its own MarshalJSON",
	} {
		assert.Len(t, matching(warnings, want), 1, "%q in %v", want, warnings)
	}
}

// instReviewSrc holds the review rows of issue 114: a decode-only byte-kind
// generic as a slice element, identities past the depth cap, a generic alias
// argument, one literal under two bindings, build-tagged copies of one
// generic struct under different arguments (W, in w_a.go and w_b.go) and a
// parenthesized pointer receiver.
const instReviewSrc = `package mod

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

type A struct {
	X int ` + "`json:\"x\"`" + `
}

type B struct {
	Y string ` + "`json:\"y\"`" + `
}

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}

type BY[T any] byte

func (b *BY[T]) UnmarshalJSON([]byte) error { return nil }

type BE[T any] byte

func (b BE[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type Alias[T any] = Box[T]

type List[T any] []struct {
	Value T ` + "`json:\"value\"`" + `
}

type PT[X any] struct {
	V X ` + "`json:\"v\"`" + `
}

func (r (*PT[X])) MarshalText() ([]byte, error) { return nil, nil }
func (r *PT[X]) UnmarshalText([]byte) error    { return nil }

type VT[X any] struct {
	V X ` + "`json:\"v\"`" + `
}

func (r (VT[X])) MarshalText() ([]byte, error) { return nil, nil }
func (r *VT[X]) UnmarshalText([]byte) error    { return nil }

type Rows struct {
	BY []BY[int]                        ` + "`json:\"by\"`" + `
	BE []BE[int]                        ` + "`json:\"be\"`" + `
	D1 Box[[][][][][][][][][][]int]     ` + "`json:\"d1\"`" + `
	D2 Box[[][][][][][][][][][]string]  ` + "`json:\"d2\"`" + `
	AB Box[Alias[int]]                  ` + "`json:\"ab\"`" + `
	BB Box[Box[int]]                    ` + "`json:\"bb\"`" + `
	LL List[List[int]]                  ` + "`json:\"ll\"`" + `
	W  W                                ` + "`json:\"w\"`" + `
	PT PT[int]                          ` + "`json:\"pt\"`" + `
	VT VT[int]                          ` + "`json:\"vt\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}
`

// analyzeInstReview analyzes instReviewSrc with W's two build-tagged copies.
func analyzeInstReview(t *testing.T) (a *ProjectAnalyzer, rows map[string]string) {
	t.Helper()
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("mod", "module.go"): instReviewSrc,
		filepath.Join("mod", "w_a.go"):    "//go:build linux\n\npackage mod\n\ntype W Page[A]\n",
		filepath.Join("mod", "w_b.go"):    "//go:build !linux\n\npackage mod\n\ntype W Page[B]\n",
	})
	require.Len(t, routes, 1)
	rows = renders(t, a.typeRegistry["Rows"])
	return a, rows
}

// TestInstantiationByteElemDecodeOnly pins that a byte-kind generic type with
// only decode-side methods is a plain byte as a slice element, so the slice
// stays base64 (#97), as a non-generic one is, while one with an encode-side
// method stays a Marshaler element.
func TestInstantiationByteElemDecodeOnly(t *testing.T) {
	a, rows := analyzeInstReview(t)
	assert.Equal(t, "[]byte", rows["by"])
	assert.Equal(t, "[]marshal:BE[int](unknown)", rows["be"])
	assert.Empty(t, matching(a.Warnings(t.Context()), "UnmarshalJSON"), "%v", a.warnings)
}

// TestInstantiationIdentityPastDepthCap pins that composite levels spend no
// identity depth, so two deep arguments that differ only at the leaf are two
// Instantiations, and that a capped identity is cut, never shared.
func TestInstantiationIdentityPastDepthCap(t *testing.T) {
	a, rows := analyzeInstReview(t)
	assert.NotEqual(t, rows["d1"], rows["d2"])
	assert.Equal(t, "$Box-slice-slice-slice-slice-slice-slice-slice-slice-slice-slice-int", rows["d1"])

	file, path := instReviewFile(t, a)
	c := newResolveCtx(file, path, false)
	shape, cut := instCut("Box["+identityCap+"]", "Box[deep]", c)
	require.True(t, cut)
	assert.Equal(t, models.ShapeRecursive, shape.Kind)
	assert.Equal(t, fallbackDepthCap, c.st.first.kind)
}

// instReviewFile returns the parsed module file of an analyzeInstReview
// project.
func instReviewFile(t *testing.T, a *ProjectAnalyzer) (file *ast.File, path string) {
	t.Helper()
	for dir, files := range a.pkgCache {
		if filepath.Base(dir) != "mod" {
			continue
		}
		for p, f := range files {
			if strings.HasSuffix(p, "module.go") {
				return f, p
			}
		}
	}
	require.Fail(t, "module file not parsed")
	return nil, ""
}

// TestInstantiationGenericAliasArgument pins that a generic alias argument is
// identified by its target: Box[Alias[int]] and Box[Box[int]] are one Go type
// and one component.
func TestInstantiationGenericAliasArgument(t *testing.T) {
	a, rows := analyzeInstReview(t)
	assert.Equal(t, "$Box-Box-int", rows["ab"])
	assert.Equal(t, "$Box-Box-int", rows["bb"])
	for name := range a.typeRegistry {
		assert.NotContains(t, name, "ModBox", "no collision copy")
	}
}

// TestInstantiationLiteralUnderTwoBindings pins that one anonymous struct
// literal reached under two bindings (List[List[int]]) is two types, not a
// cycle: no recursion warning, both levels documented.
func TestInstantiationLiteralUnderTwoBindings(t *testing.T) {
	a, rows := analyzeInstReview(t)
	assert.Equal(t, "[]struct{value []struct{value int}}", rows["ll"])
	assert.Empty(t, matching(a.Warnings(t.Context()), "contains itself"), "%v", a.warnings)
}

// TestBuildTaggedCopiesDifferentBindings pins that build-tagged copies of
// one name reaching one generic struct under different type arguments
// (type W Page[A] beside type W Page[B]) disagree on shape.
func TestBuildTaggedCopiesDifferentBindings(t *testing.T) {
	a, rows := analyzeInstReview(t)
	assert.Equal(t, "W", rows["w"])
	assert.Len(t, matching(a.Warnings(t.Context()), "W's build-tagged declarations disagree on shape (no common container shape"), 1, "%v", a.warnings)
	_, registered := a.typeRegistry["W"]
	assert.False(t, registered)
}

// TestGenericParenthesizedPointerReceiver pins that a parenthesized pointer
// receiver (func (r (*PT[X])) MarshalText) is the pointer's method, so PT is
// no text type both ways and stays a Marshaler, while a parenthesized value
// receiver (VT) is a string.
func TestGenericParenthesizedPointerReceiver(t *testing.T) {
	a, rows := analyzeInstReview(t)
	assert.Equal(t, "marshal:PT[int](unknown)", rows["pt"])
	assert.Equal(t, "text:VT[int]", rows["vt"])
	assert.Len(t, matching(a.Warnings(t.Context()), "PT has its own MarshalText"), 1, "%v", a.warnings)
}

// TestGenericInlineRequest pins that a request whose Instantiation is
// inlined for an anonymous struct argument is documented as an anonymous
// struct request: an inline body, no component, no warning.
func TestGenericInlineRequest(t *testing.T) {
	run := analyzeGenericRoute(t, "req Box[struct{ X int `json:\"x\"` }], ctx server.HandlerContext", "server.Result[Receipt]")
	assert.Empty(t, run.warnings)
	require.NotNil(t, run.route.Request)
	assert.Empty(t, run.route.Request.Name)
	assert.Equal(t, "struct{v struct{x int}}", renderShapePtr(run.route.Request.Resolution))
	assertNoGenericComponent(t, run.a)
}
