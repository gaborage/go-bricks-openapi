package analyzer

import (
	"fmt"
	"go/parser"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// genericBaseB is package b of the generic-base embed tests.
const genericBaseB = `package b

import "encoding/json"

type RawOf[T any] = json.RawMessage
type Base struct {
	ID int ` + "`json:\"id\"`" + `
}
type Ptr[T any] = *Base
type QM[T any] []T

func (QM[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type List[T any] []T
type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}
`

// genericBaseDecls are the named types the generic-base embed rows embed.
const genericBaseDecls = `
type list[T any] []T
type box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}
type rawOf[T any] = json.RawMessage
type RawOf[T any] = json.RawMessage
type RA = rawOf[int]
type ltOf[T any] time.Time
type ll = list[int]
type ol = b.List[int]
type lsd list[int]
type LL = list[int]
type rawChain rawOf[int]
type ltA = ltOf[int]
type bb = box[int]
type qb = b.Box[int]
type gl = gone[int]
type qm = b.Missing[int]
type ng = nsOf[int]
type nsOf[T any] sql.NullString
type Seq[T any] []T
type sq = Seq[int]
type ltAl[T any] = time.Time
type ltD ltAl[int]
type raw = b.RawOf[int]
type hiddenP = b.Ptr[int]
type plain struct {
	P int ` + "`json:\"p\"`" + `
}
type ptrOf[T any] = *plain
type mj[T any] []T

func (mj[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type mp[T any] []T

func (*mp[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type mt[K comparable, V any] map[K]V

func (mt[K, V]) MarshalText() ([]byte, error) { return nil, nil }

type Mj[T any] []T

func (Mj[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type mbad[T any] []T

func (mbad[T]) MarshalJSON() string { return "" }

type mjA = mj[int]
type mjD mj[int]
type pv[T any] []T

func (v (pv[T])) MarshalJSON() ([]byte, error) { return nil, nil }

type pp[T any] []T

func (v *(pp[T])) MarshalJSON() ([]byte, error) { return nil, nil }

type pq[T any] []T

func (v (*pq[T])) MarshalJSON() ([]byte, error) { return nil, nil }
`

// analyzeGenericBaseRows analyzes a project whose Rows type holds one field
// per row struct, each embedding its embed beside its own field Y.
func analyzeGenericBaseRows(t *testing.T, rows []embedRow) *ProjectAnalyzer {
	t.Helper()
	var decls, fields strings.Builder
	decls.WriteString(genericBaseDecls)
	for _, r := range rows {
		fmt.Fprintf(&decls, "type %s struct {\n\t%s\n\tY int `json:\"y\"`\n}\n", r.typ, r.embed)
		fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", r.typ, r.typ, strings.ToLower(r.typ))
	}
	imports := "\t\"database/sql\"\n\t\"encoding/json\"\n\t\"time\"\n\t\"github.com/example/app/b\"\n"
	return analyzeDirectiveProject(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("b", "b.go"):        genericBaseB,
		filepath.Join("mod", "module.go"): rowsModule(imports, decls.String(), fields.String()),
	})
}

// assertPromotedMarshaler asserts that the row struct typ embeds a type
// whose marshal method it promotes (issue 114 reads generic receivers and
// sees through an instantiation): it is no component, its Rows field is a
// Marshaler leaf, and that field raises the promoted-method warning naming
// via, the embed as written.
func assertPromotedMarshaler(t *testing.T, a *ProjectAnalyzer, typ, method, via string) {
	t.Helper()
	assert.NotContains(t, a.typeRegistry, typ)
	f := fieldByJSON(t, a.typeRegistry["Rows"], strings.ToLower(typ))
	require.NotNil(t, f.Resolution, typ)
	assert.Equal(t, models.ShapeMarshaler, f.Resolution.Kind, typ)
	want := fmt.Sprintf("has type %s: %s has the %s method promoted from its embedded %s,", typ, typ, method, via)
	assert.Len(t, matching(a.warnings, want), 1, "%q in %v", want, a.warnings)
}

// TestEmbeddedGenericNonStructBase pins issue 191, as issue 114 resolves it:
// a generic instantiation reached through an alias or a defined type (ll,
// lsd), or over an in-module generic type (ol, b.List[int]), has its generic
// type read. A non-struct one embedded under an unexported name, json-tagged
// or not, a chain through a generic alias of json.RawMessage (rawChain)
// included, adds no property and no warning, as encoding/json skips it, over
// an exported generic type too (sq); one over time.Time (ltA), or a defined
// type over a generic alias of it (ltD, which drops its methods), adds
// nothing. An exported one (LL, b.List[int], Seq[int]) is one property typed
// with its substituted type, with no warning. A struct base reached through
// an alias (bb, qb) promotes its substituted fields. An undeclared one (gl,
// qm) and one from outside the module (ng) keep the dropped-embed warning.
// An exported instantiation of a generic alias of a non-struct Marshaler
// (RawOf[int], RA, b.RawOf[int]) promotes its MarshalJSON, so the embedding
// struct is a Marshaler type, as it is with the non-generic alias.
func TestEmbeddedGenericNonStructBase(t *testing.T) {
	rows := []embedRow{
		{"GLl", "ll", nil},
		{"GOl", "ol", nil},
		{"GLsd", "lsd", nil},
		{"GLlTagged", "ll `json:\"ll\"`", nil},
		{"GOlTagged", "ol `json:\"ol\"`", nil},
		{"GLsdTagged", "lsd `json:\"lsd\"`", nil},
		{"GPtrLsd", "*lsd", nil},
		{"GList", "list[int]", nil},
		{"GRawChain", "rawChain", nil},
		{"GLtA", "ltA", nil},
		{"GExpLL", "LL", map[string]string{"LL": "[]int"}},
		{"GQList", "b.List[int]", map[string]string{"List": "[]int"}},
		{"GBb", "bb", map[string]string{"v": "int"}},
		{"GQb", "qb", map[string]string{"v": "int"}},
		{"GGl", "gl", nil},
		{"GQm", "qm", nil},
		{"GNg", "ng", nil},
		{"GSq", "sq", nil},
		{"GLtD", "ltD", nil},
		{"GSeq", "Seq[int]", map[string]string{"Seq": "[]int"}},
	}
	marshalers := []embedRow{{"GRawOfExp", "RawOf[int]", nil}, {"GRA", "RA", nil}, {"GQRawOf", "b.RawOf[int]", nil}}
	a := analyzeGenericBaseRows(t, slices.Concat(rows, marshalers))
	for _, r := range rows {
		want := map[string]string{"y": goTypeInt}
		for k, v := range r.want {
			want[k] = v
		}
		assert.Equal(t, want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	assertPromotedMarshaler(t, a, "GRawOfExp", "MarshalJSON", "RawOf[int]")
	assertPromotedMarshaler(t, a, "GRA", "MarshalJSON", "RA")
	assertPromotedMarshaler(t, a, "GQRawOf", "MarshalJSON", "b.RawOf[int]")
	warnings := embedWarnings(a)
	for _, want := range []string{
		"has type gl: gone[int] is a generic instantiation",
		"has type qm: b.Missing[int] is a generic instantiation",
		"has type ng: nsOf[int] is a generic instantiation",
	} {
		assert.Len(t, matching(warnings, want), 1, "%q in %v", want, warnings)
	}
	assert.Len(t, warnings, 3, "%v", warnings)
	all := a.Warnings(t.Context())
	assert.Len(t, all, 6, "the three dropped embeds and the three Marshaler fields' warnings: %v", all)
}

// TestGenericEmbedMarshalerAndPointerWarn pins that an unexported embed is
// skipped only when its type is known not to be a struct and to promote no
// marshal method. An instantiation of a generic alias of json.RawMessage
// reached with no defined type on the way (rawOf[int], raw = b.RawOf[int])
// keeps MarshalJSON, which makes the embedding struct a Marshaler type,
// json-tagged or not, exactly as the non-generic alias of json.RawMessage
// does (issue 215), while a defined type on the way (rawChain) drops the
// methods and is skipped. A generic alias of a pointer (ptrOf[int], hiddenP
// = b.Ptr[int]) may point at a struct, which encoding/json promotes, or
// keeps under a json name, however its embed is named: it is warned
// untagged and nests under a json name.
func TestGenericEmbedMarshalerAndPointerWarn(t *testing.T) {
	rows := []embedRow{
		{"MRawChain", "rawChain", nil},
		{"MPtrOf", "ptrOf[int]", nil},
		{"MHiddenP", "hiddenP", nil},
		{"MHiddenPTagged", "hiddenP `json:\"base\"`", map[string]string{"base": "*$Base"}},
	}
	marshalers := []embedRow{
		{"MRawOfLower", "rawOf[int]", nil},
		{"MRawOfTagged", "rawOf[int] `json:\"r\"`", nil},
		{"MRaw", "raw", nil},
		{"MRawTagged", "raw `json:\"r\"`", nil},
	}
	a := analyzeGenericBaseRows(t, slices.Concat(rows, marshalers))
	for _, r := range rows {
		want := map[string]string{"y": goTypeInt}
		for k, v := range r.want {
			want[k] = v
		}
		assert.Equal(t, want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	assertPromotedMarshaler(t, a, "MRawOfLower", "MarshalJSON", "rawOf[int]")
	assertPromotedMarshaler(t, a, "MRawOfTagged", "MarshalJSON", "rawOf[int]")
	assertPromotedMarshaler(t, a, "MRaw", "MarshalJSON", "raw")
	assertPromotedMarshaler(t, a, "MRawTagged", "MarshalJSON", "raw")
	warnings := embedWarnings(a)
	for _, want := range []string{
		"has type ptrOf[int]: ptrOf[int] is a generic instantiation",
		"has type hiddenP: b.Ptr[int] is a generic instantiation",
	} {
		assert.Len(t, matching(warnings, want), 1, "%q in %v", want, warnings)
	}
	assert.Len(t, warnings, 2, "%v", warnings)
	assert.Len(t, a.Warnings(t.Context()), 6, "the two dropped embeds and the four Marshaler fields' warnings: %v", a.warnings)
}

// TestGenericEmbedOwnMarshalerWarns pins that an instantiation of a generic
// non-struct type declaring one of the marshal methods (on the value or the
// pointer receiver, local or in-module, a parenthesized receiver included)
// makes the embedding struct a Marshaler type, exported embed or not and
// through an alias (mjA): issue 114 reads generic receivers. A defined type
// on the way (mjD) drops the methods, and a method whose signature is not the
// interface's (mbad) is no marshal method: both are skipped silently when
// unexported.
func TestGenericEmbedOwnMarshalerWarns(t *testing.T) {
	a := analyzeGenericBaseRows(t, []embedRow{
		{"OMj", "mj[int]", nil},
		{"OMp", "mp[int]", nil},
		{"OMt", "mt[string, int]", nil},
		{"OMjExp", "Mj[int]", nil},
		{"OQM", "b.QM[int]", nil},
		{"OMjA", "mjA", nil},
		{"OMjD", "mjD", nil},
		{"OMbad", "mbad[int]", nil},
		{"OPv", "pv[int]", nil},
		{"OPp", "pp[int]", nil},
		{"OPq", "pq[int]", nil},
	})
	for _, typ := range []string{"OMjD", "OMbad"} {
		assert.Equal(t, map[string]string{"y": goTypeInt}, renders(t, a.typeRegistry[typ]), typ)
	}
	for typ, via := range map[string]string{
		"OMj": "mj[int]", "OMp": "mp[int]", "OMjExp": "Mj[int]", "OQM": "b.QM[int]", "OMjA": "mjA",
		"OPv": "pv[int]", "OPp": "pp[int]", "OPq": "pq[int]",
	} {
		assertPromotedMarshaler(t, a, typ, "MarshalJSON", via)
	}
	assertPromotedMarshaler(t, a, "OMt", "MarshalText", "mt[string, int]")
	assert.Empty(t, embedWarnings(a))
	assert.Len(t, a.Warnings(t.Context()), 9, "one per Marshaler field, no other: %v", a.warnings)
}

// TestGenericEmbedKeepsDepthCut pins that generic carries its chain's depth:
// reached at the last level maxNamedResolutionDepth allows, the named type
// its generic type is declared over is cut like any too-deep chain (a
// non-struct, left to the property's Resolution), rather than read afresh
// as the struct it is. Below that level, a generic type declared over a
// named struct promotes that struct's fields (issue 114).
func TestGenericEmbedKeepsDepthCut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	src := "package a\n\ntype Base struct {\n\tID int\n}\ntype baseOf[T any] Base\n"
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	a := New(dir)
	file, err := parser.ParseFile(a.fileSet, path, src, parser.ParseComments)
	require.NoError(t, err)
	expr, err := parser.ParseExpr("baseOf[int]")
	require.NoError(t, err)
	ec := embedClassifier{a: a, home: dir}

	assert.Equal(t, embedStruct, ec.generic(file, path, expr, 0, false, nil).class, "a struct base is promoted")
	assert.Equal(t, embedNonStruct, ec.generic(file, path, expr, maxNamedResolutionDepth-1, false, nil).class)
}
