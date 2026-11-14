package analyzer

import (
	"fmt"
	"go/ast"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Type names the interface Marshaler tests (#179) repeat.
const (
	ifJM    = "JM"
	ifNJM   = "NJM"
	ifIface = "Iface"
)

// ifDecls declares in-module interfaces with and without marshal methods, and
// the structs embedding them (#179).
const ifDecls = `
type JM interface{ MarshalJSON() ([]byte, error) }

type JMb interface {
	MarshalJSON() ([]byte, error)
}

type JM2 interface {
	JM
	Name() string
}

type U interface {
	JM
	JMb
	MarshalJSON() ([]byte, error)
}

type J2 JM

type JA = JM

type Lit = interface{ UnmarshalJSON([]byte) error }

type BadJM interface{ MarshalJSON() string }

type Odd interface {
	BadJM
	(MarshalText)
}

type MarshalText interface{ MarshalText() ([]byte, error) }

type Ext interface {
	json.Marshaler
}

type Constraint interface{ ~int | ~string }

type QT domain.TM

//openapi:schema string decimal
type Iface interface{ MarshalJSON() ([]byte, error) }

type TMOnly interface{ MarshalText() ([]byte, error) }

type LitEmb interface {
	interface{ MarshalJSON() ([]byte, error) }
	Name() string
}

type NLit struct {
	LitEmb
	L int ` + "`json:\"l\"`" + `
}

type NJM struct {
	JM
	X int ` + "`json:\"x\"`" + `
}

type NJM2 struct {
	JM2
	Z int ` + "`json:\"z\"`" + `
}

type ND struct {
	J2
	V int ` + "`json:\"v\"`" + `
}

type NBad struct {
	BadJM
	W int ` + "`json:\"w\"`" + `
}

type NU struct {
	U
	Q int ` + "`json:\"q\"`" + `
}

type Cancel struct {
	JM
	JMb
	C int ` + "`json:\"c\"`" + `
}

type NTM struct {
	domain.TM
	Y int ` + "`json:\"y\"`" + `
}
`

// ifDomainSrc is the in-module domain package: a text interface TM.
const ifDomainSrc = `package domain

type TM interface {
	MarshalText() ([]byte, error)
	UnmarshalText([]byte) error
}
`

// ifImports are the imports ifDecls needs.
const ifImports = "\t\"encoding/json\"\n\t\"github.com/example/app/domain\"\n"

// ifProject analyzes a project holding ifDecls, the domain package and rows
// (Rows' fields), and returns its analyzer.
func ifProject(t *testing.T, rows string) *ProjectAnalyzer {
	t.Helper()
	return analyzeDirectiveProject(t, map[string]string{
		"go.mod":                             resolveGoMod,
		filepath.Join("domain", "domain.go"): ifDomainSrc,
		filepath.Join("mod", "module.go"):    rowsModule(ifImports, ifDecls, rows),
	})
}

// TestInterfaceMethodSet pins an interface's method set (#179): exact and
// non-exact declared methods, the embedded-interface union, overlapping
// identical methods (one method, not ambiguous), a defined type and an alias
// over an interface, an interface literal alias, an out-of-module embed and a
// constraint, each as a type of its own and once embedded in a struct.
func TestInterfaceMethodSet(t *testing.T) {
	a := ifProject(t, "\tN int `json:\"n\"`\n")
	file, path := ifModuleFile(t, a)
	own := func(name, typeName, method string) methodSetRow {
		return methodSetRow{name: name, found: true, kind: fallbackMarshaler, typeName: typeName, detail: method}
	}
	promoted := func(name, method, via string) methodSetRow {
		return methodSetRow{name: name, found: true, kind: fallbackMarshalerPromoted, typeName: name, detail: method, via: via}
	}
	rows := []methodSetRow{
		own(ifJM, ifJM, methodMarshalJSON),
		own("JM2", "JM2", methodMarshalJSON),
		own("U", "U", methodMarshalJSON),
		own("J2", "J2", methodMarshalJSON),
		own("JA", ifJM, methodMarshalJSON),
		own("Lit", "Lit", methodUnmarshalJSON),
		own("Odd", "Odd", methodMarshalText),
		own("TMOnly", "TMOnly", methodMarshalText),
		own("LitEmb", "LitEmb", methodMarshalJSON),
		promoted("NLit", methodMarshalJSON, "LitEmb"),
		{name: "BadJM"},
		{name: "Ext"},
		{name: "Constraint"},
		promoted(ifNJM, methodMarshalJSON, ifJM),
		promoted("NJM2", methodMarshalJSON, "JM2"),
		promoted("ND", methodMarshalJSON, "J2"),
		promoted("NU", methodMarshalJSON, "U"),
		{name: "NBad"},
		{name: "Cancel"},
		{name: "NTM", found: true, text: true},
	}
	for _, r := range rows {
		c := newResolveCtx(file, path, false)
		ms := a.methodSetOf(r.name, c, true, nil)
		m, fb := ms.marshaler(c.display(r.name))
		assert.Equal(t, r.found, m.found(), r.name)
		assert.Equal(t, r.text, m.textBothWays(), r.name)
		assert.Zero(t, m.ptr, "%s: an interface's methods are in the value method set", r.name)
		if r.found && !r.text {
			assert.Equal(t, fieldFallback{kind: r.kind, typeName: r.typeName, detail: r.detail, via: r.via}, fb, r.name)
		}
	}
	c := newResolveCtx(file, path, false)
	assert.False(t, a.methodSetOf("U", c, true, nil)[methodMarshalJSON].ambiguous, "overlapping identical methods are one")
	assert.True(t, a.methodSetOf("Cancel", c, true, nil)[methodMarshalJSON].ambiguous, "two embeds at one depth cancel")
	bad, inSet := a.methodSetOf("BadJM", c, true, nil)[methodMarshalJSON]
	assert.True(t, inSet && !bad.exact, "a non-exact method is in the set, where it shadows a deeper one")
	m, _ := a.methodSetOf("J2", c, false, nil).marshaler("J2")
	assert.True(t, m.found(), "a defined type over an interface keeps its methods whatever keep")
}

// ifModuleFile parses the mod/module.go of a's project.
func ifModuleFile(t *testing.T, a *ProjectAnalyzer) (file *ast.File, path string) {
	t.Helper()
	dir := filepath.Join(a.projectRoot, "mod")
	src, err := os.ReadFile(filepath.Join(dir, "module.go"))
	require.NoError(t, err)
	return parseInDir(t, a, dir, "module.go", string(src))
}

// TestInterfaceMarshalerFields pins the field rows (#179): an interface-typed
// field, in every body position, is a Marshaler leaf when its interface
// declares a marshal method, a text leaf when it declares the text pair, and
// unchanged otherwise; a struct embedding such an interface follows the
// promoted method, unless two embeds cancel it or its method is not exact.
func TestInterfaceMarshalerFields(t *testing.T) {
	a := ifProject(t, "\tJ JM `json:\"j\"`\n\tT domain.TM `json:\"t\"`\n\tJP *JM `json:\"jp\"`\n"+
		"\tJs []JM `json:\"js\"`\n\tJMap map[string]JM `json:\"jmap\"`\n\tJ2 J2 `json:\"j2\"`\n\tJA JA `json:\"ja\"`\n"+
		"\tJM2 JM2 `json:\"jm2\"`\n\tBad BadJM `json:\"bad\"`\n\tE Ext `json:\"e\"`\n\tAny any `json:\"any\"`\n"+
		"\tQ QT `json:\"q\"`\n\tN NJM `json:\"n\"`\n\tNs []NJM `json:\"ns\"`\n\tNT NTM `json:\"nt\"`\n\tNB NBad `json:\"nb\"`\n\tCa Cancel `json:\"ca\"`\n")
	got := renders(t, a.typeRegistry["Rows"])
	for field, want := range map[string]string{
		"j": "marshal:JM(unknown)", "t": "text:TM", "jp": "*marshal:JM(unknown)", "js": "[]marshal:JM(unknown)",
		"jmap": "map[string]marshal:JM(unknown)", "j2": "marshal:J2(unknown)", "ja": "marshal:JA(unknown)",
		"jm2": "marshal:JM2(unknown)", "n": "marshal:NJM(unknown)", "ns": "[]marshal:NJM(unknown)",
		"nt": "text:NTM", "q": "text:QT", "nb": "$NBad", "ca": "$Cancel",
	} {
		assert.Equal(t, want, got[field], field)
	}
	assert.NotContains(t, got["bad"], "marshal:", "a non-exact method does not count")
	assert.NotContains(t, got["e"], "marshal:", "an out-of-module embedded interface contributes nothing")
	assert.NotContains(t, got["any"], "marshal:")
	for _, name := range []string{ifNJM, "NTM"} {
		assert.NotContains(t, a.typeRegistry, name)
	}
	assert.Contains(t, a.typeRegistry, "NBad")

	fieldRows := []struct{ field, written, typeName, method, via string }{
		{"J", ifJM, ifJM, methodMarshalJSON, ""},
		{"JP", "*JM", ifJM, methodMarshalJSON, ""},
		{"Js", "[]JM", ifJM, methodMarshalJSON, ""},
		{"JMap", "map[string]JM", ifJM, methodMarshalJSON, ""},
		{"J2", "J2", "J2", methodMarshalJSON, ""},
		{"JA", "JA", ifJM, methodMarshalJSON, ""},
		{"JM2", "JM2", "JM2", methodMarshalJSON, ""},
		{"N", ifNJM, ifNJM, methodMarshalJSON, ifJM},
		{"Ns", "[]NJM", ifNJM, methodMarshalJSON, ifJM},
	}
	for _, r := range fieldRows {
		w := fieldWarnings(a, r.field)
		if !assert.Len(t, w, 1, r.field) {
			continue
		}
		want := fmt.Sprintf(marshalerFieldWarning, r.field, "\x00", r.written, r.typeName, r.method)
		if r.via != "" {
			want = fmt.Sprintf(marshalerPromotedFieldWarning, r.field, "\x00", r.written, r.typeName, r.method, r.via)
		}
		head, tail, _ := strings.Cut(want, "\x00")
		assert.True(t, strings.HasPrefix(w[0], head) && strings.HasSuffix(w[0], tail), "%s: %s", r.field, w[0])
	}
	for _, field := range []string{"T", "Q", "NT", "Bad", "E", "Any"} {
		assert.Empty(t, fieldWarnings(a, field), field)
	}
	// Cancel is documented from its fields: its embedded JM and JMb are
	// Marshaler fields of their own.
	assert.Len(t, fieldWarnings(a, ifJM), 1)
	assert.Len(t, fieldWarnings(a, "JMb"), 1)
}

// TestInterfaceMarshalerPayloadAndParam pins a Result[NJM] payload, which
// takes the promoted-method payload warning, a Result[JM] payload, which
// takes the own-method one, a direct json.Marshaler embed, which keeps its
// unresolvable-embed warning, and a query parameter of an interface type,
// which the Marshaler guard leaves alone: issue 195's binding warning stays
// its one warning.
func TestInterfaceMarshalerPayloadAndParam(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                             resolveGoMod,
		filepath.Join("domain", "domain.go"): ifDomainSrc,
		filepath.Join("mod", "module.go"): strings.Replace(resolveModuleHead, "import (\n",
			"import (\n"+ifImports, 1) + ifDecls +
			"\ntype Params struct {\n\tJ JM `query:\"j\"`\n}\n\n" +
			"type EmbStd struct {\n\tjson.Marshaler\n\tX int `json:\"x\"`\n}\n\n" +
			"func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" +
			"\tserver.GET(hr, r, \"/njm\", m.njm)\n\tserver.GET(hr, r, \"/p\", m.p)\n" +
			"\tserver.GET(hr, r, \"/jm\", m.jm)\n\tserver.GET(hr, r, \"/std\", m.std)\n}\n\n" +
			"func (m *Module) njm(ctx server.HandlerContext) (server.Result[NJM], server.IAPIError) { panic(0) }\n\n" +
			"func (m *Module) jm(ctx server.HandlerContext) (server.Result[JM], server.IAPIError) { panic(0) }\n\n" +
			"func (m *Module) std(ctx server.HandlerContext) (server.Result[EmbStd], server.IAPIError) { panic(0) }\n\n" +
			"func (m *Module) p(req Params, ctx server.HandlerContext) (server.Result[int], server.IAPIError) { panic(0) }\n",
	})
	assert.Contains(t, a.warnings, fmt.Sprintf(marshalerPromotedPayloadWarning, ifNJM, ifNJM, methodMarshalJSON, ifJM))
	assert.Contains(t, a.warnings, fmt.Sprintf(marshalerPayloadWarning, ifJM, ifJM, methodMarshalJSON))
	assert.True(t, slices.ContainsFunc(a.warnings, func(w string) bool {
		return strings.HasPrefix(w, "embedded field Marshaler at ") && strings.Contains(w, "json.Marshaler cannot be read")
	}), "a direct json.Marshaler embed keeps its unresolvable-embed warning: %q", a.warnings)
	assert.Len(t, a.warnings, 4, "%q", a.warnings)
	assert.NotContains(t, a.typeRegistry, ifNJM)
	w := fieldWarnings(a, "J")
	require.Len(t, w, 1, "the binding warning stays the parameter's one warning")
	assert.True(t, strings.HasPrefix(w[0], "field J at "), w[0])
	assert.Contains(t, w[0], "which go-bricks cannot bind")
	assert.NotContains(t, renders(t, a.typeRegistry["Params"])["J"], "marshal:", "a parameter is exempt from the Marshaler guard")
}

// TestInterfaceStatedSchema pins that a Stated schema on an interface
// declaring a marshal method is accepted and applied (typeMarshalerMethods
// reads the interface's method set as resolveLocal does).
func TestInterfaceStatedSchema(t *testing.T) {
	a := ifProject(t, "\tI Iface `json:\"i\"`\n")
	ti := a.typeRegistry["Rows"]
	assert.Equal(t, "marshal:Iface(unknown)", renders(t, ti)["i"])
	assert.NotNil(t, fieldByJSON(t, ti, "i").ResolvedShape().Stated)
	assert.Empty(t, a.warnings)
}

// TestEndsAtInterface pins the chains endsAtInterface follows.
func TestEndsAtInterface(t *testing.T) {
	a := ifProject(t, "\tN int `json:\"n\"`\n")
	file, path := ifModuleFile(t, a)
	for name, want := range map[string]bool{
		ifJM: true, "J2": true, "JA": true, "Lit": true, "Constraint": true, "QT": true, "Ext": true, "NJM": false, "Missing": false,
	} {
		assert.Equal(t, want, a.endsAtInterface(file, path, name, 0), name)
	}
	assert.False(t, a.endsAtInterface(file, path, ifJM, maxNamedResolutionDepth), "past the depth cap")
}

// ifJSONTextSrc is an in-module package declaring the jsontext-signature
// methods on interfaces under an aliased import, and a wrong-parameter
// variant that does not count.
const ifJSONTextSrc = `package jx

import jt "encoding/json/jsontext"

type JTo interface {
	MarshalJSONTo(enc *jt.Encoder) error
}

type JFrom interface {
	UnmarshalJSONFrom(*jt.Decoder) error
}

type JToBad interface {
	MarshalJSONTo(enc *jt.Decoder) error
}
`

// TestInterfaceTiesAndJSONTextMethods pins an interface embed tying with a
// struct embed at one depth: the struct's MarshalJSON and the interface's
// (exact or not) cancel, so the struct is documented from its fields, as
// encoding/json writes it. It also pins MarshalJSONTo and UnmarshalJSONFrom
// on in-module interfaces, read through an aliased jsontext import, and a
// wrong parameter that is ignored.
func TestInterfaceTiesAndJSONTextMethods(t *testing.T) {
	decls := "type JM interface{ MarshalJSON() ([]byte, error) }\n\n" +
		"type BadJM interface{ MarshalJSON() string }\n\n" +
		"type S struct{}\n\nfunc (S) MarshalJSON() ([]byte, error) { return nil, nil }\n\n" +
		"type NX struct {\n\tS\n\tK int `json:\"k\"`\n}\n\n" +
		"type NY struct {\n\tJM\n\tS\n\tK int `json:\"k\"`\n}\n\n" +
		"type NYBad struct {\n\tBadJM\n\tS\n\tK int `json:\"k\"`\n}\n"
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                     resolveGoMod,
		filepath.Join("jx", "jx.go"): ifJSONTextSrc,
		filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/jx\"\n", decls,
			"\tX NX `json:\"x\"`\n\tY NY `json:\"y\"`\n\tYB NYBad `json:\"yb\"`\n"+
				"\tTo jx.JTo `json:\"to\"`\n\tFrom jx.JFrom `json:\"from\"`\n\tToBad jx.JToBad `json:\"toBad\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "marshal:NX(unknown)", got["x"], "S's MarshalJSON alone is promoted")
	assert.Equal(t, "$NY", got["y"], "JM's MarshalJSON ties with S's")
	assert.Equal(t, "$NYBad", got["yb"], "a non-exact interface method still ties")
	assert.Equal(t, "marshal:JTo(unknown)", got["to"])
	assert.Equal(t, "marshal:JFrom(unknown)", got["from"])
	assert.NotContains(t, got["toBad"], "marshal:", "a *jsontext.Decoder parameter is not MarshalJSONTo's")
	assert.Equal(t, map[string]string{"JM": "marshal:JM(unknown)", "k": "int"}, renders(t, a.typeRegistry["NY"]),
		"NY is written from its fields: JM through its MarshalJSON, the field-less S adding nothing")
	for field, method := range map[string]string{"To": methodMarshalJSONTo, "From": methodUnmarshalJSONFrom} {
		w := fieldWarnings(a, field)
		if assert.Len(t, w, 1, field) {
			assert.Contains(t, w[0], "has its own "+method+" method", field)
		}
	}
	assert.Empty(t, fieldWarnings(a, "ToBad"))
}

// ifCopiesFiles returns a project whose package mod declares BT, BT2, BTSame
// and BTNone in two build-tagged files named linux and other, and a Rows
// holding them and an embed of BT2.
func ifCopiesFiles(linux, other string) map[string]string {
	return map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", linux): "//go:build linux\n\npackage mod\n\n" +
			"type BT interface{ MarshalJSON() ([]byte, error) }\n\n" +
			"type BT2 interface{ MarshalJSON() ([]byte, error) }\n\n" +
			"type BTSame interface{ MarshalJSON() ([]byte, error) }\n\n" +
			"type BTNone interface{ Name() string }\n",
		filepath.Join("mod", other): "//go:build !linux\n\npackage mod\n\n" +
			"type BT struct {\n\tX int `json:\"x\"`\n}\n\n" +
			"type BT2 interface{ Name() string }\n\n" +
			"type BTSame interface {\n\tMarshalJSON() ([]byte, error)\n}\n\n" +
			"type BTNone struct {\n\tX int `json:\"x\"`\n}\n",
		filepath.Join("mod", "module.go"): resolveModuleHead +
			"\ntype EmbBT2 struct {\n\tBT2\n\tE int `json:\"e\"`\n}\n\ntype ABT = BT\n\ntype DBT BT\n\n" +
			"type Rows struct {\n\tBT BT `json:\"bt\"`\n\tBT2 BT2 `json:\"bt2\"`\n\tBTSame BTSame `json:\"btSame\"`\n" +
			"\tBTNone BTNone `json:\"btNone\"`\n\tEmb EmbBT2 `json:\"emb\"`\n\tPBT *BT `json:\"pbt\"`\n" +
			"\tABT ABT `json:\"abt\"`\n\tDBT DBT `json:\"dbt\"`\n}\n\n" +
			"func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" +
			"\tserver.GET(hr, r, \"/rows\", m.rows)\n\tserver.GET(hr, r, \"/bt\", m.bt)\n" +
			"\tserver.GET(hr, r, \"/abt\", m.abt)\n\tserver.GET(hr, r, \"/dbt\", m.dbt)\n}\n\n" +
			"func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) { panic(0) }\n\n" +
			"func (m *Module) bt(ctx server.HandlerContext) (server.Result[BT], server.IAPIError) { panic(0) }\n\n" +
			"func (m *Module) abt(ctx server.HandlerContext) (server.Result[ABT], server.IAPIError) { panic(0) }\n\n" +
			"func (m *Module) dbt(ctx server.HandlerContext) (server.Result[DBT], server.IAPIError) { panic(0) }\n",
	}
}

// TestInterfaceCopiesFileOrderIndependent pins build-tagged copies of one
// name that disagree on an interface's marshal methods (#179): an interface
// with MarshalJSON beside a struct (BT), or beside an interface without it
// (BT2), is an untyped object with the disagreement warning, and promotes
// nothing (EmbBT2 keeps its component), whichever file sorts first. Copies
// declaring the same interface methods (BTSame) agree, and copies none of
// which holds a marshal method (BTNone) resolve as before.
func TestInterfaceCopiesFileOrderIndependent(t *testing.T) {
	type result struct {
		rows, emb  map[string]string
		warnings   []string
		registered []string
	}
	run := func(linux, other string) result {
		a := analyzeDirectiveProject(t, ifCopiesFiles(linux, other))
		ws := slices.Clone(a.warnings)
		for i, w := range ws {
			ws[i] = strings.NewReplacer(linux, "<linux>", other, "<other>").Replace(w)
		}
		slices.Sort(ws)
		return result{rows: renders(t, a.typeRegistry["Rows"]), emb: renders(t, a.typeRegistry["EmbBT2"]), warnings: ws,
			registered: slices.Sorted(maps.Keys(a.typeRegistry))}
	}
	first := run("bt_linux.go", "bt_other.go")
	assert.Equal(t, first, run("z_linux.go", "a_other.go"), "file order must not decide the output")
	assert.Equal(t, "BT", first.rows["bt"])
	assert.Equal(t, "BT2", first.rows["bt2"])
	assert.Equal(t, "*BT", first.rows["pbt"])
	assert.Equal(t, "marshal:BTSame(unknown)", first.rows["btSame"])
	assert.Equal(t, "$BTNone", first.rows["btNone"])
	assert.Equal(t, "$EmbBT2", first.rows["emb"])
	assert.Equal(t, "BT2", first.emb["BT2"])
	// An alias or defined type over BT reaches BT's struct copy, but follows
	// BT's disagreement, which its warning names.
	assert.Equal(t, "ABT", first.rows["abt"])
	assert.Equal(t, "DBT", first.rows["dbt"])
	// BT2 warns twice: as a Rows field and as EmbBT2's embedded field.
	for field, n := range map[string]int{"BT": 1, "BT2": 2, "PBT": 1, "ABT": 1, "DBT": 1} {
		w := fieldWarnings(&ProjectAnalyzer{warnings: first.warnings}, field)
		if assert.Len(t, w, n, field) {
			assert.Contains(t, w[0], "build-tagged declarations disagree on shape ("+declsInterfaceCause, field)
		}
	}
	for _, payload := range []string{"BT", "ABT", "DBT"} {
		assert.Contains(t, first.warnings, fmt.Sprintf(declsDisagreePayloadWarning, payload, "BT", declsInterfaceCause), payload)
	}
	assert.Len(t, first.warnings, 10, "and BTSame's Marshaler warning: %q", first.warnings)
	assert.Equal(t, []string{"BTNone", "EmbBT2", "Rows"}, first.registered, "no copy-disagreeing type registers")
}
