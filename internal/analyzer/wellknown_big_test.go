package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// bigTypes are math/big's three types, every marshal method of which is on
// the pointer.
var bigTypes = []string{models.WellKnownBigInt, models.WellKnownBigFloat, models.WellKnownBigRat}

// bigImports is the import line of the math/big rows modules.
const bigImports = "\t\"math/big\"\n"

// bigDecls are the named types the math/big rows use.
const bigDecls = `
type BI = big.Int
type Big big.Int
type BF big.Float
type BR big.Rat
`

// TestBigTypesResolve pins the Resolution of every field position of
// big.Int, big.Float and big.Rat: a pointer's element, an alias of one
// included, is a well-known leaf the generator types from its table; any
// other position, a slice element and a map value too, and a pointer to a
// slice of one, is a Marshaler leaf that warns once, naming the type's first
// encode-side method; a defined type over one keeps the defined-type warning.
func TestBigTypesResolve(t *testing.T) {
	rows := []resolveRow{
		{"int", "*big.Int", "*big.Int"},
		{"float", "*big.Float", "*big.Float"},
		{"rat", "*big.Rat", "*big.Rat"},
		{"ints", "[]*big.Int", "[]*big.Int"},
		{"floats", "map[string]*big.Float", "map[string]*big.Float"},
		{"alias", "*BI", "*big.Int"},
		{"twice", "**big.Rat", "**big.Rat"},
		{"valInt", "big.Int", "marshal:big.Int(unknown)"},
		{"valFloat", "big.Float", "marshal:big.Float(unknown)"},
		{"valRat", "big.Rat", "marshal:big.Rat(unknown)"},
		{"valAlias", "BI", "marshal:big.Int(unknown)"},
		{"valInts", "[]big.Int", "[]marshal:big.Int(unknown)"},
		{"valMap", "map[string]big.Rat", "map[string]marshal:big.Rat(unknown)"},
		{"valArr", "[2]big.Float", "[N]marshal:big.Float(unknown)"},
		{"ptrSlice", "*[]big.Int", "*[]marshal:big.Int(unknown)"},
		{"ptrArr", "*[2]big.Int", "*[N]marshal:big.Int(unknown)"},
		{"ptrMap", "*map[string]big.Rat", "*map[string]marshal:big.Rat(unknown)"},
		{"defInt", "*Big", "*Big"},
		{"defFloat", "*BF", "*BF"},
		{"defRat", "*BR", "*BR"},
	}
	a, got := analyzeRows(t, bigImports, bigDecls, rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
	ws := a.Warnings(t.Context())
	for _, w := range []struct{ field, written, name, method string }{
		{"ValInt", "big.Int", "big.Int", methodMarshalJSON},
		{"ValFloat", "big.Float", "big.Float", methodMarshalText},
		{"ValRat", "big.Rat", "big.Rat", methodMarshalText},
		{"ValAlias", "BI", "big.Int", methodMarshalJSON},
		{"ValInts", "[]big.Int", "big.Int", methodMarshalJSON},
		{"ValMap", "map[string]big.Rat", "big.Rat", methodMarshalText},
		{"ValArr", "[2]big.Float", "big.Float", methodMarshalText},
		{"PtrSlice", "*[]big.Int", "big.Int", methodMarshalJSON},
		{"PtrArr", "*[2]big.Int", "big.Int", methodMarshalJSON},
		{"PtrMap", "*map[string]big.Rat", "big.Rat", methodMarshalText},
	} {
		got := warningsAbout(ws, w.field)
		if assert.Len(t, got, 1, "%s: %v", w.field, ws) {
			assert.Contains(t, got[0], " has type "+w.written+": "+w.name+" has its own "+w.method+
				" method, which encoding/json uses instead of its underlying type", w.field)
		}
	}
	for _, def := range []struct{ field, name, over string }{{"DefInt", "Big", "big.Int"}, {"DefFloat", "BF", "big.Float"}, {"DefRat", "BR", "big.Rat"}} {
		assert.Len(t, matching(warningsAbout(ws, def.field), def.name+" is a defined type over "+def.over), 1, "%v", ws)
	}
	assert.Len(t, ws, 13, "%v", ws)
}

// TestBigTypesAliasedImport pins that the three resolve through an aliased
// import of math/big (mb.Int) exactly as under its own name: the tables are
// read through the import path (wellKnownPackages).
func TestBigTypesAliasedImport(t *testing.T) {
	rows := []resolveRow{
		{"int", "*mb.Int", "*big.Int"},
		{"float", "*mb.Float", "*big.Float"},
		{"rat", "*mb.Rat", "*big.Rat"},
		{"valInt", "mb.Int", "marshal:big.Int(unknown)"},
		{"valFloat", "mb.Float", "marshal:big.Float(unknown)"},
		{"valRat", "mb.Rat", "marshal:big.Rat(unknown)"},
	}
	a, got := analyzeRows(t, "\tmb \"math/big\"\n", "", rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
	ws := a.Warnings(t.Context())
	assert.Len(t, ws, 3, "%v", ws)
	assert.Len(t, matching(ws, "big.Int has its own MarshalJSON method"), 1, "%v", ws)
	assert.Len(t, matching(ws, "big.Float has its own MarshalText method"), 1, "%v", ws)
	assert.Len(t, matching(ws, "big.Rat has its own MarshalText method"), 1, "%v", ws)
}

// TestBigTypesParamsStayWellKnown pins that a parameter of a value big.X is
// resolved by kind, never as a Marshaler leaf, and notes nothing: the binder
// judges it (issue 121).
func TestBigTypesParamsStayWellKnown(t *testing.T) {
	a, c := methodSetProject(t, "import \"math/big\"\n")
	c.st.param = true
	for _, name := range bigTypes {
		r := a.resolveShape(models.TypeShape{Kind: models.ShapeNamed, Name: name}, c)
		assert.Equal(t, name, renderShape(r), name)
	}
	assert.Equal(t, fallbackNone, c.st.first.kind)
}

// TestBigTypesPayloads pins the payload rows: an explicit pointer, under a
// slice or ResultWithMeta too, is a typed payload with no warning; a value
// big.Int is a Marshaler leaf, untyped, with the payload form of the
// Marshaler warning.
func TestBigTypesPayloads(t *testing.T) {
	payloads := []struct{ result, res string }{
		{"server.Result[*big.Int]", "*big.Int"},
		{"server.Result[*big.Float]", "*big.Float"},
		{"server.Result[*big.Rat]", "*big.Rat"},
		{"server.Result[[]*big.Int]", "[]*big.Int"},
		{"server.ResultWithMeta[*big.Int]", "*big.Int"},
		{"server.Result[*BI]", "*big.Int"},
		{"server.Result[big.Int]", "marshal:big.Int(unknown)"},
	}
	var handlers, regs strings.Builder
	for i, p := range payloads {
		fmt.Fprintf(&handlers, "func (m *Module) h%d(ctx server.HandlerContext) (%s, server.IAPIError) { return %s{}, nil }\n", i, p.result, p.result)
		fmt.Fprintf(&regs, "\tserver.GET(hr, r, \"/p%d\", m.h%d)\n", i, i)
	}
	src := "package mod\n\nimport (\n\t\"math/big\"\n\n\t\"github.com/gaborage/go-bricks/app\"\n\t\"github.com/gaborage/go-bricks/server\"\n)\n\n" +
		"type Module struct{}\n\n" +
		"func (m *Module) Name() string                    { return \"mod\" }\n" +
		"func (m *Module) Init(deps *app.ModuleDeps) error { return nil }\n" +
		"func (m *Module) Shutdown() error                 { return nil }\n\n" +
		"type BI = big.Int\n\n" + handlers.String() +
		"\nfunc (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" + regs.String() + "}\n"
	a, routes := analyzeSingleModule(t, src)
	for i, p := range payloads {
		resp := routeByPath(t, routes, fmt.Sprintf("/p%d", i)).Response
		require.NotNil(t, resp, p.result)
		require.NotNil(t, resp.Resolution, p.result)
		assert.Equal(t, p.res, renderShape(*resp.Resolution), p.result)
		assert.Equal(t, i != len(payloads)-1, IsTypedPayload(resp), p.result)
	}
	ws := a.Warnings(t.Context())
	assert.Equal(t, []string{
		fmt.Sprintf(marshalerPayloadWarning, models.WellKnownBigInt, models.WellKnownBigInt, methodMarshalJSON),
	}, matching(ws, "response type "), "%v", ws)
}

// TestBigTypesMethodSets pins each type's recorded method set: every method
// on the pointer, so big.Int (MarshalJSON) and a value big.Float or big.Rat
// (pointer-only MarshalText) are no text type, and a pointer embed of
// big.Float or big.Rat, whose methods it puts in the value method set, is
// text both ways.
func TestBigTypesMethodSets(t *testing.T) {
	a, c := methodSetProject(t, "import \"math/big\"\n")
	text := bitMarshalText | bitAppendText | bitUnmarshalText
	for name, want := range map[string]struct {
		bits  methodBits
		first string
	}{
		models.WellKnownBigInt:   {bitMarshalJSON | bitUnmarshalJSON | text, methodMarshalJSON},
		models.WellKnownBigFloat: {text, methodMarshalText},
		models.WellKnownBigRat:   {text, methodMarshalText},
	} {
		ms := a.qualifiedMethodSet(name, c, true, nil)
		m, _ := ms.marshaler(name)
		assert.Equal(t, want.bits, m.has, name)
		assert.Equal(t, want.bits, m.ptr, name)
		assert.Equal(t, want.first, m.ptrEncode(), name)
		assert.False(t, m.textBothWays(), name)

		byPtr, _ := promote([]embedMethods{{via: name, set: ms, ptr: true}}).marshaler("E")
		assert.Zero(t, byPtr.ptr, name)
		assert.Equal(t, name != models.WellKnownBigInt, byPtr.textBothWays(), name)
		byValue, _ := promote([]embedMethods{{via: name, set: ms}}).marshaler("E")
		assert.Equal(t, want.bits, byValue.ptr, name)
		assert.False(t, byValue.textBothWays(), name)
	}
}

// TestWellKnownMethodSetReceivers pins every recorded method of every
// well-known type with its receiver, as Go 1.27's standard library and
// google/uuid declare it.
func TestWellKnownMethodSetReceivers(t *testing.T) {
	text := []wellKnownMethod{{methodMarshalText, false}, {methodAppendText, false}, {methodUnmarshalText, true}}
	ptrText := []wellKnownMethod{{methodMarshalText, true}, {methodAppendText, true}, {methodUnmarshalText, true}}
	assert.Equal(t, map[string][]wellKnownMethod{
		models.WellKnownTimeTime: {
			{methodMarshalJSON, false}, {methodMarshalText, false}, {methodAppendText, false},
			{methodUnmarshalJSON, true}, {methodUnmarshalText, true},
		},
		models.WellKnownUUID:          {{methodMarshalText, false}, {methodUnmarshalText, true}},
		models.WellKnownRawMessage:    {{methodMarshalJSON, false}, {methodUnmarshalJSON, true}},
		models.WellKnownJSONNumber:    {{methodMarshalJSONTo, false}, {methodUnmarshalJSONFrom, true}},
		models.WellKnownNetIP:         text,
		models.WellKnownNetipAddr:     text,
		models.WellKnownNetipPrefix:   text,
		models.WellKnownNetipAddrPort: text,
		models.WellKnownBigInt: {
			{methodMarshalJSON, true}, {methodMarshalText, true}, {methodAppendText, true},
			{methodUnmarshalJSON, true}, {methodUnmarshalText, true},
		},
		models.WellKnownBigFloat: ptrText,
		models.WellKnownBigRat:   ptrText,
	}, wellKnownMethodSets)
	ms := wellKnownMethodSet(models.WellKnownBigFloat, "mb.Float")
	assert.Equal(t, methodEntry{exact: true, ptrOnly: true, owner: "mb.Float"}, ms[methodMarshalText])
	assert.Empty(t, wellKnownMethodSet("decimal.Decimal", "decimal.Decimal"))
}

// TestBigTypesStayOutOfKindTables pins that none of the three is typed by
// kind: an entry would ignore the methods.
func TestBigTypesStayOutOfKindTables(t *testing.T) {
	a := New(t.TempDir())
	file := importsFile(t, "math/big")
	for _, name := range bigTypes {
		assert.NotContains(t, knownUnderlyingBuiltins, name)
		assert.False(t, knownByteSlices[name], name)
		assert.True(t, models.WellKnownTypeNames[name], name)
		_, ok := a.kindBackedUnderlying(name, file)
		assert.False(t, ok, name)
	}
}

// TestBigTypeEmbeds pins the six embeds and a defined type's: a pointer
// embed of big.Int promotes MarshalJSON and a value one keeps it, both {}
// with the promoted Marshaler warning; a pointer embed of big.Float or
// big.Rat is a string with no warning, and a value one {} with the warning.
// None registers a component, and a json:"-" tag changes nothing, since Go
// promotes the methods anyway. A defined type over big.Int, a struct with no
// exported field, adds nothing.
func TestBigTypeEmbeds(t *testing.T) {
	decls := bigDecls + `
type EmbPtrInt struct {
	*big.Int
}
type EmbInt struct {
	big.Int
}
type EmbPtrFloat struct {
	*big.Float
}
type EmbFloat struct {
	big.Float
}
type EmbPtrRat struct {
	*big.Rat
}
type EmbRat struct {
	big.Rat
}
type EmbHidden struct {
	big.Int ` + "`json:\"-\"`" + `
	N int ` + "`json:\"n\"`" + `
}
type EmbBig struct {
	Big
	N int ` + "`json:\"n\"`" + `
}
`
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): rowsModule(bigImports, decls,
			"\tPtrInt EmbPtrInt `json:\"ptrInt\"`\n\tInt EmbInt `json:\"int\"`\n\tPtrFloat EmbPtrFloat `json:\"ptrFloat\"`\n"+
				"\tFloat EmbFloat `json:\"float\"`\n\tPtrRat EmbPtrRat `json:\"ptrRat\"`\n\tRat EmbRat `json:\"rat\"`\n"+
				"\tBig EmbBig `json:\"big\"`\n\tHidden EmbHidden `json:\"hidden\"`\n"),
	})
	rows := renders(t, a.typeRegistry["Rows"])
	want := map[string]string{
		"ptrInt": "marshal:EmbPtrInt(unknown)", "int": "marshal:EmbInt(unknown)",
		"ptrFloat": "text:EmbPtrFloat", "float": "marshal:EmbFloat(unknown)",
		"ptrRat": "text:EmbPtrRat", "rat": "marshal:EmbRat(unknown)",
		"big": "$EmbBig", "hidden": "marshal:EmbHidden(unknown)",
	}
	assert.Equal(t, want, rows)
	for _, typ := range []string{"EmbPtrInt", "EmbInt", "EmbPtrFloat", "EmbFloat", "EmbPtrRat", "EmbRat", "EmbHidden"} {
		assert.Nil(t, a.typeRegistry[typ], "%s registers no component", typ)
	}
	assert.Equal(t, map[string]string{"n": goTypeInt}, renders(t, a.typeRegistry["EmbBig"]))
	ws := a.Warnings(t.Context())
	for field, w := range map[string]string{
		"PtrInt": "EmbPtrInt has the MarshalJSON method promoted from its embedded big.Int",
		"Int":    "EmbInt has the MarshalJSON method promoted from its embedded big.Int",
		"Float":  "EmbFloat has the MarshalText method promoted from its embedded big.Float",
		"Rat":    "EmbRat has the MarshalText method promoted from its embedded big.Rat",
		"Hidden": "EmbHidden has the MarshalJSON method promoted from its embedded big.Int",
	} {
		assert.Len(t, matching(warningsAbout(ws, field), w), 1, "%s: %v", field, ws)
	}
	assert.Len(t, ws, 5, "%v", ws)
}

// TestBigTypeParamsWarnOnce pins issue 121's rows for the three types: a
// query or header parameter of one, by value or by pointer, raises exactly
// one warning, the binding warning, and never the Marshaler warning a value
// one raises in a body position.
func TestBigTypeParamsWarnOnce(t *testing.T) {
	var extra strings.Builder
	fields := map[string]string{}
	for i, name := range bigTypes {
		for j, written := range []string{name, "*" + name} {
			field := fmt.Sprintf("B%d%d", i, j)
			tag := fmt.Sprintf("query:\"b%d%d\"", i, j)
			if j == 1 {
				tag = fmt.Sprintf("header:\"X-B%d%d\"", i, j)
			}
			fmt.Fprintf(&extra, "\t%s %s `%s`\n", field, written, tag)
			fields[field] = written
		}
	}
	ws := analyzeBindRows(t, nil, extra.String())
	for field, written := range fields {
		got := warningsAbout(ws, field)
		if assert.Len(t, got, 1, "%s: %v", field, ws) {
			assert.Contains(t, got[0], "of type "+written+", which go-bricks cannot bind ("+reasonStruct+")")
		}
	}
	assert.Empty(t, matching(ws, "has its own"), "%v", ws)
	assert.Len(t, ws, len(fields), "%v", ws)
}
