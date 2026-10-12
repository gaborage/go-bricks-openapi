package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// bindDecls declares the named types the binding rows use, in a file of
// their own.
const bindDecls = `package mod

import (
	"encoding/json"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	_ json.Number
	_ fs.FileMode
	_ net.IP
	_ netip.Addr
	_ os.FileMode
	_ time.Time
	_ uuid.UUID
	_ decimal.Decimal
)

type Cents int64
type Scope string
type Tags []string
type TimeA = time.Time
type Stamp time.Time
type ID uuid.UUID
type MyAddr netip.Addr

type Level string

func (l Level) MarshalText() ([]byte, error) { return []byte(l), nil }
func (l *Level) UnmarshalText(b []byte) error { *l = Level(b); return nil }

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

type Money struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

type Price Money

type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

type Paging struct {
	Page int   ` + "`query:\"page\"`" + `
	Size []int ` + "`query:\"size\"`" + `
}

type Inner struct {
	IDs []int ` + "`query:\"ids\"`" + `
}

type key [4]byte
`

// bindRow is one parameter field of Req: its Go name, type and tag, and the
// reason go-bricks cannot bind it ("" when it can).
type bindRow struct {
	field, goType, tag, reason string
}

func pathTag(name string) string   { return fmt.Sprintf("param:%q", name) }
func queryTag(name string) string  { return fmt.Sprintf("query:%q", name) }
func headerTag(name string) string { return fmt.Sprintf("header:%q", name) }

// bindRows are the binding rule's rows, each on the source it names.
var bindRows = []bindRow{
	// Binds: kindSetters kinds, named types over them, time.Time, pointers.
	{"S", "string", queryTag("s"), ""},
	{"B", "bool", queryTag("b"), ""},
	{"I", "int", pathTag("i"), ""},
	{"I8", "int8", headerTag("X-I8"), ""},
	{"U64", "uint64", queryTag("u64"), ""},
	{"Byte", "byte", queryTag("byte"), ""},
	{"Rune", "rune", queryTag("rune"), ""},
	{"F32", "float32", queryTag("f32"), ""},
	{"F64", "float64", queryTag("f64"), ""},
	{"Cents", "Cents", pathTag("cents"), ""},
	{"Scope", "Scope", headerTag("X-Scope"), ""},
	{"When", "time.Time", queryTag("when"), ""},
	{"PathWhen", "time.Time", pathTag("pathWhen"), ""},
	{"HeaderWhen", "time.Time", headerTag("X-When"), ""},
	{"WhenA", "TimeA", queryTag("whenA"), ""},
	{"PWhen", "*time.Time", queryTag("pWhen"), ""},
	{"PInt", "*int", queryTag("pInt"), ""},
	{"PPInt", "**int64", queryTag("ppInt"), ""},
	{"Lvl", "Level", queryTag("lvl"), ""},
	{"Dur", "time.Duration", queryTag("dur"), ""},
	{"Month", "time.Month", queryTag("month"), ""},
	{"Day", "time.Weekday", queryTag("day"), ""},
	{"Mode", "os.FileMode", queryTag("mode"), ""},
	{"FSMode", "fs.FileMode", queryTag("fsMode"), ""},
	{"Num", "json.Number", queryTag("num"), ""},
	{"Paren", "(int)", queryTag("paren"), ""},
	{"PParen", "*(string)", queryTag("pParen"), ""},
	// Binds on query and header only: a slice of a string kind.
	{"Strs", "[]string", queryTag("strs"), ""},
	{"HStrs", "[]string", headerTag("X-Strs"), ""},
	{"Scopes", "[]Scope", queryTag("scopes"), ""},
	{"HScopes", "[]Scope", headerTag("X-Scopes"), ""},
	{"TagList", "Tags", queryTag("tagList"), ""},
	{"HTagList", "Tags", headerTag("X-Tags"), ""},
	{"Nums", "[]json.Number", queryTag("nums"), ""},
	{"SParen", "[](string)", queryTag("sParen"), ""},
	// An array.
	{"UUIDPath", "uuid.UUID", pathTag("uuidPath"), reasonArray},
	{"UUIDQuery", "uuid.UUID", queryTag("uuidQuery"), reasonArray},
	{"UUIDHeader", "uuid.UUID", headerTag("X-UUID"), reasonArray},
	{"PUUID", "*uuid.UUID", queryTag("pUUID"), reasonArray},
	{"MyID", "ID", queryTag("myID"), reasonArray},
	{"Key", "[4]byte", queryTag("key"), reasonArray},
	{"Pair", "[2]string", queryTag("pair"), reasonArray},
	{"HPair", "[2]string", headerTag("X-Pair"), reasonArray},
	{"Triple", "[3]int", queryTag("triple"), reasonArray},
	// A struct other than time.Time.
	{"Addr", "Address", pathTag("addr"), reasonStruct},
	{"QAddr", "Address", queryTag("qAddr"), reasonStruct},
	{"PAddr", "*Address", queryTag("pAddr"), reasonStruct},
	{"Cash", "Money", queryTag("cash"), reasonStruct},
	{"Cost", "Price", queryTag("cost"), reasonStruct},
	{"Stamped", "Stamp", queryTag("stamped"), reasonStruct},
	{"Anon", "struct{A int}", queryTag("anon"), reasonStruct},
	{"IPAddr", "netip.Addr", queryTag("ipAddr"), reasonStruct},
	{"IPPrefix", "netip.Prefix", queryTag("ipPrefix"), reasonStruct},
	{"IPAddrPort", "netip.AddrPort", queryTag("ipAddrPort"), reasonStruct},
	{"MyIPAddr", "MyAddr", queryTag("myIPAddr"), reasonStruct},
	// A slice in a path parameter.
	{"PathStrs", "[]string", pathTag("pathStrs"), reasonPathSlice},
	{"PathTags", "Tags", pathTag("pathTags"), reasonPathSlice},
	{"PathIP", "net.IP", pathTag("pathIP"), reasonPathSlice},
	// A query or header slice whose element is not a string.
	{"Ints", "[]int", queryTag("ints"), reasonSliceElem},
	{"HInts", "[]int", headerTag("X-Ints"), reasonSliceElem},
	{"Bytes", "[]byte", queryTag("bytes"), reasonSliceElem},
	{"HBytes", "[]byte", headerTag("X-Bytes"), reasonSliceElem},
	{"Runes", "[]rune", queryTag("runes"), reasonSliceElem},
	{"PStrs", "[]*string", queryTag("pStrs"), reasonSliceElem},
	{"Raw", "json.RawMessage", queryTag("raw"), reasonSliceElem},
	{"IP", "net.IP", queryTag("ip"), reasonSliceElem},
	{"Times", "[]time.Time", queryTag("times"), reasonSliceElem},
	// A pointer to a slice.
	{"PStrSlice", "*[]string", queryTag("pStrSlice"), reasonPointerSlice},
	{"PTags", "*Tags", queryTag("pTags"), reasonPointerSlice},
	{"PRaw", "*json.RawMessage", queryTag("pRaw"), reasonPointerSlice},
	// A map, an interface, a complex number, a uintptr, a func, a chan.
	{"Attrs", "map[string]string", queryTag("attrs"), reasonMap},
	{"Any", "any", queryTag("any"), reasonInterface},
	{"Err", "error", queryTag("err"), reasonInterface},
	{"C64", "complex64", queryTag("c64"), reasonComplex},
	{"C128", "complex128", queryTag("c128"), reasonComplex},
	{"Ptr", "uintptr", queryTag("ptr"), reasonUintptr},
	{"Fn", "func()", queryTag("fn"), reasonFuncOrChan},
	{"Ch", "chan int", queryTag("ch"), reasonFuncOrChan},
}

// bindModuleHead is resolveModuleHead with the imports Req's qualified field
// types are written under.
const bindModuleHead = `package mod

import (
	"encoding/json"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }
`

// bindModule builds a module whose /a and /b routes both take Req, with the
// given fields, and whose /embed-* routes take the embed requests.
func bindModule(fields string) string {
	return bindModuleHead + "\ntype Req struct {\n" + fields + `}

type EmbedValue struct {
	Paging
}

type EmbedPtr struct {
	*Paging
}

type EmbedTagged struct {
	Paging ` + "`query:\"paging\"`" + `
}

func (m *Module) a(req Req, ctx server.HandlerContext) (server.Result[int], server.IAPIError) { return server.Result[int]{}, nil }
func (m *Module) b(req *Req, ctx server.HandlerContext) (server.Result[int], server.IAPIError) { return server.Result[int]{}, nil }
func (m *Module) ev(req EmbedValue, ctx server.HandlerContext) (server.Result[int], server.IAPIError) {
	return server.Result[int]{}, nil
}
func (m *Module) ep(req EmbedPtr, ctx server.HandlerContext) (server.Result[int], server.IAPIError) {
	return server.Result[int]{}, nil
}
func (m *Module) et(req EmbedTagged, ctx server.HandlerContext) (server.Result[int], server.IAPIError) {
	return server.Result[int]{}, nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/a", m.a)
	server.GET(hr, r, "/b", m.b)
	server.GET(hr, r, "/embed-value", m.ev)
	server.GET(hr, r, "/embed-ptr", m.ep)
	server.GET(hr, r, "/embed-tagged", m.et)
}
`
}

// analyzeBindRows analyzes a project whose Req holds rows plus extra field
// lines, and returns its warnings.
func analyzeBindRows(t *testing.T, rows []bindRow, extra string) []string {
	t.Helper()
	var fields strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&fields, "\t%s %s `%s`\n", r.field, r.goType, r.tag)
	}
	fields.WriteString(extra)
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod":        resolveGoMod,
		"mod/types.go":  bindDecls,
		"mod/module.go": bindModule(fields.String()),
	})
	return a.Warnings(t.Context())
}

// warningsAbout returns the warnings about the field named name.
func warningsAbout(ws []string, name string) []string {
	return matching(ws, "field "+name+" at ")
}

// TestUnbindableParamRows pins every row of the binding rule end to end: a
// parameter go-bricks cannot bind raises exactly one warning, naming its
// source, name, type and reason, once although two routes take Req, and one
// that binds raises none.
func TestUnbindableParamRows(t *testing.T) {
	ws := analyzeBindRows(t, bindRows, "")
	warned := 0
	for _, r := range bindRows {
		got := warningsAbout(ws, r.field)
		if r.reason == "" {
			assert.Empty(t, got, "%s %s", r.goType, r.tag)
			continue
		}
		warned++
		if !assert.Len(t, got, 1, "%s %s", r.goType, r.tag) {
			continue
		}
		source, param, _ := strings.Cut(r.tag, ":")
		if source == tagParam {
			source = paramTypePath
		}
		want := fmt.Sprintf("is the %s parameter %s of type %s, which go-bricks cannot bind (%s), so a request that sends it gets a 400",
			source, param, r.goType, r.reason)
		assert.True(t, strings.HasSuffix(got[0], want), "%s: %s", r.field, got[0])
	}
	assert.Len(t, ws, warned, "no other warning: %v", ws)
}

// TestUnbindableParamOneWarning pins decision 6: a parameter whose kind the
// analyzer knows gets the binding warning as its one warning, replacing the
// uintptr or schema fallback warning it raises as a field, while one whose
// type the analyzer cannot resolve keeps that warning and gets no binding
// warning: an unresolvable name, a slice of one in a query, and a generic
// instantiation.
func TestUnbindableParamOneWarning(t *testing.T) {
	rows := []bindRow{
		{"Ptr", "uintptr", queryTag("ptr"), reasonUintptr},
		{"C128", "complex128", queryTag("c128"), reasonComplex},
		{"Err", "error", queryTag("err"), reasonInterface},
		{"Stamped", "Stamp", queryTag("stamped"), reasonStruct},
		{"Ptrs", "[]uintptr", pathTag("ptrs"), reasonPathSlice},
		{"Decs", "[]decimal.Decimal", pathTag("decs"), reasonPathSlice},
	}
	extra := "\tDec decimal.Decimal `query:\"dec\"`\n\tQDecs []decimal.Decimal `query:\"qDecs\"`\n" +
		"\tPDec *decimal.Decimal `query:\"pDec\"`\n\tBoxed Box[int] `query:\"boxed\"`\n"
	ws := analyzeBindRows(t, rows, extra)
	for _, r := range rows {
		got := warningsAbout(ws, r.field)
		if assert.Len(t, got, 1, r.field) {
			assert.Contains(t, got[0], "which go-bricks cannot bind ("+r.reason+")", r.field)
		}
	}
	for field, want := range map[string]string{
		"Dec":   "decimal.Decimal resolves to no schema",
		"QDecs": "decimal.Decimal resolves to no schema",
		"PDec":  "decimal.Decimal resolves to no schema",
		"Boxed": "Box[int] is a generic type instantiation",
	} {
		got := warningsAbout(ws, field)
		if assert.Len(t, got, 1, field) {
			assert.Contains(t, got[0], want, field)
			assert.NotContains(t, got[0], "go-bricks cannot bind", field)
		}
	}
	assert.Len(t, ws, len(rows)+4, "%v", ws)
}

// TestUnbindableParamScope pins decision 8: only the request type's own
// top-level fields are checked. A promoted parameter of an untagged value
// or pointer embed, a tagged embed, a nested struct's tagged field and a body
// field of an unbindable type raise no binding warning (nor any other).
func TestUnbindableParamScope(t *testing.T) {
	ws := analyzeBindRows(t, nil, "\tIn Inner `json:\"in\"`\n\tBody map[string]int `json:\"body\"`\n")
	assert.Empty(t, ws)
}

// TestUnbindableParamAfterNestedRegistration pins that the binding warning
// is a field's one warning whichever route reaches the request struct first.
// When another route's response holds the struct as a field and registers it
// first, the uintptr and schema fallback warnings its fields raise there are
// replaced in place once a route takes it as its request, so both route
// orders yield the same warnings.
func TestUnbindableParamAfterNestedRegistration(t *testing.T) {
	var got [2][]string
	for i, order := range [2][2]string{{"a", "b"}, {"b", "a"}} {
		a, _ := analyzeProjectRoutes(t, map[string]string{
			"go.mod":       resolveGoMod,
			"mod/types.go": "package mod\n\nimport \"time\"\n\ntype Stamp time.Time\n",
			"mod/module.go": resolveModuleHead + `
type Req struct {
	Ptr     uintptr ` + "`query:\"ptr\"`" + `
	Stamped Stamp   ` + "`query:\"stamped\"`" + `
	Ints    []int   ` + "`query:\"ints\"`" + `
}

type Resp struct {
	R Req ` + "`json:\"r\"`" + `
}

func (m *Module) a(ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) { return server.Result[Resp]{}, nil }
func (m *Module) b(req Req, ctx server.HandlerContext) (server.Result[int], server.IAPIError) { return server.Result[int]{}, nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/` + order[0] + `", m.` + order[0] + `)
	server.GET(hr, r, "/` + order[1] + `", m.` + order[1] + `)
}
`,
		})
		got[i] = a.Warnings(t.Context())
	}
	reasons := []string{reasonUintptr, reasonStruct, reasonSliceElem}
	require.Len(t, got[0], len(reasons), "%v", got[0])
	for i, field := range []string{"Ptr", "Stamped", "Ints"} {
		assert.Contains(t, got[0][i], "field "+field+" at ")
		assert.Contains(t, got[0][i], "go-bricks cannot bind ("+reasons[i]+")")
	}
	assert.Equal(t, got[0], got[1])
}

// TestUnbindableParamTaggedEmbed pins that an embedded non-struct type with
// a parameter tag, which buildBindingPlan binds as one field named for its
// type, is judged like a named field, while an embedded struct, tagged or
// not and behind a pointer or not, is left to issue 147, and an unexported
// embed is skipped, as go-bricks skips it.
func TestUnbindableParamTaggedEmbed(t *testing.T) {
	extra := "\tuuid.UUID `query:\"id\"`\n\tScope `query:\"scope\"`\n\t*Cents `header:\"X-Cents\"`\n" +
		"\t*Address `query:\"addr\"`\n\tStamp `query:\"stamp\"`\n\tkey `query:\"key\"`\n"
	ws := analyzeBindRows(t, nil, extra)
	require.Len(t, ws, 1, "%v", ws)
	assert.Contains(t, ws[0], `field UUID at `)
	assert.Contains(t, ws[0], `is the query parameter "id" of type uuid.UUID, which go-bricks cannot bind (`+reasonArray+")")
}

// TestUnbindableParamMarshalerRequest pins that a struct Marshaler request
// (#111), whose parameters are read without registering it, is checked
// too, with the binding warning as a field's one warning.
func TestUnbindableParamMarshalerRequest(t *testing.T) {
	a, _ := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		"mod/module.go": resolveModuleHead + `
type Cmd struct {
	Ptr  uintptr ` + "`query:\"ptr\"`" + `
	Name string  ` + "`query:\"name\"`" + `
	Body string  ` + "`json:\"body\"`" + `
}

func (c *Cmd) UnmarshalJSON(b []byte) error { return nil }

func (m *Module) a(req Cmd, ctx server.HandlerContext) (server.Result[int], server.IAPIError) { return server.Result[int]{}, nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/a", m.a)
}
`,
	})
	ws := a.Warnings(t.Context())
	require.Len(t, ws, 2, "%v", ws)
	assert.Contains(t, ws[0], "field Ptr at ")
	assert.Contains(t, ws[0], "go-bricks cannot bind ("+reasonUintptr+")")
	assert.Contains(t, ws[1], "request type Cmd: Cmd has its own UnmarshalJSON method")
}

// TestWellKnownBinderKinds pins issue 121's table of well-known type names
// and their binder kinds, the rows of names not well-known yet included:
// big.Int is consulted only once issue 113 makes it well-known, so a
// big.Int parameter, unresolvable until then, gets no verdict.
func TestWellKnownBinderKinds(t *testing.T) {
	assert.Equal(t, map[string]binderKind{
		models.WellKnownTimeTime:      kindTime,
		models.WellKnownJSONNumber:    kindString,
		models.WellKnownUUID:          kindArray,
		models.WellKnownRawMessage:    kindByteSlice,
		models.WellKnownNetIP:         kindByteSlice,
		models.WellKnownNetipAddr:     kindStruct,
		models.WellKnownNetipPrefix:   kindStruct,
		models.WellKnownNetipAddrPort: kindStruct,
		"big.Int":                     kindStruct,
	}, wellKnownBinderKinds)
	for name := range wellKnownBinderKinds {
		assert.Equal(t, name != bigInt, models.WellKnownTypeNames[name], "%s is well-known", name)
	}
	big := models.TypeShape{Kind: models.ShapeNamed, Name: bigInt}
	unresolvable := fieldFallback{kind: fallbackUnresolvable, typeName: bigInt}
	for _, fb := range []fieldFallback{{}, unresolvable} {
		_, known := paramBindReason(&models.TypeShape{Kind: models.ShapePointer, Elem: &big}, paramTypeQuery, fb)
		assert.False(t, known, "%+v", fb)
	}
}

// TestParamBindReasonEdges pins the classifier's rows no source position
// reaches end to end: a build-tagged scalar's kind, a defined type over a
// well-known type by fallback detail, and the shapes that get no verdict.
func TestParamBindReasonEdges(t *testing.T) {
	str := models.TypeShape{Kind: models.ShapeKindOnly, Name: goTypeString}
	integer := models.TypeShape{Kind: models.ShapeKindOnly, Name: kindInteger}
	named := models.TypeShape{Kind: models.ShapeNamed, Name: "D"}
	over := func(detail string) fieldFallback {
		return fieldFallback{kind: fallbackDefinedOverQualified, typeName: "D", detail: detail}
	}
	cases := []struct {
		name   string
		s      models.TypeShape
		fb     fieldFallback
		reason string
		known  bool
	}{
		{"kind-only string", str, fieldFallback{}, "", true},
		{"kind-only integer", integer, fieldFallback{}, "", true},
		{"slice of kind-only string", models.TypeShape{Kind: models.ShapeSlice, Elem: &str}, fieldFallback{}, "", true},
		{"slice of kind-only integer", models.TypeShape{Kind: models.ShapeSlice, Elem: &integer}, fieldFallback{}, reasonSliceElem, true},
		{"defined over time.Time", named, over(models.WellKnownTimeTime), reasonStruct, true},
		{"defined over uuid.UUID", named, over(models.WellKnownUUID), reasonArray, true},
		{"defined over netip.Prefix", named, over(models.WellKnownNetipPrefix), reasonStruct, true},
		{"defined over decimal.Decimal", named, over("decimal.Decimal"), "", false},
		// A well-known spelling whose resolution noted it unresolvable is
		// judged by what the analyzer resolved, not by its spelling.
		{"well-known spelling noted unresolvable", models.TypeShape{Kind: models.ShapeNamed, Name: models.WellKnownUUID},
			fieldFallback{kind: fallbackUnresolvable, typeName: models.WellKnownUUID}, "", false},
		{"declarations disagree", named, fieldFallback{kind: fallbackDeclsDisagree, typeName: "D"}, "", false},
		{"recursion cut", models.TypeShape{Kind: models.ShapeRecursive, Name: "T"}, fieldFallback{kind: fallbackRecursive}, "", false},
		{"slice of a recursion cut", models.TypeShape{Kind: models.ShapeSlice, Elem: &models.TypeShape{Kind: models.ShapeRecursive, Name: "T"}},
			fieldFallback{kind: fallbackRecursive}, "", false},
		{"text leaf", models.TypeShape{Kind: models.ShapeText, Name: "T"}, fieldFallback{}, "", false},
		{"pointer without element", models.TypeShape{Kind: models.ShapePointer}, fieldFallback{}, "", false},
		{"slice without element", models.TypeShape{Kind: models.ShapeSlice}, fieldFallback{}, "", false},
		{"unknown builtin", models.TypeShape{Kind: models.ShapePrimitive, Name: "T"}, fieldFallback{}, "", false},
		{"zero shape", models.TypeShape{}, fieldFallback{}, reasonFuncOrChan, true},
	}
	for _, c := range cases {
		reason, known := paramBindReason(&c.s, paramTypeQuery, c.fb)
		assert.Equal(t, c.reason, reason, c.name)
		assert.Equal(t, c.known, known, c.name)
	}
}
