package analyzer

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Type names the Marshaler-type tests (#111) repeat.
const (
	mtLevel     = "Level"
	mtStatusV   = "StatusV"
	mtMoney     = "Money"
	mtWrapLevel = "WrapLevel"
	mtMoneyT    = "MoneyT"
	mtFields    = "Fields"
	mtFilter    = "Filter"
	mtEmpty     = "Empty"
	mtListReq   = "ListReq"
	mtPrice     = "domain.Price"
)

// mtDomainSrc is the in-module domain package: a text Tag, a struct Price with
// its own MarshalJSON, and a text struct Amount.
const mtDomainSrc = `package domain

type Tag int

func (t Tag) MarshalText() ([]byte, error) { return []byte("t"), nil }
func (t *Tag) UnmarshalText(b []byte) error { return nil }

type Price struct {
	Cents int64 ` + "`json:\"cents\"`" + `
}

func (p Price) MarshalJSON() ([]byte, error) { return []byte("1"), nil }

type Amount struct {
	Value int64 ` + "`json:\"value\"`" + `
}

func (a Amount) MarshalText() ([]byte, error) { return []byte("1 USD"), nil }
func (a *Amount) UnmarshalText(b []byte) error { return nil }
`

// mtTypesSrc declares the Marshaler types of the marshaler_types golden
// (Appendix B's types.go) in package mod.
const mtTypesSrc = `package mod

import (
	"encoding/json"
	"encoding/json/jsontext"
	"time"

	"github.com/google/uuid"
)

type Level int

func (l Level) MarshalText() ([]byte, error) { return []byte("high"), nil }
func (l *Level) UnmarshalText(b []byte) error { return nil }

type LA = Level
type LD Level

type LevelP int

func (l *LevelP) MarshalText() ([]byte, error) { return []byte("p"), nil }
func (l *LevelP) UnmarshalText(b []byte) error { return nil }

type StatusV int

func (s StatusV) MarshalText() ([]byte, error) { return []byte("active"), nil }

type LevelU int

func (l *LevelU) UnmarshalText(b []byte) error { return nil }

type Both int

func (b Both) MarshalText() ([]byte, error) { return nil, nil }
func (b *Both) UnmarshalText(p []byte) error { return nil }
func (b Both) MarshalJSON() ([]byte, error) { return nil, nil }

type TextJ int

func (x TextJ) MarshalText() ([]byte, error) { return nil, nil }
func (x *TextJ) UnmarshalText(p []byte) error { return nil }
func (x *TextJ) UnmarshalJSON(p []byte) error { return nil }

type Code string

func (c Code) MarshalText() ([]byte, error) { return []byte(c), nil }
func (c *Code) UnmarshalText(b []byte) error { return nil }

type FlagT2 byte

func (f FlagT2) MarshalText() ([]byte, error) { return nil, nil }
func (f *FlagT2) UnmarshalText(b []byte) error { return nil }

type TextApp int

func (x TextApp) MarshalText() ([]byte, error) { return nil, nil }
func (x TextApp) AppendText(b []byte) ([]byte, error) { return b, nil }
func (x *TextApp) UnmarshalText(b []byte) error { return nil }

type AppOnly int

func (x AppOnly) AppendText(b []byte) ([]byte, error) { return b, nil }
func (x *AppOnly) UnmarshalText(b []byte) error { return nil }

type ToText int

func (x ToText) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }
func (x ToText) MarshalText() ([]byte, error) { return nil, nil }
func (x *ToText) UnmarshalText(b []byte) error { return nil }

type FromOnly int

func (x *FromOnly) UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }

type Money struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

type MoneyP struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m *MoneyP) MarshalJSON() ([]byte, error) { return nil, nil }

type MoneyU struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m *MoneyU) UnmarshalJSON(b []byte) error { return nil }

type MA = Money
type MD Money

type MoneyOdd struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (MoneyOdd) MarshalJSON() string { return "" }

type MoneyT struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m MoneyT) MarshalText() ([]byte, error) { return []byte("1 USD"), nil }
func (m *MoneyT) UnmarshalText(b []byte) error { return nil }

type EmbP struct {
	LevelP
	N int ` + "`json:\"n\"`" + `
}

type PtrEmbP struct {
	*LevelP
	N int ` + "`json:\"n\"`" + `
}

type EmbEmbP struct{ EmbP }

type PtrEmbEmbP struct{ *EmbP }

type AllFive int

func (AllFive) MarshalJSON() ([]byte, error) { return nil, nil }
func (*AllFive) UnmarshalJSON(b []byte) error { return nil }
func (AllFive) MarshalText() ([]byte, error) { return nil, nil }
func (*AllFive) UnmarshalText(b []byte) error { return nil }
func (AllFive) AppendText(b []byte) ([]byte, error) { return b, nil }

type TimeTie struct {
	time.Time
	AllFive
	Note string ` + "`json:\"note\"`" + `
}

type Pair struct {
	Level
	Code
}

type Wrapper struct {
	StatusV
	Name string ` + "`json:\"name\"`" + `
}

type Stamped struct {
	time.Time
	Note string ` + "`json:\"note\"`" + `
}

type Twin struct {
	Level
	StatusV
}

type Shadow struct {
	Level
	Note string ` + "`json:\"note\"`" + `
}

func (s Shadow) MarshalJSON() ([]byte, error) { return nil, nil }

type OnlyInner struct {
	In string ` + "`json:\"in\"`" + `
}

type MoneyWithInner struct {
	Inner OnlyInner ` + "`json:\"inner\"`" + `
}

func (m MoneyWithInner) MarshalJSON() ([]byte, error) { return nil, nil }

type WrapLevel struct {
	Level
	Note string ` + "`json:\"note\"`" + `
}

type PtrEmb struct {
	*Level
	Note string ` + "`json:\"note\"`" + `
}

type TaggedEmb struct {
	Level ` + "`json:\"lvl\"`" + `
	Note  string ` + "`json:\"note\"`" + `
}

type Deep struct {
	WrapLevel
	X int ` + "`json:\"x\"`" + `
}

type WithID struct {
	uuid.UUID
	Note string ` + "`json:\"note\"`" + `
}

type CustomBody struct {
	ID   string ` + "`param:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func (c *CustomBody) UnmarshalJSON(b []byte) error { return nil }

type Raw struct {
	json.RawMessage
	N int ` + "`json:\"n\"`" + `
}
`

// mtExtraSrc declares the method-set rows pinned by unit tests only.
const mtExtraSrc = `package mod

import (
	"time"

	"github.com/shopspring/decimal"
)

type MDW WrapLevel

type Excl struct {
	Level ` + "`json:\"-\"`" + `
	N     int ` + "`json:\"n\"`" + `
}

type Near struct {
	StatusV
	WrapLevel
}

type OddShadow struct {
	Level
	N int ` + "`json:\"n\"`" + `
}

func (o OddShadow) MarshalText() string { return "" }

type TA = time.Time

type StampA struct {
	TA
	N int ` + "`json:\"n\"`" + `
}

type StampD time.Time

type StampDEmb struct {
	StampD
	N int ` + "`json:\"n\"`" + `
}

type Box[T any] struct {
	V T ` + "`json:\"v\"`" + `
}

type GenEmb struct {
	Box[int]
	N int ` + "`json:\"n\"`" + `
}

type Tags []string

type TagEmb struct {
	Tags
	N int ` + "`json:\"n\"`" + `
}

type IntEmb struct {
	int
	N int ` + "`json:\"n\"`" + `
}

type Node struct {
	*Node
	Name string ` + "`json:\"name\"`" + `
}

type A struct {
	*B
	X int ` + "`json:\"x\"`" + `
}

type B struct {
	*A
	Y int ` + "`json:\"y\"`" + `
}

type Dec struct {
	decimal.Decimal
	N int ` + "`json:\"n\"`" + `
}

type E0 struct{ E1 }
type E1 struct{ E2 }
type E2 struct{ E3 }
type E3 struct{ E4 }
type E4 struct{ E5 }
type E5 struct{ E6 }
type E6 struct{ E7 }
type E7 struct{ E8 }
type E8 struct{ E9 }
type E9 struct{ Level }

type FieldShadow struct {
	Level
	MarshalText int ` + "`json:\"mt\"`" + `
}

type FieldMid struct {
	Level
	MarshalText int ` + "`json:\"mt\"`" + `
}

type FieldOuter struct {
	FieldMid
	N int ` + "`json:\"n\"`" + `
}

type FieldInner struct {
	MarshalText int ` + "`json:\"mt\"`" + `
}

type FieldTie struct {
	Level
	FieldInner
}

type MarshalText struct {
	N int ` + "`json:\"n\"`" + `
}

type EmbNamed struct {
	Level
	*MarshalText
}
`

// mtWithCodeSrc is the handlers' file's own types: WithCode uses domain, so
// it lives beside the import (#123).
const mtWithCodeSrc = `
type WithCode struct {
	domain.Tag
	X int ` + "`json:\"x\"`" + `
}
`

// writeMTProject writes a module (resolveGoMod) holding the domain package,
// mod/types.go, mod/extra.go and mod/module.go (moduleSrc), and returns its
// root.
func writeMTProject(t *testing.T, moduleSrc string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":                             resolveGoMod,
		filepath.Join("domain", "domain.go"): mtDomainSrc,
		filepath.Join("mod", "types.go"):     mtTypesSrc,
		filepath.Join("mod", "extra.go"):     mtExtraSrc,
		filepath.Join("mod", "module.go"):    moduleSrc,
	}
	for rel, src := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o600))
	}
	return dir
}

// mtModuleHead is the handlers' file's head: the Module boilerplate and the
// types that use domain.
const mtModuleHead = `package mod

import (
	"github.com/example/app/domain"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

var _ server.IAPIError
` + mtWithCodeSrc

// mtFieldsSrc is Appendix B's Fields: one field per row.
const mtFieldsSrc = `
type Fields struct {
	Level     Level             ` + "`json:\"level\" validate:\"required,min=1,max=3\" example:\"high\"`" + `
	LevelPtr  *Level            ` + "`json:\"levelPtr\"`" + `
	LevelA    LA                ` + "`json:\"levelA\"`" + `
	LevelP    LevelP            ` + "`json:\"levelP\"`" + `
	LevelOne  Level             ` + "`json:\"levelOne\" validate:\"oneof=1 2 3\" example:\"2\"`" + `
	Levels    []Level           ` + "`json:\"levels\" validate:\"min=1,dive,max=3\"`" + `
	Grid      [][]Level         ` + "`json:\"grid\"`" + `
	LevelMap  map[string]Level  ` + "`json:\"levelMap\"`" + `
	LevelPMap map[string]LevelP ` + "`json:\"levelPMap\"`" + `
	Flags     []FlagT2          ` + "`json:\"flags\"`" + `
	Code      Code              ` + "`json:\"code\" validate:\"min=2,max=5,email\"`" + `
	LD        LD                ` + "`json:\"ld\"`" + `
	StatusV   StatusV           ` + "`json:\"statusV\"`" + `
	LevelU    LevelU            ` + "`json:\"levelU\"`" + `
	Both      Both              ` + "`json:\"both\"`" + `
	TextJ     TextJ             ` + "`json:\"textJ\"`" + `
	TextApp   TextApp           ` + "`json:\"textApp\"`" + `
	AppOnly   AppOnly           ` + "`json:\"appOnly\"`" + `
	ToText    ToText            ` + "`json:\"toText\"`" + `
	FromOnly  FromOnly          ` + "`json:\"fromOnly\"`" + `

	Money     Money                   ` + "`json:\"money\"`" + `
	MoneyPtr  *Money                  ` + "`json:\"moneyPtr\"`" + `
	Moneys    []Money                 ` + "`json:\"moneys\"`" + `
	MoneyMap  map[string]Money        ` + "`json:\"moneyMap\"`" + `
	MoneyP    MoneyP                  ` + "`json:\"moneyP\"`" + `
	MoneyU    MoneyU                  ` + "`json:\"moneyU\"`" + `
	MA        MA                      ` + "`json:\"ma\"`" + `
	Price     domain.Price            ` + "`json:\"price\"`" + `
	MoneyPPtr *MoneyP                 ` + "`json:\"moneyPPtr\"`" + `
	MoneyPs   []MoneyP                ` + "`json:\"moneyPs\"`" + `
	MoneyPMap map[string]MoneyP       ` + "`json:\"moneyPMap\"`" + `
	MoneyUPtr *MoneyU                 ` + "`json:\"moneyUPtr\"`" + `
	MoneyUs   []MoneyU                ` + "`json:\"moneyUs\"`" + `
	MoneyUMap map[string]MoneyU       ` + "`json:\"moneyUMap\"`" + `
	MAPtr     *MA                     ` + "`json:\"maPtr\"`" + `
	MAs       []MA                    ` + "`json:\"mas\"`" + `
	MAMap     map[string]MA           ` + "`json:\"maMap\"`" + `
	PricePtr  *domain.Price           ` + "`json:\"pricePtr\"`" + `
	Prices    []domain.Price          ` + "`json:\"prices\"`" + `
	PriceMap  map[string]domain.Price ` + "`json:\"priceMap\"`" + `
	MoneyT    MoneyT                  ` + "`json:\"moneyT\"`" + `
	MoneyTPtr *MoneyT                 ` + "`json:\"moneyTPtr\"`" + `
	MoneyTs   []MoneyT                ` + "`json:\"moneyTs\"`" + `
	Amount    domain.Amount           ` + "`json:\"amount\"`" + `
	EmbP      EmbP                    ` + "`json:\"embP\"`" + `
	MoneyOdd  MoneyOdd                ` + "`json:\"moneyOdd\"`" + `
	MD        MD                      ` + "`json:\"md\"`" + `
	Pair      Pair                    ` + "`json:\"pair\"`" + `
	Wrapper   Wrapper                 ` + "`json:\"wrapper\"`" + `
	Stamped   Stamped                 ` + "`json:\"stamped\"`" + `
	Twin      Twin                    ` + "`json:\"twin\"`" + `
	Shadow    Shadow                  ` + "`json:\"shadow\"`" + `
	WithInner MoneyWithInner          ` + "`json:\"withInner\"`" + `
	WrapLevel WrapLevel               ` + "`json:\"wrapLevel\"`" + `
	PtrEmb    PtrEmb                  ` + "`json:\"ptrEmb\"`" + `
	TaggedEmb TaggedEmb               ` + "`json:\"taggedEmb\"`" + `
	Deep      Deep                    ` + "`json:\"deep\"`" + `
	WithID    WithID                  ` + "`json:\"withID\"`" + `
	WithCode  WithCode                ` + "`json:\"withCode\"`" + `
	Tag       domain.Tag              ` + "`json:\"tag\"`" + `
	Raw       Raw                     ` + "`json:\"raw\"`" + `
}
`

// mtModule is the handlers' file: mtModuleHead, extra declarations, and one
// route per "METHOD path handler signature" line of routes.
func mtModule(decls string, routes ...string) string {
	var reg, handlers strings.Builder
	for i, r := range routes {
		parts := strings.SplitN(r, " ", 3)
		name := fmt.Sprintf("h%d", i)
		fmt.Fprintf(&reg, "\tserver.%s(hr, r, %q, m.%s)\n", parts[0], parts[1], name)
		fmt.Fprintf(&handlers, "func (m *Module) %s%s { panic(0) }\n\n", name, parts[2])
	}
	return mtModuleHead + decls +
		"\nfunc (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" + reg.String() + "}\n\n" +
		handlers.String()
}

// analyzeMT analyzes a writeMTProject project and returns the analyzer and
// its routes.
func analyzeMT(t *testing.T, moduleSrc string) (*ProjectAnalyzer, []models.Route) {
	t.Helper()
	a := New(writeMTProject(t, moduleSrc))
	project, err := a.AnalyzeProject()
	require.NoError(t, err)
	var routes []models.Route
	for i := range project.Modules {
		routes = append(routes, project.Modules[i].Routes...)
	}
	return a, routes
}

// mtFieldsProject is the GET /fields project every field row is read from.
func mtFieldsProject(t *testing.T) *ProjectAnalyzer {
	t.Helper()
	a, _ := analyzeMT(t, mtModule(mtFieldsSrc,
		"GET /fields (ctx server.HandlerContext) (server.Result[Fields], server.IAPIError)"))
	return a
}

// TestMarshalerTypeFieldResolution pins every S, M and U field row's
// Resolution: text leaves, Marshaler leaves (a struct's with no Elem) and
// refs. A MarshalText only on the pointer (LevelP, and EmbP's value embed of
// it) is not a string both ways: encoding/json skips it on a non-addressable
// value, such as this Result[Fields] payload's fields.
func TestMarshalerTypeFieldResolution(t *testing.T) {
	got := renders(t, mtFieldsProject(t).typeRegistry[mtFields])
	want := map[string]string{
		"level": "text:Level", "levelPtr": "*text:Level", "levelA": "text:Level", "levelP": "marshal:LevelP(int)",
		"levelOne": "text:Level", "levels": "[]text:Level", "grid": "[][]text:Level",
		"levelMap": "map[string]text:Level", "levelPMap": "map[string]marshal:LevelP(int)", "flags": "[]text:FlagT2",
		"code": "text:Code", "ld": "int", "statusV": "marshal:StatusV(int)", "levelU": "marshal:LevelU(int)",
		"both": "marshal:Both(int)", "textJ": "marshal:TextJ(int)", "textApp": "text:TextApp",
		"appOnly": "marshal:AppOnly(int)", "toText": "marshal:ToText(int)", "fromOnly": "marshal:FromOnly(int)",
		"moneyT": "text:MoneyT", "moneyTPtr": "*text:MoneyT", "moneyTs": "[]text:MoneyT", "amount": "text:Amount",
		"embP": "marshal:EmbP(unknown)", "moneyOdd": "$MoneyOdd", "md": "$MD", "pair": "$Pair",
		"wrapper": "marshal:Wrapper(unknown)", "stamped": "marshal:Stamped(unknown)", "twin": "marshal:Twin(unknown)",
		"shadow": "marshal:Shadow(unknown)", "withInner": "marshal:MoneyWithInner(unknown)",
		"wrapLevel": "text:WrapLevel", "ptrEmb": "text:PtrEmb", "taggedEmb": "text:TaggedEmb", "deep": "text:Deep",
		"withID": "text:WithID", "withCode": "text:WithCode", "tag": "text:Tag", "raw": "marshal:Raw(unknown)",
	}
	for _, four := range []struct{ prefix, leaf string }{
		{"money", "marshal:Money(unknown)"}, {"moneyP", "marshal:MoneyP(unknown)"}, {"moneyU", "marshal:MoneyU(unknown)"},
		{"ma", "marshal:MA(unknown)"}, {"price", "marshal:Price(unknown)"},
	} {
		want[four.prefix] = four.leaf
		want[four.prefix+"Ptr"] = "*" + four.leaf
		want[four.prefix+"Map"] = "map[string]" + four.leaf
	}
	want["moneys"], want["moneyPs"], want["moneyUs"] = "[]marshal:Money(unknown)", "[]marshal:MoneyP(unknown)", "[]marshal:MoneyU(unknown)"
	want["mas"], want["prices"] = "[]marshal:MA(unknown)", "[]marshal:Price(unknown)"
	assert.Equal(t, want, got)
}

// TestStructMarshalerRegistersNoComponent pins that no struct Marshaler type,
// and nothing reachable only through one, registers.
func TestStructMarshalerRegistersNoComponent(t *testing.T) {
	a := mtFieldsProject(t)
	assert.ElementsMatch(t, []string{mtFields, "MD", "MoneyOdd", "Pair"}, slices.Collect(maps.Keys(a.typeRegistry)))
	for _, name := range []string{
		mtMoney, "MoneyP", "MoneyU", "MA", "Price", mtMoneyT, "Amount", "EmbP", "Wrapper", "Stamped", "Twin", "Shadow",
		"MoneyWithInner", "OnlyInner", mtWrapLevel, "PtrEmb", "TaggedEmb", "Deep", "WithID", "WithCode", "Raw",
	} {
		assert.NotContains(t, a.typeRegistry, name)
	}
}

// TestMarshalerTypeFieldWarnings pins one warning per Marshaler field, with
// its exact text, and none for a text or unchanged row.
func TestMarshalerTypeFieldWarnings(t *testing.T) {
	a := mtFieldsProject(t)
	type row struct{ field, written, typeName, method, via string }
	rows := make([]row, 0, 36)
	rows = append(rows, []row{
		{"LevelP", "LevelP", "LevelP", methodMarshalText, ""},
		{"LevelPMap", "map[string]LevelP", "LevelP", methodMarshalText, ""},
		{"EmbP", "EmbP", "EmbP", methodMarshalText, "LevelP"},
		{"StatusV", mtStatusV, mtStatusV, methodMarshalText, ""},
		{"LevelU", "LevelU", "LevelU", methodUnmarshalText, ""},
		{"Both", "Both", "Both", methodMarshalText, ""},
		{"TextJ", "TextJ", "TextJ", methodMarshalText, ""},
		{"AppOnly", "AppOnly", "AppOnly", methodAppendText, ""},
		{"ToText", "ToText", "ToText", methodMarshalJSONTo, ""},
		{"FromOnly", "FromOnly", "FromOnly", methodUnmarshalJSONFrom, ""},
		{"Wrapper", "Wrapper", "Wrapper", methodMarshalText, mtStatusV},
		{"Stamped", "Stamped", "Stamped", methodMarshalJSON, models.WellKnownTimeTime},
		{"Twin", "Twin", "Twin", methodUnmarshalText, mtLevel},
		{"Shadow", "Shadow", "Shadow", methodMarshalJSON, ""},
		{"WithInner", "MoneyWithInner", "MoneyWithInner", methodMarshalJSON, ""},
		{"Raw", "Raw", "Raw", methodMarshalJSON, models.WellKnownRawMessage},
	}...)
	for _, four := range []struct{ field, written, typeName, method string }{
		{mtMoney, mtMoney, mtMoney, methodMarshalJSON}, {"MoneyP", "MoneyP", "MoneyP", methodMarshalJSON},
		{"MoneyU", "MoneyU", "MoneyU", methodUnmarshalJSON}, {"MA", "MA", mtMoney, methodMarshalJSON},
		{"Price", mtPrice, mtPrice, methodMarshalJSON},
	} {
		plural := map[string]string{mtMoney: "Moneys", "MA": "MAs", "Price": "Prices"}[four.field]
		if plural == "" {
			plural = four.field + "s"
		}
		rows = append(rows,
			row{four.field, four.written, four.typeName, four.method, ""},
			row{four.field + "Ptr", "*" + four.written, four.typeName, four.method, ""},
			row{plural, "[]" + four.written, four.typeName, four.method, ""},
			row{four.field + "Map", "map[string]" + four.written, four.typeName, four.method, ""})
	}
	for _, r := range rows {
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
	assert.Len(t, a.warnings, len(rows), "no other warning: %v", a.warnings)
}

// TestMarshalerTypeParamsByKind pins that parameters stay typed by kind: a
// text Level is an integer, and a struct Marshaler type keeps its component.
func TestMarshalerTypeParamsByKind(t *testing.T) {
	decls := "\ntype Params struct {\n\tLevel  Level  `query:\"level\"`\n\tHLevel Level  `header:\"X-Level\"`\n" +
		"\tM      MoneyT `query:\"m\"`\n}\n"
	a, _ := analyzeMT(t, mtModule(decls,
		"GET /params (req Params, ctx server.HandlerContext) (server.Result[int], server.IAPIError)"))
	got := renders(t, a.typeRegistry["Params"])
	assert.Equal(t, "int", got["Level"])
	assert.Equal(t, "int", got["HLevel"])
	assert.Equal(t, "$MoneyT", got["M"])
	assert.Contains(t, a.typeRegistry, mtMoneyT)
	assert.Empty(t, a.warnings)
}

// TestStructMarshalerUnderlyingMode pins a defined type over a qualified
// alias of a struct: the struct's promoted methods are kept, its declared
// ones dropped.
func TestStructMarshalerUnderlyingMode(t *testing.T) {
	run := func(t *testing.T, qSrc string) string {
		t.Helper()
		a := analyzeDirectiveProject(t, map[string]string{
			"go.mod":                          resolveGoMod,
			filepath.Join("q", "q.go"):        qSrc,
			filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/q\"\n", "type D q.UA\n", "\tD D `json:\"d\"`\n"),
		})
		return renders(t, a.typeRegistry["Rows"])["d"]
	}
	const head = "package q\n\ntype UA = User\n\n"
	t.Run("promoted kept", func(t *testing.T) {
		src := head + "type Level int\n\nfunc (l Level) MarshalText() ([]byte, error) { return nil, nil }\n" +
			"func (l *Level) UnmarshalText(b []byte) error { return nil }\n\ntype User struct{ Level }\n"
		assert.Equal(t, "text:UA", run(t, src))
	})
	t.Run("declared dropped", func(t *testing.T) {
		src := head + "type User struct{ N int }\n\nfunc (u User) MarshalJSON() ([]byte, error) { return nil, nil }\n"
		assert.Equal(t, "$UA", run(t, src))
	})
}

// TestStructMarshalerFreesShortName pins review round 3's finding 2: a struct
// Marshaler type no longer claims its short component name, so a same-named
// plain struct of another package takes it.
func TestStructMarshalerFreesShortName(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("a", "a.go"): "package a\n\ntype Money struct {\n\tAmount int64 `json:\"amount\"`\n}\n\n" +
			"func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }\n",
		filepath.Join("b", "b.go"): "package b\n\ntype Money struct {\n\tCents int64 `json:\"cents\"`\n}\n",
		filepath.Join("mod", "module.go"): strings.Replace(resolveModuleHead, "import (\n",
			"import (\n\t\"github.com/example/app/a\"\n\t\"github.com/example/app/b\"\n", 1) +
			"\ntype Out struct {\n\tA a.Money `json:\"a\"`\n\tB b.Money `json:\"b\"`\n}\n\n" +
			"func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" +
			"\tserver.GET(hr, r, \"/out\", m.out)\n}\n\n" +
			"func (m *Module) out(ctx server.HandlerContext) (server.Result[Out], server.IAPIError) { panic(0) }\n",
	}
	for rel, src := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(src), 0o600))
	}
	project, err := New(dir).AnalyzeProject()
	require.NoError(t, err)
	out := project.Types["Out"]
	require.NotNil(t, out)
	assert.Equal(t, "$"+mtMoney, renderShape(fieldByJSON(t, out, "b").ResolvedShape()))
	assert.Equal(t, "marshal:Money(unknown)", renderShape(fieldByJSON(t, out, "a").ResolvedShape()))
	require.Contains(t, project.Types, mtMoney)
	assert.Equal(t, "Cents", project.Types[mtMoney].Fields[0].Name)
	assert.NotContains(t, project.Types, "BMoney")
	assert.Len(t, project.Types, 2, "Out and b.Money only")
}

// TestMethodSetMemoResetBetweenRuns proves the method-set memo is per-run
// state: once Level gains its own MarshalJSON on disk, a second analysis on
// the same analyzer must see WrapLevel promote it and stop treating WrapLevel
// as a text type.
func TestMethodSetMemoResetBetweenRuns(t *testing.T) {
	dir := writeMTProject(t, mtModule(mtFieldsSrc,
		"GET /fields (ctx server.HandlerContext) (server.Result[Fields], server.IAPIError)"))
	a := New(dir)
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	require.Equal(t, "text:WrapLevel", renders(t, a.typeRegistry[mtFields])["wrapLevel"])

	typesFile := filepath.Join(dir, "mod", "types.go")
	src, err := os.ReadFile(typesFile)
	require.NoError(t, err)
	src = append(src, "\nfunc (l Level) MarshalJSON() ([]byte, error) { return nil, nil }\n"...)
	require.NoError(t, os.WriteFile(typesFile, src, 0o600))

	_, err = a.AnalyzeProject()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(renders(t, a.typeRegistry[mtFields])["wrapLevel"], "marshal:"+mtWrapLevel),
		"a second run must not reuse the first run's method sets, got %q", renders(t, a.typeRegistry[mtFields])["wrapLevel"])
}
