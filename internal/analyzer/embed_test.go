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

// embedB is package b of the embed tests.
const embedB = `package b

import "github.com/google/uuid"

type Cents int64
type LU uuid.UUID
type Tags []string
type Base struct {
	ID int ` + "`json:\"id\"`" + `
}
type Base2 = Base
type Dangling = Gone
type Amount int64
type Wrapper struct {
	Amount
	W int ` + "`json:\"w\"`" + `
}
type Status string
type Meta struct {
	Status
	Code int ` + "`json:\"code\"`" + `
}
`

// embedImports are the import lines of the embed rows module.
const embedImports = "\t\"database/sql\"\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"time\"\n" +
	"\t\"github.com/google/uuid\"\n\t\"github.com/shopspring/decimal\"\n\t\"github.com/example/app/b\"\n"

// embedDecls are the named types the embed rows embed.
const embedDecls = `
type Cents int64
type Tags []string
type Attrs map[string]string
type Namer interface{ Name() string }
type CA = Cents
type CentsD b.Cents
type cents int
type Level int

func (l Level) MarshalText() ([]byte, error) { return nil, nil }
func (l *Level) UnmarshalText(p []byte) error { return nil }

type Code string

func (c Code) MarshalText() ([]byte, error) { return nil, nil }
func (c *Code) UnmarshalText(p []byte) error { return nil }

type Base struct {
	ID int ` + "`json:\"id\"`" + `
}
type BA = Base
type BD Base
type QA = b.Base
type QD b.Base
type QA2 = b.Base2
type NS = sql.NullString
type NS2 sql.NullString
type ns = sql.NullString
type nsd sql.NullString
type LT time.Time
type LU uuid.UUID
type LR json.RawMessage
type Address struct {
	City string ` + "`json:\"city\"`" + `
}
type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}
type AddrPage Page[Address]
type page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}
type list[T any] []T
type pair[K comparable, V any] map[K]V
type cnt[T any] int64
type tagsOf[T any] Tags
type qc[T any] b.Cents
type parenOf[T any] (int)
type ltOf[T any] time.Time
type baseOf[T any] Base
type nsOf[T any] sql.NullString
type parenBase[T any] (Base)
type pageA[T any] page[T]
type R1 struct {
	Cents
}
type Inner struct {
	Cents string ` + "`json:\"Cents\"`" + `
}
type DeepInner struct {
	Inner
}
type HasCentsField struct {
	Cents string
}
type DeepR1 struct {
	R1
}
type RB struct {
	b.Cents
}
type Shared struct {
	sql.NullInt64
	S int ` + "`json:\"s\"`" + `
}
type Status struct {
	b.Meta
	Detail string ` + "`json:\"detail\"`" + `
}
type Inner2 struct {
	V int ` + "`validate:\"required\"`" + `
	W int ` + "`json:\"w\"`" + `
}
type I2A = Inner2
type I2D Inner2
type HasV struct {
	V string
}
type TagV struct {
	X int ` + "`json:\"V\"`" + `
}
type TagVS struct {
	X string ` + "`json:\"V\"`" + `
}
type DeepHasV struct {
	HasV
}
type DeepTagV struct {
	TagV
}
type DeepTagVA = DeepTagV
type DeepTagVD DeepTagV
type PQ struct {
	ID string ` + "`query:\"id\"`" + `
}
type PQ2 struct {
	Key int ` + "`query:\"id\"`" + `
}
type PJQ struct {
	ID string ` + "`query:\"qid\" json:\"ID\"`" + `
}
type PBody struct {
	Name string
}
type PBodyID struct {
	ID int
}
type ESelfA = ESelf
type ESelfD ESelfDef
type PB (Base)
type PBA = (Base)
type PS (struct {
	Z int ` + "`json:\"z\"`" + `
})
`

// embedRow is one struct embedding embed beside its own field Y, and the
// rendered resolutions of its fields (y included) after extraction.
type embedRow struct {
	typ, embed string
	want       map[string]string
}

// analyzeEmbedRows analyzes a project whose Rows type holds one field per row
// struct, so every row registers as a component.
func analyzeEmbedRows(t *testing.T, rows []embedRow) *ProjectAnalyzer {
	t.Helper()
	var decls, fields strings.Builder
	decls.WriteString(embedDecls)
	for _, r := range rows {
		fmt.Fprintf(&decls, "type %s struct {\n\t%s\n\tY int `json:\"y\"`\n}\n", r.typ, r.embed)
		fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", r.typ, r.typ, strings.ToLower(r.typ))
	}
	return analyzeDirectiveProject(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("b", "b.go"):        embedB,
		filepath.Join("mod", "module.go"): rowsModule(embedImports, decls.String(), fields.String()),
	})
}

// embedWarnings returns the warnings raised for embedded fields.
func embedWarnings(a *ProjectAnalyzer) []string {
	var out []string
	for _, w := range a.warnings {
		if strings.HasPrefix(w, "embedded field ") {
			out = append(out, w)
		}
	}
	return out
}

// TestEmbeddedNonStructBecomesProperty pins issue 116's property rows: an
// exported non-struct embed is one property keyed by its unqualified type
// name, as encoding/json writes it, typed as a field of that type would be;
// alias and defined types over a struct promote its fields; a well-known
// Marshaler embed and an unexported one, a predeclared type (error, int) or
// a generic type declared as a non-struct type literal (list[int]) or
// named non-struct type (cnt[int], qc[int]) included, stay
// dropped, and so does a defined type over time.Time, a struct with no
// exported field. None of them warns.
func TestEmbeddedNonStructBecomesProperty(t *testing.T) {
	rows := []embedRow{
		{"ECents", "Cents", map[string]string{"Cents": goTypeInt64}},
		{"EPtrCents", "*Cents", map[string]string{"Cents": "*int64"}},
		{"EQCents", "b.Cents", map[string]string{"Cents": goTypeInt64}},
		{"EQPtrCents", "*b.Cents", map[string]string{"Cents": "*int64"}},
		{"ETags", "Tags", map[string]string{"Tags": "[]string"}},
		{"EQTags", "b.Tags", map[string]string{"Tags": "[]string"}},
		{"EAttrs", "Attrs", map[string]string{"Attrs": "map[string]string"}},
		{"ENamer", "Namer", map[string]string{"Namer": goTypeInterface}},
		{"ECA", "CA", map[string]string{"CA": goTypeInt64}},
		{"ECentsD", "CentsD", map[string]string{"CentsD": goTypeInt64}},
		{"EDuration", "time.Duration", map[string]string{"Duration": goTypeInt64}},
		{"EOmit", "Cents `json:\",omitempty\"`", map[string]string{"Cents": goTypeInt64}},
		{"EPair", "Level\n\tCode", map[string]string{"Level": "text:Level", "Code": "text:Code"}},
		{"ER1", "R1", map[string]string{"Cents": goTypeInt64}},
		{"EBA", "BA", map[string]string{"id": goTypeInt}},
		{"EBD", "*BD", map[string]string{"id": goTypeInt}},
		{"EQA", "QA", map[string]string{"id": goTypeInt}},
		{"EQD", "QD", map[string]string{"id": goTypeInt}},
		{"EQA2", "QA2", map[string]string{"id": goTypeInt}},
		{"EQBase", "b.Base", map[string]string{"id": goTypeInt}},
		{"EQBase2", "b.Base2", map[string]string{"id": goTypeInt}},
		{"EUnexported", "cents", map[string]string{}},
		{"EError", "error", map[string]string{}},
		{"EInt", "int", map[string]string{}},
		{"EString", "string", map[string]string{}},
		{"EAny", "any", map[string]string{}},
		{"EListLower", "list[int]", map[string]string{}},
		{"EPairLower", "pair[string, int]", map[string]string{}},
		{"ECntLower", "cnt[int]", map[string]string{}},
		{"ETagsOfLower", "tagsOf[int]", map[string]string{}},
		{"EQcLower", "qc[int]", map[string]string{}},
		{"EParenOfLower", "parenOf[int]", map[string]string{}},
		{"ELtOfLower", "ltOf[int]", map[string]string{}},
		{"ELT", "LT", map[string]string{}},
	}
	a := analyzeEmbedRows(t, rows)
	for _, r := range rows {
		want := map[string]string{"y": goTypeInt}
		for k, v := range r.want {
			want[k] = v
		}
		assert.Equal(t, want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestEmbeddedJSONNumberIsMarshaler pins an embedded json.Number as a
// well-known Marshaler embed: the jsonv2-backed encoding/json, Go 1.27's
// default, writes the embedding struct through the promoted MarshalJSONTo,
// the classic one as an object, so the struct is {} with the promoted
// Marshaler warning and registers no component, alone or through an alias,
// and whatever the embed's tag: Go promotes the methods of a tagged embed,
// one named by json or excluded with json:"-" included, too.
// A defined type over json.Number drops the methods and is one string
// property, and a field of type json.Number, or of an alias of it, keeps
// its number schema.
func TestEmbeddedJSONNumberIsMarshaler(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): rowsModule("\t\"encoding/json\"\n",
			"type JA = json.Number\ntype JN json.Number\n"+
				"type ENumber struct {\n\tjson.Number\n\tY int `json:\"y\"`\n}\n"+
				"type EJA struct {\n\tJA\n\tY int `json:\"y\"`\n}\n"+
				"type EJN struct {\n\tJN\n\tY int `json:\"y\"`\n}\n"+
				"type ETagged struct {\n\tjson.Number `json:\"num\"`\n\tY int `json:\"y\"`\n}\n"+
				"type ESkip struct {\n\tjson.Number `json:\"-\"`\n\tY int `json:\"y\"`\n}\n",
			"\tENumber ENumber `json:\"eNumber\"`\n\tEJA EJA `json:\"eja\"`\n\tEJN EJN `json:\"ejn\"`\n\tN json.Number `json:\"n\"`\n\tJ JA `json:\"j\"`\n"+
				"\tETagged ETagged `json:\"eTagged\"`\n\tESkip ESkip `json:\"eSkip\"`\n"),
	})
	assert.Nil(t, a.typeRegistry["ENumber"])
	assert.Nil(t, a.typeRegistry["EJA"])
	assert.Nil(t, a.typeRegistry["ETagged"], "a json-named embed promotes the methods too")
	assert.Nil(t, a.typeRegistry["ESkip"], "so does an embed json:\"-\" excludes")
	assert.Equal(t, map[string]string{"JN": goTypeString, "y": goTypeInt}, renders(t, a.typeRegistry["EJN"]))
	rows := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, models.WellKnownJSONNumber, rows["n"])
	assert.Equal(t, models.WellKnownJSONNumber, rows["j"], "an alias field keeps the number schema too")
	assert.Equal(t, "marshal:ENumber(unknown)", rows["eNumber"])
	assert.Equal(t, "marshal:EJA(unknown)", rows["eja"])
	assert.Equal(t, "marshal:ETagged(unknown)", rows["eTagged"])
	assert.Equal(t, "marshal:ESkip(unknown)", rows["eSkip"])
	warnings := a.Warnings(t.Context())
	assert.Len(t, warnings, 4, "%v", warnings)
	assert.Len(t, matching(warnings, "has the MarshalJSONTo method promoted from its embedded json.Number"), 3, "%v", warnings)
	assert.Empty(t, embedWarnings(a))
}

// TestEmbeddedPropertyKeepsWrittenName pins the property's JSON name: the
// written type name (Cents, CA), never lower-cased and never the aliased
// type's name.
func TestEmbeddedPropertyKeepsWrittenName(t *testing.T) {
	a := analyzeEmbedRows(t, []embedRow{{"ECA", "CA", nil}, {"EQPtrCents", "*b.Cents", nil}})
	ca := fieldByJSON(t, a.typeRegistry["ECA"], "CA")
	assert.Equal(t, "CA", ca.Name)
	cents := fieldByJSON(t, a.typeRegistry["EQPtrCents"], "Cents")
	assert.Equal(t, "*b.Cents", renderShape(cents.Shape), "the property keeps the written pointer shape")
}

// TestEmbeddedDefinedOverWellKnown pins a defined type over a well-known
// non-struct Marshaler type (type LU uuid.UUID, type LR json.RawMessage),
// locally or in-module: it drops the target's methods, so the promoted-method
// path does not own it, and encoding/json writes it as one field of the
// underlying type. The property is typed as a field of the defined type is,
// with that field's warning and no dropped-embed one.
func TestEmbeddedDefinedOverWellKnown(t *testing.T) {
	a := analyzeEmbedRows(t, []embedRow{
		{"ELU", "LU", nil},
		{"ELR", "*LR", nil},
		{"EQLU", "b.LU", nil},
		{"FRef", "U LU `json:\"u\"`\n\tR *LR `json:\"r\"`\n\tQ b.LU `json:\"q\"`", nil},
	})
	ref := renders(t, a.typeRegistry["FRef"])
	assert.Equal(t, map[string]string{"LU": ref["u"], "y": goTypeInt}, renders(t, a.typeRegistry["ELU"]))
	assert.Equal(t, map[string]string{"LR": ref["r"], "y": goTypeInt}, renders(t, a.typeRegistry["ELR"]))
	assert.Equal(t, map[string]string{"LU": ref["q"], "y": goTypeInt}, renders(t, a.typeRegistry["EQLU"]))
	assert.Empty(t, matching(a.warnings, "the embedded field is dropped"))
	assert.Len(t, matching(a.warnings, "is a defined type over uuid.UUID"), 4, "the two properties warn as the two fields do: %v", a.warnings)
}

// TestEmbeddedUnexpandableWarns pins issue 116's warned rows: an embed this
// tool cannot classify as struct or non-struct (a type from outside the
// module, a local chain ending at one, an undeclared name) or a generic
// instantiation is dropped with one warning, which feeds --strict, whatever
// the case of its name: encoding/json promotes an unexported struct embed.
func TestEmbeddedUnexpandableWarns(t *testing.T) {
	rows := []embedRow{
		{"ENullString", "sql.NullString", nil},
		{"EPtrNullString", "*sql.NullString", nil},
		{"EStringer", "fmt.Stringer", nil},
		{"ENS", "NS", nil},
		{"ENS2", "NS2", nil},
		{"ENsLower", "ns", nil},
		{"ENsdLower", "nsd", nil},
		{"EDecimal", "decimal.Decimal", nil},
		{"EMissing", "Missing", nil},
		{"EQMissing", "b.Missing", nil},
		{"EPage", "Page[Address]", nil},
		{"EPtrPage", "*Page[Address]", nil},
		{"EAddrPage", "AddrPage", nil},
		{"EPageLower", "page[Address]", nil},
		{"EPageALower", "pageA[Address]", nil},
		{"EGoneLower", "gone[int]", nil},
		{"EBaseOfLower", "baseOf[int]", nil},
		{"ENsOfLower", "nsOf[int]", nil},
		{"EParenBaseLower", "parenBase[int]", nil},
		{"EDangling", "b.Dangling", nil},
		{"EShared1", "Shared", map[string]string{"s": goTypeInt}},
		{"EShared2", "Shared", map[string]string{"s": goTypeInt}},
	}
	wants := []string{
		"embedded field Stringer at " + filepath.Join("mod", "module.go") + ":",
		"has type sql.NullString: sql.NullString cannot be read (a type from outside the module,",
		"has type *sql.NullString: sql.NullString cannot be read",
		"has type fmt.Stringer: fmt.Stringer cannot be read",
		"has type NS: sql.NullString cannot be read",
		"has type NS2: sql.NullString cannot be read",
		"has type ns: sql.NullString cannot be read", // unexported, yet encoding/json promotes a struct's fields
		"has type nsd: sql.NullString cannot be read",
		"has type decimal.Decimal: decimal.Decimal cannot be read",
		"has type Missing: Missing cannot be read",
		"has type b.Missing: b.Missing cannot be read",
		"has type Page[Address]: Page[Address] is a generic instantiation, which is not expanded",
		"has type *Page[Address]: Page[Address] is a generic instantiation",
		"has type AddrPage: Page[Address] is a generic instantiation",
		"has type page[Address]: page[Address] is a generic instantiation",
		"has type pageA[Address]: pageA[Address] is a generic instantiation", // a defined type over a struct: not read
		"has type gone[int]: gone[int] is a generic instantiation",
		"has type baseOf[int]: baseOf[int] is a generic instantiation", // a named struct: encoding/json promotes its fields
		"has type nsOf[int]: nsOf[int] is a generic instantiation",
		"has type parenBase[int]: parenBase[int] is a generic instantiation",
		"has type b.Dangling: b.Gone cannot be read", // named in its own package
		"has type sql.NullInt64:",                    // Shared's embed, reached twice, warns once
	}
	a := analyzeEmbedRows(t, rows)
	for _, r := range rows {
		want := map[string]string{"y": goTypeInt}
		for k, v := range r.want {
			want[k] = v
		}
		assert.Equal(t, want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	warnings := embedWarnings(a)
	assert.Len(t, warnings, len(rows)-1, "one warning per dropped embed (Shared's once): %v", warnings)
	assert.Len(t, a.Warnings(t.Context()), len(rows)-1, "no other warning")
	for _, want := range wants {
		assert.Len(t, matching(warnings, want), 1, "%q in %v", want, warnings)
	}
}

// TestEmbeddedPropertyCollisions pins encoding/json's dominance rule for a
// JSON name an untagged embed's property carries, in either declaration
// order: the shallower field wins, and at one depth, the struct's own or a
// promoted one, a tagged field beats the property. An untagged named field
// collides under its Go name, the key encoding/json writes, though the
// generator lower-cases it.
func TestEmbeddedPropertyCollisions(t *testing.T) {
	rows := []embedRow{
		{"EEmbedFirst", "Cents\n\tX string `json:\"Cents\"`", map[string]string{"Cents": goTypeString}},
		{"ETagFirst", "X string `json:\"Cents\"`\n\tCents", map[string]string{"Cents": goTypeString}},
		{"EShadow", "Inner\n\tCents", map[string]string{"Cents": goTypeInt64}},
		{"EColl1", "R1\n\tInner", map[string]string{"Cents": goTypeString}},
		{"EColl2", "Inner\n\tR1", map[string]string{"Cents": goTypeString}},
		{"EPropShallower1", "R1\n\tDeepInner", map[string]string{"Cents": goTypeInt64}},
		{"EPropShallower2", "DeepInner\n\tR1", map[string]string{"Cents": goTypeInt64}},
		{"ETagShallower1", "DeepR1\n\tInner", map[string]string{"Cents": goTypeString}},
		{"ETagShallower2", "Inner\n\tDeepR1", map[string]string{"Cents": goTypeString}},
		{"EPropVsNamed", "Cents\n\tHasCentsField", map[string]string{"Cents": goTypeInt64}},
		{"ENamedVsProp", "Cents string\n\tR1", map[string]string{"Cents": goTypeString}},
	}
	a := analyzeEmbedRows(t, rows)
	for _, r := range rows {
		ti := a.typeRegistry[r.typ]
		require.NotNil(t, ti, r.typ)
		count := 0
		for i := range ti.Fields {
			f := &ti.Fields[i]
			if f.JSONName == "Cents" || f.JSONName == "" && f.Name == "Cents" {
				count++
			}
		}
		assert.Equal(t, 1, count, "%s keeps one Cents", r.typ)
		assert.Equal(t, r.want["Cents"], renders(t, ti)["Cents"], r.typ)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestEmbeddedPropertyTieKeepsFirst pins the residual: two properties of one
// JSON name at one promoted depth, or a property and an untagged named field
// of that Go name, which encoding/json both drops, keep the first, as two
// tagged fields at one depth do.
func TestEmbeddedPropertyTieKeepsFirst(t *testing.T) {
	a := analyzeEmbedRows(t, []embedRow{
		{"ETie1", "R1\n\tRB", nil}, {"ETie2", "RB\n\tR1", nil},
		{"ETie3", "R1\n\tHasCentsField", nil}, {"ETie4", "HasCentsField\n\tR1", nil},
	})
	assert.Equal(t, "Cents", renderShape(fieldByJSON(t, a.typeRegistry["ETie1"], "Cents").Shape))
	assert.Equal(t, "b.Cents", renderShape(fieldByJSON(t, a.typeRegistry["ETie2"], "Cents").Shape))
	assert.Equal(t, goTypeInt64, renders(t, a.typeRegistry["ETie3"])["Cents"])
	assert.Equal(t, goTypeString, renders(t, a.typeRegistry["ETie4"])["Cents"])
	for _, typ := range []string{"ETie1", "ETie2", "ETie3", "ETie4"} {
		assert.Len(t, a.typeRegistry[typ].Fields, 2, typ)
	}
}

// TestEmbeddedPropertyNamedLikeAncestor pins that the cycle guard holds
// back only a struct embed: a non-struct embed
// never recurses, so one named like the struct being registered (Amount
// holding b.Wrapper, which embeds b.Amount) or like a struct on the
// promotion chain (EAnc through Status to b.Meta, which embeds b.Status)
// is still its property, as encoding/json writes it.
func TestEmbeddedPropertyNamedLikeAncestor(t *testing.T) {
	a := analyzeEmbedRows(t, []embedRow{{"Amount", "b.Wrapper", nil}, {"EAnc", "Status", nil}})
	assert.Equal(t, map[string]string{"Amount": goTypeInt64, "w": goTypeInt, "y": goTypeInt}, renders(t, a.typeRegistry["Amount"]))
	assert.Equal(t, map[string]string{"Status": goTypeString, "code": goTypeInt, "detail": goTypeString, "y": goTypeInt},
		renders(t, a.typeRegistry["EAnc"]))
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestUntaggedFieldShadowsPromoted pins that an untagged named field
// collides under its Go name, the key encoding/json writes, with a field
// promoted from deeper, through a direct, alias or defined-type struct embed
// and in either declaration order: the shallower field wins, so its type and
// required flag are the ones kept, and the generator, which lower-cases both,
// never sees two. A deeper tagged json:"V" loses to it the same way, and two
// untagged V at one promoted depth, which encoding/json both drops, keep the
// first.
func TestUntaggedFieldShadowsPromoted(t *testing.T) {
	rows := []embedRow{
		{"EVDirect", "V string\n\tInner2", map[string]string{"V": goTypeString}},
		{"EVAlias", "V string\n\tI2A", map[string]string{"V": goTypeString}},
		{"EVDefined", "V string\n\tI2D", map[string]string{"V": goTypeString}},
		{"EVAliasFirst", "*I2A\n\tV string", map[string]string{"V": goTypeString}},
		{"EVRequired", "V string `validate:\"required\"`\n\tI2A", map[string]string{"V": goTypeString}},
		{"EVTagged", "V string\n\tTagV", map[string]string{"V": goTypeString}},
		{"EVTie1", "Inner2\n\tHasV", map[string]string{"V": goTypeInt}},
		{"EVTie2", "HasV\n\tInner2", map[string]string{"V": goTypeString}},
	}
	a := analyzeEmbedRows(t, rows)
	for _, r := range rows {
		ti := a.typeRegistry[r.typ]
		require.NotNil(t, ti, r.typ)
		var kept []*models.FieldInfo
		for i := range ti.Fields {
			if f := &ti.Fields[i]; f.JSONName == "V" || f.JSONName == "" && f.Name == "V" {
				kept = append(kept, f)
			}
		}
		require.Len(t, kept, 1, "%s keeps one V", r.typ)
		assert.Equal(t, r.want["V"], renders(t, ti)["V"], r.typ)
		assert.Equal(t, r.typ == "EVRequired" || r.typ == "EVTie1", kept[0].Required, "%s keeps the winner's required flag", r.typ)
	}
	assert.Contains(t, renders(t, a.typeRegistry["EVAlias"]), "w", "the alias embed's other fields are still promoted")
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestKeyDominance pins encoding/json's dominance rule for every body key,
// whatever the declaration order: the shallower field wins at any promoted
// depth, so a deeper field declared first loses (DeepHasV beside Inner2, a
// deeper tagged json:"V" beside HasV, or beside a shallower tagged one when
// only tagged fields carry the key, reached directly or through an alias or
// a defined type), and at one depth, a struct's own fields or promoted ones,
// a tagged field beats an untagged one. Two tagged fields at the winning
// depth, which encoding/json both drops, keep the first.
func TestKeyDominance(t *testing.T) {
	rows := []embedRow{
		{"EVPromTagged1", "HasV\n\tTagV", map[string]string{"V": goTypeInt}},
		{"EVPromTagged2", "TagV\n\tHasV", map[string]string{"V": goTypeInt}},
		{"EVDeeperFirst", "DeepHasV\n\tInner2", map[string]string{"V": goTypeInt}},
		{"EVShallowerFirst", "Inner2\n\tDeepHasV", map[string]string{"V": goTypeInt}},
		{"EVTaggedDeeperFirst", "DeepTagV\n\tHasV", map[string]string{"V": goTypeString}},
		{"EVTaggedDeeperLast", "HasV\n\tDeepTagV", map[string]string{"V": goTypeString}},
		{"EVOwnTagged1", "V string\n\tX int `json:\"V\"`", map[string]string{"V": goTypeInt}},
		{"EVOwnTagged2", "X int `json:\"V\"`\n\tV string", map[string]string{"V": goTypeInt}},
		{"EVTaggedTie1", "HasV\n\tTagV\n\tTagVS", map[string]string{"V": goTypeInt}},
		{"EVTaggedTie2", "HasV\n\tTagVS\n\tTagV", map[string]string{"V": goTypeString}},
		{"EVTagOnlyDeeperFirst", "DeepTagV\n\tTagVS", map[string]string{"V": goTypeString}},
		{"EVTagOnlyAliasDeeperFirst", "DeepTagVA\n\tTagVS", map[string]string{"V": goTypeString}},
		{"EVTagOnlyDefinedDeeperFirst", "*DeepTagVD\n\tTagVS", map[string]string{"V": goTypeString}},
	}
	a := analyzeEmbedRows(t, rows)
	for _, r := range rows {
		ti := a.typeRegistry[r.typ]
		require.NotNil(t, ti, r.typ)
		count := 0
		for i := range ti.Fields {
			if f := &ti.Fields[i]; f.JSONName == "V" || f.JSONName == "" && f.Name == "V" {
				count++
			}
		}
		assert.Equal(t, 1, count, "%s keeps one V", r.typ)
		assert.Equal(t, r.want["V"], renders(t, ti)["V"], r.typ)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestParameterKeys pins that a path, query or header parameter keeps the
// body key encoding/json gives it (its JSON name, or its Go name), since
// go-bricks decodes the body into the whole request struct: it hides a body
// field of that key it outranks (a promoted untagged or tagged field, or an
// embed's property at its depth), but no body field or parameter of its key
// ever drops it. Parameters are deduplicated by location and name instead:
// the struct's own beats a promoted one, json-tagged or not, and of two
// promoted ones the first is kept.
func TestParameterKeys(t *testing.T) {
	rows := []struct {
		typ, embed string
		want       []string
	}{
		{"EPNamedVsPromoted", "ID int `param:\"id\"`\n\tPQ", []string{"ID path:id", "ID query:id"}},
		{"EPNamedVsBody", "Name string `query:\"Name\"`\n\tPBody", []string{"Name query:Name"}},
		{"EPBodyVsJSONParam", "ID int\n\tPJQ", []string{"ID body:int:", "ID query:qid"}},
		{"EPJSONParamVsBody", "ID string `param:\"id\" json:\"ID\"`\n\tPBodyID", []string{"ID path:id"}},
		{"EPTaggedVsJSONParam", "X int `json:\"ID\"`\n\tPJQ", []string{"X body:int:ID", "ID query:qid"}},
		{"EPJSONParams", "ID string `param:\"id\" json:\"ID\"`\n\tPJQ", []string{"ID path:id", "ID query:qid"}},
		{"EPJSONParamVsTagged", "V string `query:\"v\" json:\"V\"`\n\tTagV", []string{"V query:v"}},
		{"EPJSONParamVsProp", "Cents\n\tX string `query:\"x\" json:\"Cents\"`", []string{"X query:x"}},
		{"EPSkipParamVsBody", "ID string `param:\"id\" json:\"-\"`\n\tPBodyID", []string{"ID path:id", "ID body:int:"}},
		{"EPDupOwnFirst", "Key int `query:\"id\"`\n\tPQ", []string{"Key query:id"}},
		{"EPDupOwnLast", "PQ\n\tKey int `query:\"id\" json:\"key\"`", []string{"Key query:id"}},
		{"EPDupPromoted", "PQ\n\tPQ2", []string{"ID query:id"}},
		{"EPDupPromotedRev", "PQ2\n\tPQ", []string{"Key query:id"}},
	}
	embedRows := make([]embedRow, len(rows))
	for i, r := range rows {
		embedRows[i] = embedRow{typ: r.typ, embed: r.embed}
	}
	a := analyzeEmbedRows(t, embedRows)
	for _, r := range rows {
		ti := a.typeRegistry[r.typ]
		require.NotNil(t, ti, r.typ)
		got := make([]string, 0, len(ti.Fields))
		for i := range ti.Fields {
			if f := &ti.Fields[i]; f.Name != "Y" {
				got = append(got, paramOrBody(f))
			}
		}
		assert.ElementsMatch(t, r.want, got, r.typ)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// paramOrBody renders f as "Name in:param" for a parameter and as
// "Name body:type:jsonName" for a body field.
func paramOrBody(f *models.FieldInfo) string {
	if f.ParamType != "" {
		return f.Name + " " + f.ParamType + ":" + f.ParamName
	}
	return f.Name + " body:" + renderShape(f.ResolvedShape()) + ":" + f.JSONName
}

// TestEmbeddedSelfThroughAliasOrDefined pins that the embed cycle guard
// knows a struct by its declaration, not by the name an embed writes: a
// struct embedding itself through an alias or a defined type is not promoted
// into itself, so a parameter, which no JSON key dedupes, is not doubled.
func TestEmbeddedSelfThroughAliasOrDefined(t *testing.T) {
	a := analyzeEmbedRows(t, []embedRow{
		{"ESelf", "*ESelfA\n\tID int `param:\"id\"`", nil},
		{"ESelfDef", "*ESelfD\n\tID int `param:\"id\"`", nil},
	})
	for _, typ := range []string{"ESelf", "ESelfDef"} {
		ti := a.typeRegistry[typ]
		require.NotNil(t, ti, typ)
		names := make([]string, 0, len(ti.Fields))
		for i := range ti.Fields {
			names = append(names, ti.Fields[i].Name)
		}
		assert.ElementsMatch(t, []string{"ID", "Y"}, names, typ)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestEmbeddedParenthesizedStructPromotes pins that a struct reached through
// a parenthesized declaration (type PB (Base), type PBA = (Base), or a
// parenthesized struct literal) is promoted, as encoding/json promotes it,
// not written as a property named like the type.
func TestEmbeddedParenthesizedStructPromotes(t *testing.T) {
	rows := []embedRow{
		{"EParenDefined", "PB", map[string]string{"id": goTypeInt, "y": goTypeInt}},
		{"EParenAlias", "PBA", map[string]string{"id": goTypeInt, "y": goTypeInt}},
		{"EParenLiteral", "*PS", map[string]string{"z": goTypeInt, "y": goTypeInt}},
	}
	a := analyzeEmbedRows(t, rows)
	for _, r := range rows {
		assert.Equal(t, r.want, renders(t, a.typeRegistry[r.typ]), r.typ)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestEmbeddedCycleTerminates pins that an invalid defined-type cycle, which
// only the type checker rejects, terminates: the embed is a property whose
// resolution is cut.
func TestEmbeddedCycleTerminates(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): rowsModule("", "type CycA CycB\ntype CycB CycA\ntype ECyc struct {\n\tCycA\n}\n",
			"\tECyc ECyc `json:\"eCyc\"`\n"),
	})
	assert.Equal(t, map[string]string{"CycA": "cycle:CycA"}, renders(t, a.typeRegistry["ECyc"]))
	assert.Len(t, fieldWarnings(a, "CycA"), 1)
}

// embedRequestHead is the module head of the embed request tests.
const embedRequestHead = "package mod\n\nimport (\n\t\"database/sql\"\n\t\"encoding/json\"\n\t\"time\"\n\n" +
	"\t\"github.com/gaborage/go-bricks/app\"\n\t\"github.com/gaborage/go-bricks/server\"\n\t\"github.com/google/uuid\"\n)\n\n" +
	"type Module struct{}\n\n" +
	"func (m *Module) Name() string                    { return \"mod\" }\n" +
	"func (m *Module) Init(deps *app.ModuleDeps) error { return nil }\n" +
	"func (m *Module) Shutdown() error                 { return nil }\n"

// TestEmbeddedNonStructRequestParams pins request rows: a non-struct embed
// with a query tag is that parameter (kept beside a tagged body field of its
// type name, at the struct's own depth or a promoted one), and so is an
// exported well-known Marshaler embed with one (uuid.UUID, *time.Time),
// typed by kind as any parameter is. A request read only for its parameters
// (a struct Marshaler request through an embedded time.Time, uuid.UUID,
// json.RawMessage, or an alias of uuid.UUID, which keeps its methods) gets
// no property for an untagged well-known Marshaler embed: the
// promoted-method path owns it; an unexported one stays dropped, tagged or
// not, as go-bricks cannot set it.
func TestEmbeddedNonStructRequestParams(t *testing.T) {
	_, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): embedRequestHead + `
type Cents int64

type QReq struct {
	Cents ` + "`query:\"c\" validate:\"min=1\"`" + `
	ID     string ` + "`param:\"id\"`" + `
	Amount string ` + "`json:\"Cents\"`" + `
}

type MReq struct {
	time.Time
	ID string ` + "`param:\"id\"`" + `
}

type UReq struct {
	*uuid.UUID
	ID string ` + "`param:\"id\"`" + `
}

type RReq struct {
	json.RawMessage
	ID string ` + "`param:\"id\"`" + `
}

type UA = uuid.UUID

type UQReq struct {
	uuid.UUID ` + "`query:\"u\"`" + `
	ID string ` + "`param:\"id\"`" + `
}

type TQReq struct {
	*time.Time ` + "`query:\"since\"`" + `
	ID string ` + "`param:\"id\"`" + `
}

type ua = uuid.UUID

type LQReq struct {
	ua ` + "`query:\"u\"`" + `
	ID string ` + "`param:\"id\"`" + `
}

type AReq struct {
	UA
	ID string ` + "`param:\"id\"`" + `
}

type QP struct {
	Cents ` + "`query:\"c\"`" + `
}

type BP struct {
	Amount string ` + "`json:\"Cents\"`" + `
}

type PReq struct {
	QP
	BP
	ID string ` + "`param:\"id\"`" + `
}

type Out struct {
	X int ` + "`json:\"x\"`" + `
}

func (m *Module) p(req PReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) q(req QReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) t(req MReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) u(req UReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) r(req RReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) a(req AReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) uq(req UQReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) tq(req TQReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) lq(req LQReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/uq/:id", m.uq)
	server.GET(hr, r, "/tq/:id", m.tq)
	server.GET(hr, r, "/lq/:id", m.lq)
	server.GET(hr, r, "/p/:id", m.p)
	server.GET(hr, r, "/a/:id", m.a)
	server.GET(hr, r, "/q/:id", m.q)
	server.GET(hr, r, "/t/:id", m.t)
	server.GET(hr, r, "/u/:id", m.u)
	server.GET(hr, r, "/r/:id", m.r)
}
`,
	})
	p := routeByPath(t, routes, "/p/{id}")
	require.NotNil(t, p.Request)
	assert.Len(t, p.Request.Fields, 3, "a promoted parameter embed, outranked by a tagged body field of its key at its depth, is kept beside it")
	q := routeByPath(t, routes, "/q/{id}")
	require.NotNil(t, q.Request)
	require.Len(t, q.Request.Fields, 3, "a parameter is never dropped for a body field of its JSON name")
	c := q.Request.Fields[0]
	assert.Equal(t, "query", c.ParamType)
	assert.Equal(t, "c", c.ParamName)
	assert.Equal(t, goTypeInt64, renderShape(c.ResolvedShape()))
	assert.Equal(t, "1", c.Constraints["min"])

	for path, want := range map[string]string{"/uq/{id}": "u:uuid.UUID", "/tq/{id}": "since:*time.Time"} {
		r := routeByPath(t, routes, path)
		require.NotNil(t, r.Request, path)
		require.Len(t, r.Request.Fields, 2, "a tagged well-known Marshaler embed is its parameter: %s", path)
		f := r.Request.Fields[0]
		assert.Equal(t, "query", f.ParamType, path)
		assert.Equal(t, want, f.ParamName+":"+renderShape(f.ResolvedShape()), path)
	}

	for _, path := range []string{"/t/{id}", "/u/{id}", "/r/{id}", "/a/{id}", "/lq/{id}"} {
		r := routeByPath(t, routes, path)
		require.NotNil(t, r.Request, path)
		require.Len(t, r.Request.Fields, 1, path)
		assert.Equal(t, "id", r.Request.Fields[0].ParamName, path)
	}
}

// TestEmbeddedWellKnownMarshalerNoWarning pins the well-known Marshaler
// embeds of TestEmbeddedNonStructRequestParams warning-free, in a project of
// their own so the assertion is not about other rows.
func TestEmbeddedWellKnownMarshalerNoWarning(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): embedRequestHead + `
type MReq struct {
	time.Time
	ID string ` + "`param:\"id\"`" + `
}

type UReq struct {
	*uuid.UUID
	ID string ` + "`param:\"id\"`" + `
}

type RReq struct {
	json.RawMessage
	ID string ` + "`param:\"id\"`" + `
}

type Out struct {
	X int ` + "`json:\"x\"`" + `
}

func (m *Module) t(req MReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) u(req UReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) r(req RReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/t/:id", m.t)
	server.GET(hr, r, "/u/:id", m.u)
	server.GET(hr, r, "/r/:id", m.r)
}
`,
	})
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestEmbeddedMarshalerRequestQuiet pins that a struct Marshaler request,
// whose fields are read only to find its parameters, raises no
// dropped-embed warning: encoding/json never reads those fields. The same
// embed reached through a struct registered as a component still warns,
// once, whichever is read first.
func TestEmbeddedMarshalerRequestQuiet(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): embedRequestHead + `
type Inner struct {
	sql.NullString
	N int ` + "`json:\"n\"`" + `
}

type MReq struct {
	time.Time
	sql.NullString
	Inner
	ID string ` + "`param:\"id\"`" + `
}

type Out struct {
	X int ` + "`json:\"x\"`" + `
}

type InnerOut struct {
	I Inner ` + "`json:\"i\"`" + `
}

func (m *Module) t(req MReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) i(ctx server.HandlerContext) (server.Result[InnerOut], server.IAPIError) { return server.Result[InnerOut]{}, nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/t/:id", m.t)
	server.GET(hr, r, "/i", m.i)
}
`,
	})
	warnings := embedWarnings(a)
	require.Len(t, warnings, 1, "%v", warnings)
	assert.Contains(t, warnings[0], "embedded field NullString at "+filepath.Join("mod", "module.go")+":20:", "Inner's, not MReq's")
	assert.Len(t, a.Warnings(t.Context()), 2, "and MReq's promoted-Marshaler request warning")
}

// matching returns the entries of ws that contain sub.
func matching(ws []string, sub string) []string {
	var out []string
	for _, w := range ws {
		if strings.Contains(w, sub) {
			out = append(out, w)
		}
	}
	return out
}

// TestEmbeddedAllConstrainedPackageWarns pins the README's all-constrained
// case: when a //go:build tools file of package tools sorts first in b and
// every file is constrained, an unaliased import of b is named tools, so an
// embedded b.Base, which used to lose its promoted fields silently, is
// dropped with a warning naming that cause; so is an aliased bb.Cents, a
// named non-struct type the tools clause does not declare.
func TestEmbeddedAllConstrainedPackageWarns(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                           resolveGoMod,
		filepath.Join("b", "aaa_tools.go"): "//go:build tools\n\npackage tools\n",
		filepath.Join("b", "b.go"): "//go:build !plan9\n\npackage b\n\ntype Base struct {\n\tCreated string `json:\"created\"`\n}\n\n" +
			"type Cents int64\n",
		filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n", "",
			"\tb.Base\n\tbb.Cents\n\tX int `json:\"x\"`\n"),
	})
	assert.Equal(t, map[string]string{"x": goTypeInt}, renders(t, a.typeRegistry["Rows"]))
	warnings := embedWarnings(a)
	require.Len(t, warnings, 2, "%v", warnings)
	cause := "an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause"
	for _, want := range []string{"has type b.Base: b.Base cannot be read", "has type bb.Cents: bb.Cents cannot be read"} {
		if got := matching(warnings, want); assert.Len(t, got, 1, want) {
			assert.Contains(t, got[0], cause)
		}
	}
}

// TestEmbeddedLocalMarshalerRequestHasBody pins a consequence of the
// property rows: a struct Marshaler request whose local embed promotes a
// marshal method now has a body field, so it is no longer parameters-only.
// A promoted MarshalText alone documents an untyped body with the promoted
// Marshaler request warning; a text pair documents a string body silently.
func TestEmbeddedLocalMarshalerRequestHasBody(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): embedRequestHead + `
type StatusV int

func (s StatusV) MarshalText() ([]byte, error) { return nil, nil }

type Level int

func (l Level) MarshalText() ([]byte, error) { return nil, nil }
func (l *Level) UnmarshalText(p []byte) error { return nil }

type SReq struct {
	StatusV
	ID string ` + "`param:\"id\"`" + `
}

type LReq struct {
	Level
	ID string ` + "`param:\"id\"`" + `
}

type Out struct {
	X int ` + "`json:\"x\"`" + `
}

func (m *Module) s(req SReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) l(req LReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { return server.Result[Out]{}, nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/s/:id", m.s)
	server.POST(hr, r, "/l/:id", m.l)
}
`,
	})
	s := routeByPath(t, routes, "/s/{id}").Request
	require.NotNil(t, s.Resolution)
	assert.Equal(t, "marshal:SReq(unknown)", renderShape(*s.Resolution))
	l := routeByPath(t, routes, "/l/{id}").Request
	require.NotNil(t, l.Resolution)
	assert.Equal(t, "text:LReq", renderShape(*l.Resolution))
	warnings := a.Warnings(t.Context())
	if assert.Len(t, warnings, 1) {
		assert.Contains(t, warnings[0], "SReq has the MarshalText method promoted from its embedded StatusV")
	}
}
