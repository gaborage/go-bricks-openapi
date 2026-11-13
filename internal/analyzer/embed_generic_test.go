package analyzer

import (
	"fmt"
	"go/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestEmbeddedGenericNonStructBase pins issue 191: a generic instantiation
// reached through an alias or a defined type (ll, lsd), or over an in-module
// generic type (ol, b.List[int]), has its generic type read. A non-struct
// one embedded under an unexported name, json-tagged or not, a chain through
// a generic alias of json.RawMessage (rawChain) included, adds no property
// and no warning, as encoding/json skips it, over an exported generic type
// too (sq); one over time.Time (ltA), or a defined type over a generic alias
// of it (ltD, which drops its methods), adds nothing. An exported one (LL,
// b.List[int], Seq[int]) is one property with only the
// unresolved-generic field warning. A struct base (bb, qb), an undeclared
// one (gl, qm) and one from outside the module (ng) keep the dropped-embed
// warning, and so does an exported instantiation of a generic alias of a
// non-struct Marshaler (RawOf[int], RA, b.RawOf[int]): its promoted methods
// make the embedding struct a Marshaler, which method-set reading does not
// see through an instantiation (TestGenericEmbedMarshalerAndPointerWarn
// pins the unexported ones).
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
		{"GExpLL", "LL", map[string]string{"LL": "inst:list[int]"}},
		{"GQList", "b.List[int]", map[string]string{"List": "inst:b.List[int]"}},
		{"GBb", "bb", nil},
		{"GQb", "qb", nil},
		{"GGl", "gl", nil},
		{"GQm", "qm", nil},
		{"GNg", "ng", nil},
		{"GSq", "sq", nil},
		{"GLtD", "ltD", nil},
		{"GSeq", "Seq[int]", map[string]string{"Seq": "inst:Seq[int]"}},
		{"GRawOfExp", "RawOf[int]", nil},
		{"GRA", "RA", nil},
		{"GQRawOf", "b.RawOf[int]", nil},
	}
	a := analyzeGenericBaseRows(t, rows)
	for _, r := range rows {
		want := map[string]string{"y": goTypeInt}
		for k, v := range r.want {
			want[k] = v
		}
		assert.Equal(t, want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	warnings := embedWarnings(a)
	for _, want := range []string{
		"has type bb: box[int] is a generic instantiation, which is not expanded",
		"has type qb: b.Box[int] is a generic instantiation",
		"has type gl: gone[int] is a generic instantiation",
		"has type qm: b.Missing[int] is a generic instantiation",
		"has type ng: nsOf[int] is a generic instantiation",
		"has type RawOf[int]: RawOf[int] is a generic instantiation",
		"has type RA: rawOf[int] is a generic instantiation",
		"has type b.RawOf[int]: b.RawOf[int] is a generic instantiation",
	} {
		assert.Len(t, matching(warnings, want), 1, "%q in %v", want, warnings)
	}
	assert.Len(t, warnings, 8, "%v", warnings)
	all := a.Warnings(t.Context())
	assert.Len(t, all, 11, "the eight dropped embeds and the three properties' field warnings: %v", all)
	assert.Len(t, fieldWarnings(a, "Seq"), 1, "%v", all)
	assert.Len(t, fieldWarnings(a, "LL"), 1, "%v", all)
	assert.Len(t, fieldWarnings(a, "List"), 1, "%v", all)
}

// TestGenericEmbedMarshalerAndPointerWarn pins that an unexported embed is
// skipped only when its type is known not to be a struct and to promote no
// marshal method. An instantiation of a generic alias of json.RawMessage
// reached with no defined type on the way (rawOf[int], raw = b.RawOf[int])
// keeps MarshalJSON, which makes the embedding struct a Marshaler that
// method-set reading does not see through an instantiation: it is warned
// untagged and nests under a json name, while a defined type on the way
// (rawChain) drops the methods and is skipped. A generic alias of a pointer
// (ptrOf[int], hiddenP = b.Ptr[int]) may point at a struct, which
// encoding/json promotes, or keeps under a json name, however its embed is
// named: it is warned untagged and nests under a json name too.
func TestGenericEmbedMarshalerAndPointerWarn(t *testing.T) {
	rows := []embedRow{
		{"MRawOfLower", "rawOf[int]", nil},
		{"MRawOfTagged", "rawOf[int] `json:\"r\"`", map[string]string{"r": "inst:rawOf[int]"}},
		{"MRaw", "raw", nil},
		{"MRawTagged", "raw `json:\"r\"`", map[string]string{"r": "inst:b.RawOf[int]"}},
		{"MRawChain", "rawChain", nil},
		{"MPtrOf", "ptrOf[int]", nil},
		{"MHiddenP", "hiddenP", nil},
		{"MHiddenPTagged", "hiddenP `json:\"base\"`", map[string]string{"base": "inst:b.Ptr[int]"}},
	}
	a := analyzeGenericBaseRows(t, rows)
	for _, r := range rows {
		want := map[string]string{"y": goTypeInt}
		for k, v := range r.want {
			want[k] = v
		}
		assert.Equal(t, want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	warnings := embedWarnings(a)
	for _, want := range []string{
		"has type rawOf[int]: rawOf[int] is a generic instantiation, which is not expanded",
		"has type raw: b.RawOf[int] is a generic instantiation",
		"has type ptrOf[int]: ptrOf[int] is a generic instantiation",
		"has type hiddenP: b.Ptr[int] is a generic instantiation",
	} {
		assert.Len(t, matching(warnings, want), 1, "%q in %v", want, warnings)
	}
	assert.Len(t, warnings, 4, "%v", warnings)
	// Each tagged row adds its nested field's own warning beside the
	// untagged row's embed warning.
	assert.Len(t, fieldWarnings(a, "raw"), 2, "%v", a.warnings)
	assert.Len(t, fieldWarnings(a, "hiddenP"), 2, "%v", a.warnings)
	assert.Len(t, fieldWarnings(a, "rawOf"), 2, "%v", a.warnings)
}

// TestGenericEmbedOwnMarshalerWarns pins that an instantiation of a generic
// non-struct type declaring one of the marshal methods (on the value or the
// pointer receiver, local or in-module) is warned, exported embed or not and
// through an alias (mjA): the promoted method makes the embedding struct a
// Marshaler, which method-set reading does not see through an
// instantiation. A defined type on the way (mjD) drops the methods, and a
// method whose signature is not the interface's (mbad) is no marshal method:
// both are skipped silently when unexported.
func TestGenericEmbedOwnMarshalerWarns(t *testing.T) {
	rows := []embedRow{
		{"OMj", "mj[int]", nil},
		{"OMp", "mp[int]", nil},
		{"OMt", "mt[string, int]", nil},
		{"OMjExp", "Mj[int]", nil},
		{"OQM", "b.QM[int]", nil},
		{"OMjA", "mjA", nil},
		{"OMjD", "mjD", nil},
		{"OMbad", "mbad[int]", nil},
	}
	a := analyzeGenericBaseRows(t, rows)
	for _, r := range rows {
		assert.Equal(t, map[string]string{"y": goTypeInt}, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	warnings := embedWarnings(a)
	for _, want := range []string{
		"has type mj[int]: mj[int] is a generic instantiation, which is not expanded",
		"has type mp[int]: mp[int] is a generic instantiation",
		"has type mt[string, int]: mt[string, int] is a generic instantiation",
		"has type Mj[int]: Mj[int] is a generic instantiation",
		"has type b.QM[int]: b.QM[int] is a generic instantiation",
		"has type mjA: mj[int] is a generic instantiation",
	} {
		assert.Len(t, matching(warnings, want), 1, "%q in %v", want, warnings)
	}
	assert.Len(t, warnings, 6, "%v", warnings)
	assert.Len(t, a.Warnings(t.Context()), 6, "no other warning")
}

// TestGenericEmbedKeepsDepthCut pins that generic carries its chain's depth:
// reached at the last level maxNamedResolutionDepth allows, the named type
// its generic type is declared over is cut like any too-deep chain (a
// non-struct, left to the property's Resolution), rather than read afresh
// as the struct it is.
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

	assert.Equal(t, embedGeneric, ec.generic(file, path, expr, 0, false).class, "a struct base is warned")
	assert.Equal(t, embedNonStruct, ec.generic(file, path, expr, maxNamedResolutionDepth-1, false).class)
}
