package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// parseInDir parses src as a file named name inside dir (which need not hold
// it on disk) and returns the analyzer, the file and its path.
func parseInDir(t *testing.T, a *ProjectAnalyzer, dir, name, src string) (file *ast.File, path string) {
	t.Helper()
	path = filepath.Join(dir, name)
	file, err := parser.ParseFile(a.fileSet, path, src, 0)
	require.NoError(t, err)
	return file, path
}

// marshalerSetOf parses src in a fresh temp dir and returns what T's own
// methods say about its JSON form.
func marshalerSetOf(t *testing.T, src string) marshalerMethods {
	t.Helper()
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "t.go", src)
	return a.marshalerSet("T", newResolveCtx(file, path, false))
}

// TestMarshalerSetSevenMethods pins each of the seven encoding/json methods,
// on a value and on a pointer receiver, and which side it is on.
func TestMarshalerSetSevenMethods(t *testing.T) {
	cases := []struct {
		method string
		decl   string // the method after "func (<recv>) "
		encode bool
	}{
		{methodMarshalJSON, "MarshalJSON() ([]byte, error) { return nil, nil }", true},
		{methodMarshalText, "MarshalText() ([]byte, error) { return nil, nil }", true},
		{methodMarshalJSONTo, "MarshalJSONTo(enc *jsontext.Encoder) error { return nil }", true},
		{methodAppendText, "AppendText(b []byte) ([]byte, error) { return b, nil }", true},
		{methodUnmarshalJSON, "UnmarshalJSON(b []byte) error { return nil }", false},
		{methodUnmarshalText, "UnmarshalText(b []uint8) error { return nil }", false},
		{methodUnmarshalJSONFrom, "UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }", false},
	}
	for _, tc := range cases {
		for _, recv := range []string{"t T", "t *T"} {
			t.Run(tc.method+"/"+recv, func(t *testing.T) {
				src := "package p\n\nimport \"encoding/json/jsontext\"\n\nvar _ jsontext.Value\n\ntype T int\n\nfunc (" + recv + ") " + tc.decl + "\n"
				got := marshalerSetOf(t, src)
				assert.True(t, got.found())
				assert.Equal(t, tc.encode, got.encode)
				assert.Equal(t, !tc.encode, got.decode)
				assert.Equal(t, tc.method, got.first)
			})
		}
	}

	t.Run("aliased jsontext import", func(t *testing.T) {
		src := "package p\n\nimport jt \"encoding/json/jsontext\"\n\ntype T int\n\n" +
			"func (T) MarshalJSONTo(enc *jt.Encoder) error { return nil }\n" +
			"func (*T) UnmarshalJSONFrom(dec *jt.Decoder) error { return nil }\n"
		got := marshalerSetOf(t, src)
		assert.True(t, got.encode)
		assert.True(t, got.decode)
		assert.Equal(t, methodMarshalJSONTo, got.first)
	})
}

// TestMarshalerSetWrongSignatures pins that only the exact encoding/json
// signatures count: a same-named method with any other shape is ignored.
func TestMarshalerSetWrongSignatures(t *testing.T) {
	cases := map[string]string{
		"MarshalText returns string":   "type T int\nfunc (T) MarshalText() string { return \"\" }\n",
		"MarshalJSON one result":       "type T int\nfunc (T) MarshalJSON() []byte { return nil }\n",
		"MarshalJSON wrong 2nd result": "type T int\nfunc (T) MarshalJSON() ([]byte, string) { return nil, \"\" }\n",
		"MarshalJSON takes a param":    "type T int\nfunc (T) MarshalJSON(x int) ([]byte, error) { return nil, nil }\n",
		"UnmarshalJSON takes string":   "type T int\nfunc (*T) UnmarshalJSON(s string) error { return nil }\n",
		"UnmarshalJSON no result":      "type T int\nfunc (*T) UnmarshalJSON(b []byte) {}\n",
		"AppendText no param":          "type T int\nfunc (T) AppendText() ([]byte, error) { return nil, nil }\n",
		"MarshalJSONTo json.Encoder":   "import \"encoding/json\"\ntype T int\nfunc (T) MarshalJSONTo(enc *json.Encoder) error { return nil }\n",
		"MarshalJSONTo foreign jsontext": "import \"github.com/go-json-experiment/json/jsontext\"\ntype T int\n" +
			"func (T) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }\n",
		"MarshalJSONTo value encoder": "import \"encoding/json/jsontext\"\ntype T int\nfunc (T) MarshalJSONTo(enc jsontext.Encoder) error { return nil }\n",
		"method on another type":      "type T int\ntype U int\nfunc (U) MarshalJSON() ([]byte, error) { return nil, nil }\n",
		"plain function":              "type T int\nfunc MarshalJSON() ([]byte, error) { return nil, nil }\n",
		"array not slice":             "type T int\nfunc (*T) UnmarshalJSON(b [4]byte) error { return nil }\n",
		"unrelated method":            "type T int\nfunc (T) String() string { return \"\" }\n",
		"MarshalJSONTo local Encoder": "type T int\ntype Encoder struct{}\nfunc (T) MarshalJSONTo(enc *Encoder) error { return nil }\n",
		"MarshalJSONTo jsontext.Decoder": "import \"encoding/json/jsontext\"\ntype T int\n" +
			"func (T) MarshalJSONTo(enc *jsontext.Decoder) error { return nil }\n",
		"slice of other elem": "type T int\nfunc (*T) UnmarshalJSON(b []int) error { return nil }\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got := marshalerSetOf(t, "package p\n\n"+body)
			assert.False(t, got.found(), "%+v", got)
		})
	}
}

// TestMarshalerSetNamedResults pins that named results count, and that a
// single field list entry naming two params is two params.
func TestMarshalerSetNamedResults(t *testing.T) {
	got := marshalerSetOf(t, "package p\n\ntype T int\n\nfunc (T) MarshalJSON() (b []byte, err error) { return nil, nil }\n")
	assert.True(t, got.encode)

	got = marshalerSetOf(t, "package p\n\ntype T int\n\nfunc (T) UnmarshalJSON(a, b []byte) error { return nil }\n")
	assert.False(t, got.found())
}

// TestMarshalerSetSiblingFileAndOtherPackage pins the file scan: a sibling
// same-package file counts, another package in the directory does not, and an
// unreadable directory still scans the current file.
func TestMarshalerSetSiblingFileAndOtherPackage(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "m.go"),
		[]byte("package p\n\nfunc (*T) UnmarshalText(b []byte) error { return nil }\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gen.go"),
		[]byte("//go:build ignore\n\npackage main\n\nfunc (T) MarshalJSON() ([]byte, error) { return nil, nil }\n"), 0o600))
	a := New(dir)
	file, path := parseInDir(t, a, dir, "t.go", "package p\n\ntype T int\n")
	got := a.marshalerSet("T", newResolveCtx(file, path, false))
	assert.True(t, got.decode, "the sibling same-package method counts")
	assert.False(t, got.encode, "a package main sibling does not")
	assert.Equal(t, methodUnmarshalText, got.first)

	gone := filepath.Join(dir, "missing")
	own, ownPath := parseInDir(t, a, gone, "t.go", "package p\n\ntype T int\n\nfunc (T) MarshalText() ([]byte, error) { return nil, nil }\n")
	got = a.marshalerSet("T", newResolveCtx(own, ownPath, false))
	assert.True(t, got.encode, "an unreadable dir still scans the current file")
}

// TestFieldTypes pins the field list flattening the signature matcher uses.
func TestFieldTypes(t *testing.T) {
	assert.Empty(t, fieldTypes(nil))
	fd := func(src string) *ast.FuncDecl {
		f, err := parser.ParseFile(New(t.TempDir()).fileSet, "x.go", "package p\n"+src, 0)
		require.NoError(t, err)
		return f.Decls[0].(*ast.FuncDecl)
	}
	assert.Len(t, fieldTypes(fd("func f(a, b []byte) {}").Type.Params), 2)
	assert.Len(t, fieldTypes(fd("func f() ([]byte, error) { return nil, nil }").Type.Results), 2)
	assert.Empty(t, fieldTypes(fd("func f() {}").Type.Results))
}

// TestMarshalerCandidateUnknownName pins that a method outside the seven is
// never a candidate, whatever its signature.
func TestMarshalerCandidateUnknownName(t *testing.T) {
	a := New(t.TempDir())
	f, err := parser.ParseFile(a.fileSet, "x.go", "package p\nfunc (T) Marshal() ([]byte, error) { return nil, nil }\n", 0)
	require.NoError(t, err)
	fd, _ := a.marshalerCandidate(f.Decls[0], "T")
	assert.Nil(t, fd)
}

// TestTextBothWays pins the string rule (#111 as amended): MarshalText and
// UnmarshalText, with no JSON-specific method on either side; AppendText
// neither helps nor hurts.
func TestTextBothWays(t *testing.T) {
	const (
		mText   = "func (T) MarshalText() ([]byte, error) { return nil, nil }\n"
		mTextP  = "func (*T) MarshalText() ([]byte, error) { return nil, nil }\n"
		uText   = "func (*T) UnmarshalText(b []byte) error { return nil }\n"
		appText = "func (T) AppendText(b []byte) ([]byte, error) { return b, nil }\n"
		mJSON   = "func (T) MarshalJSON() ([]byte, error) { return nil, nil }\n"
		mJSONTo = "func (T) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }\n"
		uJSON   = "func (*T) UnmarshalJSON(b []byte) error { return nil }\n"
		uFrom   = "func (*T) UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }\n"
	)
	cases := []struct {
		name    string
		methods string
		want    bool
	}{
		{"pair", mText + uText, true},
		{"pair on pointer", mTextP + uText, false},
		{"pair and AppendText", mText + appText + uText, true},
		{"AppendText and UnmarshalText", appText + uText, false},
		{"MarshalJSONTo and pair", mJSONTo + mText + uText, false},
		{"MarshalJSON and pair", mJSON + mText + uText, false},
		{"UnmarshalJSON and pair", uJSON + mText + uText, false},
		{"UnmarshalJSONFrom and pair", uFrom + mText + uText, false},
		{"UnmarshalJSONFrom only", uFrom, false},
		{"MarshalText only", mText, false},
		{"UnmarshalText only", uText, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package p\n\nimport \"encoding/json/jsontext\"\n\nvar _ jsontext.Value\n\ntype T int\n\n" + tc.methods
			assert.Equal(t, tc.want, marshalerSetOf(t, src).textBothWays())
		})
	}
	got := marshalerSetOf(t, "package p\n\ntype T int\n\n"+mText+appText+uText)
	assert.Equal(t, bitMarshalText|bitAppendText|bitUnmarshalText, got.has)
}

// TestPromote pins Go's selector rules over embedded method sets: the
// shallowest depth wins, a tie there is ambiguous and hides deeper entries,
// and exactness propagates.
func TestPromote(t *testing.T) {
	exact := func(depth int) methodEntry { return methodEntry{depth: depth, exact: true} }
	t.Run("unique", func(t *testing.T) {
		got := promote([]embedMethods{{via: "Level", set: methodSet{methodMarshalText: exact(0)}}})
		assert.Equal(t, methodEntry{depth: 1, exact: true, via: "Level"}, got[methodMarshalText])
	})
	t.Run("tie", func(t *testing.T) {
		got := promote([]embedMethods{
			{via: "Level", set: methodSet{methodMarshalText: exact(0)}},
			{via: "StatusV", set: methodSet{methodMarshalText: exact(0)}},
		})
		assert.Equal(t, methodEntry{depth: 1, ambiguous: true}, got[methodMarshalText])
	})
	t.Run("shallowest wins in any order", func(t *testing.T) {
		deep := embedMethods{via: "WrapLevel", set: methodSet{methodMarshalText: exact(1)}}
		shallow := embedMethods{via: "StatusV", set: methodSet{methodMarshalText: exact(0)}}
		for _, embeds := range [][]embedMethods{{deep, shallow}, {shallow, deep}} {
			assert.Equal(t, methodEntry{depth: 1, exact: true, via: "StatusV"}, promote(embeds)[methodMarshalText])
		}
	})
	t.Run("ambiguous hides deeper", func(t *testing.T) {
		got := promote([]embedMethods{
			{via: "X", set: methodSet{methodMarshalText: {depth: 0, ambiguous: true}}},
			{via: "Y", set: methodSet{methodMarshalText: exact(1)}},
		})
		assert.True(t, got[methodMarshalText].ambiguous)
		assert.Equal(t, 1, got[methodMarshalText].depth)
	})
	t.Run("inexact propagates", func(t *testing.T) {
		got := promote([]embedMethods{{via: "Odd", set: methodSet{methodMarshalJSON: {depth: 0}}}})
		assert.False(t, got[methodMarshalJSON].exact)
	})
}

// TestMethodSetOverAndMarshaler pins how a method set is merged and read.
func TestMethodSetOverAndMarshaler(t *testing.T) {
	own := methodSet{methodMarshalText: {depth: 0, exact: true, owner: "T"}}
	got := own.over(methodSet{
		methodMarshalText:   {depth: 1, exact: true, via: "Deeper"},
		methodUnmarshalText: {depth: 1, exact: true, via: "Deeper"},
	})
	assert.Equal(t, "T", got[methodMarshalText].owner, "the shallower entry is kept")
	assert.Equal(t, "Deeper", got[methodUnmarshalText].via)

	m, fb := methodSet{
		methodUnmarshalJSON: {depth: 0, exact: true, owner: "S"},
		methodMarshalText:   {depth: 1, exact: true, via: "Level"},
		methodMarshalJSON:   {depth: 0, owner: "S"},
		methodAppendText:    {depth: 1, ambiguous: true, exact: true},
	}.marshaler("S")
	assert.True(t, m.encode)
	assert.True(t, m.decode)
	assert.Equal(t, bitMarshalText|bitUnmarshalJSON, m.has, "inexact and ambiguous entries do not count")
	assert.Equal(t, fieldFallback{kind: fallbackMarshalerPromoted, typeName: "S", detail: methodMarshalText, via: "Level"}, fb,
		"MarshalText is read before UnmarshalJSON")

	_, fb = methodSet{methodMarshalJSON: {depth: 0, exact: true, owner: mtMoney}}.marshaler("MA")
	assert.Equal(t, fieldFallback{kind: fallbackMarshaler, typeName: mtMoney, detail: methodMarshalJSON}, fb)

	m, fb = methodSet{}.marshaler("E")
	assert.False(t, m.found())
	assert.Equal(t, fieldFallback{}, fb)
}

// methodSetRow is one named type's method set as methodSetOf reads it.
type methodSetRow struct {
	name     string
	found    bool
	text     bool
	kind     fallbackKind
	typeName string
	detail   string
	via      string
}

// assertMethodSetRows checks each row against methodSetOf on a project
// written by writeMTProject.
func assertMethodSetRows(t *testing.T, rows []methodSetRow) {
	t.Helper()
	dir := writeMTProject(t, mtModuleHead)
	a := New(dir)
	a.modulePath = "github.com/example/app"
	file, path := parseInDir(t, a, filepath.Join(dir, "mod"), "types.go", mtTypesSrc)
	for _, r := range rows {
		c := newResolveCtx(file, path, false)
		m, fb := a.methodSetOf(r.name, c, true, nil).marshaler(c.display(r.name))
		assert.Equal(t, r.found, m.found(), r.name)
		assert.Equal(t, r.text, m.textBothWays(), r.name)
		if r.found && !r.text {
			assert.Equal(t, fieldFallback{kind: r.kind, typeName: r.typeName, detail: r.detail, via: r.via}, fb, r.name)
		}
	}
}

// TestMethodSetOfOwnAndAliased pins declared methods, aliases, defined types
// and text structs.
func TestMethodSetOfOwnAndAliased(t *testing.T) {
	own := func(name, typeName, method string) methodSetRow {
		return methodSetRow{name: name, found: true, kind: fallbackMarshaler, typeName: typeName, detail: method}
	}
	text := func(name string) methodSetRow { return methodSetRow{name: name, found: true, text: true} }
	assertMethodSetRows(t, []methodSetRow{
		own(mtMoney, mtMoney, methodMarshalJSON),
		own("MoneyP", "MoneyP", methodMarshalJSON),
		own("MoneyU", "MoneyU", methodUnmarshalJSON),
		{name: "MoneyOdd"},
		own("MA", mtMoney, methodMarshalJSON),
		{name: "MD"},
		text("MDW"),
		text(mtMoneyT),
		own("Shadow", "Shadow", methodMarshalJSON),
		{name: "StampDEmb"},
		{name: "GenEmb"},
		{name: "TagEmb"},
		{name: "IntEmb"},
		{name: "Node"},
		{name: "A"},
		{name: "Dec"},
	})
}

// TestMethodSetOfPromoted pins promotion: by value, pointer, json-tagged or
// excluded embed, depth (uncapped), ties, shadowing, well-known and
// cross-package embeds, and pointer-only MarshalText, which only a pointer
// embed puts in the value method set.
func TestMethodSetOfPromoted(t *testing.T) {
	promoted := func(name, method, via string) methodSetRow {
		return methodSetRow{name: name, found: true, kind: fallbackMarshalerPromoted, typeName: name, detail: method, via: via}
	}
	text := func(name string) methodSetRow { return methodSetRow{name: name, found: true, text: true} }
	assertMethodSetRows(t, []methodSetRow{
		promoted("EmbP", methodMarshalText, "LevelP"),
		text("PtrEmbP"),
		promoted("EmbEmbP", methodMarshalText, "EmbP"),
		text("PtrEmbEmbP"),
		{name: "TimeTie"},
		text(mtWrapLevel),
		text("PtrEmb"),
		text("TaggedEmb"),
		text("Excl"),
		text("Deep"),
		text("Near"),
		text("WithID"),
		text("WithCode"),
		promoted("Wrapper", methodMarshalText, mtStatusV),
		promoted("Twin", methodUnmarshalText, mtLevel),
		promoted("OddShadow", methodUnmarshalText, mtLevel),
		promoted("Stamped", methodMarshalJSON, models.WellKnownTimeTime),
		promoted("StampA", methodMarshalJSON, "TA"),
		promoted("Raw", methodMarshalJSON, models.WellKnownRawMessage),
		{name: "Pair"},
		text("E0"),
		text("E1"),
		text("E2"),
		promoted("FieldShadow", methodUnmarshalText, mtLevel),
		promoted("FieldOuter", methodUnmarshalText, "FieldMid"),
		promoted("FieldTie", methodUnmarshalText, mtLevel),
		promoted("EmbNamed", methodUnmarshalText, mtLevel),
	})
}

// TestEmbeddedFieldName pins the field name Go gives an embedded field.
func TestEmbeddedFieldName(t *testing.T) {
	for src, want := range map[string]string{
		"T": "T", "*T": "T", "pkg.T": "T", "*pkg.T": "T",
		"Box[int]": "Box", "*pkg.Pair[int, string]": "Pair", "[]T": "",
	} {
		e, err := parser.ParseExpr(src)
		require.NoError(t, err, src)
		assert.Equal(t, want, embeddedFieldName(e), src)
	}
}

// methodSetProject parses src as package p in a fresh temp dir and returns the
// analyzer and a context resolving in it.
func methodSetProject(t *testing.T, src string) (*ProjectAnalyzer, resolveCtx) {
	t.Helper()
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "t.go", "package p\n"+src)
	return a, newResolveCtx(file, path, false)
}

// TestMethodSetOfMemoizesDiamond pins that a lattice of embeds, every type
// embedding every type of the next level, is walked once per type rather than
// once per path (K^L walks).
func TestMethodSetOfMemoizesDiamond(t *testing.T) {
	const k, levels = 3, 5
	var b strings.Builder
	for l := range levels {
		for i := range k {
			fmt.Fprintf(&b, "type T%d_%d struct {\n", l, i)
			for j := range k {
				if l+1 < levels {
					fmt.Fprintf(&b, "\tT%d_%d `json:\"e%d\"`\n", l+1, j, j)
				}
			}
			b.WriteString("}\n")
		}
	}
	a, c := methodSetProject(t, b.String())
	m, _ := a.methodSetOf("T0_0", c, true, nil).marshaler("T0_0")
	assert.False(t, m.found())
	assert.Equal(t, 1+k*(levels-1), a.methodSetWalks, "each reachable type is walked once")

	a.methodSetOf("T0_0", c, true, nil)
	assert.Equal(t, 1+k*(levels-1), a.methodSetWalks, "a second call is recalled")
}

// mtLevelSrc declares Level with the text pair, a string both ways.
const mtLevelSrc = "type Level int\n" +
	"func (l Level) MarshalText() ([]byte, error) { return nil, nil }\n" +
	"func (l *Level) UnmarshalText(p []byte) error { return nil }\n"

// TestMethodSetOfCycleCutNotMemoized pins that a set a cycle cut shortened is
// not recalled: Y is first walked inside X, where its *X is cut, so that set
// lacks X's Level; reached again from W it must still promote Level's text
// pair through *X, as Go's shallowest-embedding rule says.
func TestMethodSetOfCycleCutNotMemoized(t *testing.T) {
	a, c := methodSetProject(t, mtLevelSrc+"type X struct {\n\tLevel\n\tY\n}\ntype Y struct{ *X }\ntype W struct{ Y }\n")
	m, _ := a.methodSetOf("X", c, true, nil).marshaler("X")
	assert.True(t, m.textBothWays(), "X promotes Level")
	m, _ = a.methodSetOf("W", c, true, nil).marshaler("W")
	assert.True(t, m.textBothWays(), "W promotes Level's text pair through Y's *X")
}

// TestMethodSetOfUncappedAndOrderFree pins that promotion has no depth cap
// (Go promotes through any depth, so a text pair twelve embeds down still
// makes M0 a string both ways) and that a set does not depend on which type
// was walked first: M0 then M4, and M4 then M0, read alike.
func TestMethodSetOfUncappedAndOrderFree(t *testing.T) {
	const levels = 12
	var b strings.Builder
	b.WriteString(mtLevelSrc)
	for i := range levels {
		next := fmt.Sprintf("M%d", i+1)
		if i+1 == levels {
			next = mtLevel
		}
		fmt.Fprintf(&b, "type M%d struct {\n\t%s\n\tG%d int\n}\n", i, next, i)
	}
	for _, order := range [][]string{{"M0", "M4"}, {"M4", "M0"}} {
		a, c := methodSetProject(t, b.String())
		for _, name := range order {
			m, _ := a.methodSetOf(name, c, true, nil).marshaler(name)
			assert.True(t, m.textBothWays(), "%s, walked in order %v", name, order)
		}
	}
}
