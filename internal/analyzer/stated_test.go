package analyzer

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// statedModHead is a module whose one route returns server.Result[Holder];
// a test appends Holder and the types it exercises.
const statedModHead = resolveModuleHead + `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/h", m.h)
}

func (m *Module) h(ctx server.HandlerContext) (server.Result[Holder], server.IAPIError) {
	return server.Result[Holder]{}, nil
}
`

const (
	statedModFile = "module.go"
	moneyDecl     = "type Money struct{ Units int64 }\n\nfunc (m Money) MarshalJSON() ([]byte, error) { return nil, nil }\n"
	moneyField    = "type Holder struct {\n\tM Money `json:\"m\"`\n}\n"
	moneyWarning  = "field M at %s has type Money: Money has its own MarshalJSON method"
	stringDecimal = "//openapi:schema string decimal\n"
)

// analyzeStated analyzes a project whose mod/module.go is statedModHead+body,
// plus extra files (path relative to the project root -> source).
func analyzeStated(t *testing.T, body string, extra map[string]string) *ProjectAnalyzer {
	t.Helper()
	files := map[string]string{
		"go.mod":                            resolveGoMod,
		filepath.Join("mod", statedModFile): statedModHead + body,
	}
	for k, v := range extra {
		files[k] = v
	}
	return analyzeDirectiveProject(t, files)
}

// statedLoc is the "mod/module.go:<line>: " prefix of a diagnostic raised on
// the first line of statedModHead+body that contains marker.
func statedLoc(t *testing.T, body, marker string) string {
	t.Helper()
	return fileLoc(t, filepath.Join("mod", statedModFile), statedModHead+body, marker)
}

// fileLoc is the "<rel>:<line>: " prefix of the first line of src holding
// marker.
func fileLoc(t *testing.T, rel, src, marker string) string {
	t.Helper()
	i := strings.Index(src, marker)
	require.GreaterOrEqual(t, i, 0, "marker %q not in source", marker)
	return rel + ":" + strconv.Itoa(strings.Count(src[:i], "\n")+1) + ": "
}

// holderLeaf returns the Resolution of Holder's field json, every pointer,
// slice and map level shed.
func holderLeaf(t *testing.T, a *ProjectAnalyzer, json string) models.TypeShape {
	t.Helper()
	r := fieldByJSON(t, a.typeRegistry["Holder"], json).Resolution
	require.NotNil(t, r)
	for r.Elem != nil && r.Kind != models.ShapeMarshaler {
		r = r.Elem
	}
	return *r
}

// assertStated asserts that Holder's field json is a leaf of kind with the
// Stated schema want (nil: none).
func assertStated(t *testing.T, a *ProjectAnalyzer, json string, kind models.ShapeKind, want *models.StatedSchema) {
	t.Helper()
	leaf := holderLeaf(t, a, json)
	assert.Equal(t, kind, leaf.Kind, json)
	assert.Equal(t, want, leaf.Stated, json)
}

// hasWarningPrefix reports whether one of warnings starts with prefix.
func hasWarningPrefix(warnings []string, prefix string) bool {
	for _, w := range warnings {
		if strings.HasPrefix(w, prefix) {
			return true
		}
	}
	return false
}

func TestParseSchemaArgs(t *testing.T) {
	cases := []struct {
		args    string
		want    models.StatedSchema
		problem string
	}{
		{"string", models.StatedSchema{Type: "string"}, ""},
		{"string decimal", models.StatedSchema{Type: "string", Format: "decimal"}, ""},
		{"boolean flag", models.StatedSchema{Type: "boolean", Format: "flag"}, ""},
		{"integer", models.StatedSchema{Type: "integer"}, ""},
		{"number\tdouble", models.StatedSchema{Type: "number", Format: "double"}, ""},
		{"", models.StatedSchema{}, schemaNoArgsMsg},
		{"decimal", models.StatedSchema{}, `directive schema: ignoring "decimal" — expected string, integer, number or boolean`},
		{"String", models.StatedSchema{}, `directive schema: ignoring "String" — expected string, integer, number or boolean`},
		{"object", models.StatedSchema{}, `directive schema: ignoring "object" — expected string, integer, number or boolean`},
		{"array", models.StatedSchema{}, `directive schema: ignoring "array" — expected string, integer, number or boolean`},
		{"string decimal x", models.StatedSchema{}, `directive schema: ignoring "string decimal x" — expected a type and at most one format`},
	}
	for _, c := range cases {
		t.Run(c.args, func(t *testing.T) {
			got, problem := parseSchemaArgs(c.args)
			assert.Equal(t, c.want, got)
			assert.Equal(t, c.problem, problem)
		})
	}
}

// TestSchemaDirectiveArgumentWarnings pins the exact text of each argument
// diagnostic, raised once at its line, the line then ignored.
func TestSchemaDirectiveArgumentWarnings(t *testing.T) {
	cases := []struct{ line, want string }{
		{"//openapi:schema", schemaNoArgsMsg},
		{"//openapi:schema decimal", `directive schema: ignoring "decimal" — expected string, integer, number or boolean`},
		{"//openapi:schema String", `directive schema: ignoring "String" — expected string, integer, number or boolean`},
		{"//openapi:schema object", `directive schema: ignoring "object" — expected string, integer, number or boolean`},
		{"//openapi:schema string decimal extra // why", `directive schema: ignoring "string decimal extra" — expected a type and at most one format`},
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			// T is a Marshaler type no route reaches: only the line warns.
			body := c.line + "\ntype T int\n\nfunc (T) MarshalJSON() ([]byte, error) { return nil, nil }\n\ntype Holder struct{}\n"
			a := analyzeStated(t, body, nil)
			assert.Equal(t, []string{statedLoc(t, body, c.line) + c.want}, a.Warnings(t.Context()))
		})
	}
}

// TestSchemaDirectiveDetached pins the Detached text of a schema directive
// outside a package-level type declaration's doc comment, and that it then
// states nothing.
func TestSchemaDirectiveDetached(t *testing.T) {
	const detached = "directive schema has no effect — it is not in the doc comment of a package-level type declaration"
	cases := []struct{ name, body, marker string }{
		{"in a function body", "func f() {\n\t" + stringDecimal + "\ttype Local int\n\t_ = Local(0)\n}\n", "//openapi:schema"},
		{"as a trailing comment", "type Other int //openapi:schema string decimal\n", "//openapi:schema"},
		{"above a non-type declaration", stringDecimal + "var V = 1\n", "//openapi:schema"},
		{"above a function", stringDecimal + "func g() {}\n", "//openapi:schema"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := moneyDecl + moneyField + c.body
			a := analyzeStated(t, body, nil)
			warnings := a.Warnings(t.Context())
			assert.Contains(t, warnings, statedLoc(t, body, c.marker)+detached)
			assertStated(t, a, "m", models.ShapeMarshaler, nil)
		})
	}
}

// TestSchemaDirectiveAboveRouteRegistration pins that a group above a route
// registration holding public and schema applies public to the route and
// reports schema as Detached.
func TestSchemaDirectiveAboveRouteRegistration(t *testing.T) {
	src := resolveModuleHead + `
type Holder struct{}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:public
	//openapi:schema string
	server.GET(hr, r, "/h", m.h)
}

func (m *Module) h(ctx server.HandlerContext) (server.Result[Holder], server.IAPIError) {
	return server.Result[Holder]{}, nil
}
`
	a, routes := analyzeSingleModule(t, src)
	require.Len(t, routes, 1)
	assert.True(t, routes[0].Public)
	loc := fileLoc(t, filepath.Join("mod", statedModFile), src, "//openapi:public")
	assert.Equal(t, []string{loc + "directive schema has no effect — it is not in the doc comment of a package-level type declaration"},
		a.Warnings(t.Context()))
}

// TestSchemaDirectivePlacedBesidePublic pins per-name placement in a type's
// doc comment: schema applies, and public gets today's Detached text.
func TestSchemaDirectivePlacedBesidePublic(t *testing.T) {
	body := "//openapi:public\n" + stringDecimal + moneyDecl + moneyField
	a := analyzeStated(t, body, nil)
	assert.Equal(t, []string{statedLoc(t, body, "//openapi:public") +
		"directive public has no effect — it is not directly above an analyzed route registration"}, a.Warnings(t.Context()))
	assertStated(t, a, "m", models.ShapeMarshaler, &models.StatedSchema{Type: "string", Format: "decimal"})
}

// TestSchemaDirectiveBothMisplaced pins that a group whose route and type
// directives are both misplaced raises one diagnostic, at its first
// recognised directive, naming each with the placement it needs.
func TestSchemaDirectiveBothMisplaced(t *testing.T) {
	body := moneyDecl + moneyField + "func f() {\n\t//openapi:public\n\t//openapi:errors 404\n\t//openapi:schema string\n\t_ = 1\n}\n"
	a := analyzeStated(t, body, nil)
	assert.Equal(t, []string{fmtMoneyWarning(t, body), statedLoc(t, body, "//openapi:public") + "directives public, errors, schema have no effect — " +
		"public, errors are not directly above an analyzed route registration, and schema is not in the doc comment of a " +
		"package-level type declaration"}, a.Warnings(t.Context()))
}

// fmtMoneyWarning is the start of Holder.m's Marshaler warning: Money stays
// {} with it.
func fmtMoneyWarning(t *testing.T, body string) string {
	t.Helper()
	return strings.Replace(moneyWarning, "%s", strings.TrimSuffix(statedLoc(t, body, "M Money"), ": ")+":2", 1) +
		", which encoding/json uses instead of its underlying type — emitting an untyped schema ({}); parameters are typed by kind and unaffected"
}

func TestDetachedDirectiveMessage(t *testing.T) {
	cases := []struct {
		names []string
		want  string
	}{
		{[]string{"public"}, "directive public has no effect — it is not directly above an analyzed route registration"},
		{[]string{"public", "errors"}, "directives public, errors have no effect — they are not directly above an analyzed route registration"},
		{[]string{"schema"}, "directive schema has no effect — it is not in the doc comment of a package-level type declaration"},
		{[]string{"schema", "public"}, "directives schema, public have no effect — public is not directly above an analyzed route " +
			"registration, and schema is not in the doc comment of a package-level type declaration"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, detachedDirectiveMessage(c.names))
	}
}

// TestSchemaDirectiveAboveTypeGroup pins that a doc comment above a whole
// type group applies to no type, with its own diagnostic, and that a grouped
// type's own doc comment applies.
func TestSchemaDirectiveAboveTypeGroup(t *testing.T) {
	body := stringDecimal + "type (\n\tMoney struct{ Units int64 }\n\n\t//openapi:schema integer\n\tCount int\n)\n\n" +
		"func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }\n" +
		"func (c Count) MarshalJSON() ([]byte, error) { return nil, nil }\n" +
		"type Holder struct {\n\tM Money `json:\"m\"`\n\tC Count `json:\"c\"`\n}\n"
	a := analyzeStated(t, body, nil)
	assert.Equal(t, []string{fmtMoneyWarning(t, body), statedLoc(t, body, "//openapi:schema string") + schemaAboveGroupMsg},
		a.Warnings(t.Context()))
	assertStated(t, a, "m", models.ShapeMarshaler, nil)
	assertStated(t, a, "c", models.ShapeMarshaler, &models.StatedSchema{Type: "integer"})
}

// TestSchemaDirectiveRefusals pins the "has no effect" text for each type
// that does not accept a Stated schema, and that the type is then documented
// exactly as without the directive.
func TestSchemaDirectiveRefusals(t *testing.T) {
	cases := []struct {
		name, decls, field, marker, want string
		kind                             models.ShapeKind
		marshalerWarns                   bool
	}{
		{
			name: "not a Marshaler type", decls: "//openapi:schema string\ntype Plain int\n", field: "Plain",
			want: "directive schema has no effect — Plain is not a Marshaler type, so it is documented from its declaration",
			kind: models.ShapePrimitive,
		},
		{
			name: "pointer-only MarshalJSON", field: "PM", marshalerWarns: true, kind: models.ShapeMarshaler,
			decls: "//openapi:schema string\ntype PM struct{ U int }\n\nfunc (p *PM) MarshalJSON() ([]byte, error) { return nil, nil }\n",
			want: "directive schema has no effect — PM's MarshalJSON method is not in its value method set, and encoding/json " +
				"skips it on a map value or a payload returned by value; declare it on the value receiver",
		},
		{
			name: "value MarshalJSON, pointer MarshalText", field: "VT", marshalerWarns: true, kind: models.ShapeMarshaler,
			decls: "//openapi:schema string\ntype VT int\n\nfunc (v VT) MarshalJSON() ([]byte, error) { return nil, nil }\n" +
				"func (v *VT) MarshalText() ([]byte, error) { return nil, nil }\n",
			want: "directive schema has no effect — VT's MarshalText method is not in its value method set, and encoding/json " +
				"skips it on a map value or a payload returned by value; declare it on the value receiver",
		},
		{
			name: "pointer-only method promoted through a value embed", field: "EP", marshalerWarns: true, kind: models.ShapeMarshaler,
			decls: "type PM struct{ U int }\n\nfunc (p *PM) MarshalJSON() ([]byte, error) { return nil, nil }\n\n" +
				"//openapi:schema string\ntype EP struct{ PM }\n",
			want: "directive schema has no effect — EP's MarshalJSON method is not in its value method set, and encoding/json " +
				"skips it on a map value or a payload returned by value; declare it on the value receiver",
		},
		{
			name: "text both ways stated as integer", field: "Lvl", kind: models.ShapeText,
			decls: "//openapi:schema integer\ntype Lvl int\n\nfunc (l Lvl) MarshalText() ([]byte, error) { return nil, nil }\n" +
				"func (l *Lvl) UnmarshalText(b []byte) error { return nil }\n",
			want: "directive schema integer has no effect — Lvl encodes as text both ways, so only string [<format>] applies",
		},
		{
			name: "plain alias", field: "MA", marshalerWarns: true, kind: models.ShapeMarshaler,
			decls: moneyDecl + "\n//openapi:schema string\ntype MA = Money\n",
			want:  "directive schema has no effect — MA is an alias of Money; state it on Money",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := c.decls + "type Holder struct {\n\tF " + c.field + " `json:\"f\"`\n}\n"
			a := analyzeStated(t, body, nil)
			warnings := a.Warnings(t.Context())
			assert.Contains(t, warnings, statedLoc(t, body, "//openapi:schema")+c.want)
			assert.Equal(t, c.marshalerWarns, hasWarningPrefix(warnings, "field F at "), "the Marshaler warning stays: %v", warnings)
			assert.Len(t, warnings, map[bool]int{false: 1, true: 2}[c.marshalerWarns], "%v", warnings)
			assertStated(t, a, "f", c.kind, nil)
		})
	}
}

// TestSchemaDirectiveOnUnreachedType pins that the diagnostics cover every
// type declaration in a walked file, whether or not a route reaches it, and
// that an unreached Marshaler type with a valid directive raises nothing.
func TestSchemaDirectiveOnUnreachedType(t *testing.T) {
	body := "//openapi:schema string\ntype Plain int\n\n" + stringDecimal + moneyDecl + "type Holder struct{}\n"
	a := analyzeStated(t, body, nil)
	assert.Equal(t, []string{statedLoc(t, body, "//openapi:schema string\n") +
		"directive schema has no effect — Plain is not a Marshaler type, so it is documented from its declaration"}, a.Warnings(t.Context()))
}

// TestSchemaDirectiveOnGenericType pins that a directive on a generic type
// declaration never applies, with or without generic-receiver marshal
// methods, decided from its type parameters.
func TestSchemaDirectiveOnGenericType(t *testing.T) {
	body := "//openapi:schema string\ntype JBox[T any] struct{ V T }\n\n" +
		"//openapi:schema string\ntype KBox[T any] struct{ V T }\n\nfunc (b KBox[T]) MarshalJSON() ([]byte, error) { return nil, nil }\n\n" +
		"type Holder struct {\n\tK KBox[int] `json:\"k\"`\n}\n"
	a := analyzeStated(t, body, nil)
	warnings := a.Warnings(t.Context())
	assert.Contains(t, warnings, statedLoc(t, body, "//openapi:schema string\ntype JBox")+
		"directive schema has no effect — JBox is a generic type, whose wire form usually depends on its type arguments")
	assert.Contains(t, warnings, statedLoc(t, body, "//openapi:schema string\ntype KBox")+
		"directive schema has no effect — KBox is a generic type, whose wire form usually depends on its type arguments")
	// KBox[int] is documented exactly as without the directive: no Stated schema.
	k := fieldByJSON(t, a.typeRegistry["Holder"], "k").Resolution
	require.NotNil(t, k)
	assert.Equal(t, models.ShapeInstantiation, k.Kind)
	assert.Nil(t, k.Stated, "%v", warnings)
}

// TestSchemaDirectiveRepeatedLines pins the duplicate rule within one group:
// identical lines collapse silently, different ones warn once and none
// applies, and an invalid line warns for itself beside a valid one that
// applies.
func TestSchemaDirectiveRepeatedLines(t *testing.T) {
	t.Run("identical", func(t *testing.T) {
		body := stringDecimal + stringDecimal + moneyDecl + moneyField
		a := analyzeStated(t, body, nil)
		assert.Empty(t, a.Warnings(t.Context()))
		assertStated(t, a, "m", models.ShapeMarshaler, &models.StatedSchema{Type: "string", Format: "decimal"})
	})
	t.Run("different", func(t *testing.T) {
		body := stringDecimal + "//openapi:schema integer\n" + moneyDecl + moneyField
		a := analyzeStated(t, body, nil)
		assert.Equal(t, []string{fmtMoneyWarning(t, body), statedLoc(t, body, "//openapi:schema") + schemaConflictMsg}, a.Warnings(t.Context()))
		assertStated(t, a, "m", models.ShapeMarshaler, nil)
	})
	t.Run("invalid beside valid", func(t *testing.T) {
		body := "//openapi:schema object\n" + stringDecimal + moneyDecl + moneyField
		a := analyzeStated(t, body, nil)
		assert.Equal(t, []string{statedLoc(t, body, "//openapi:schema object") +
			`directive schema: ignoring "object" — expected string, integer, number or boolean`}, a.Warnings(t.Context()))
		assertStated(t, a, "m", models.ShapeMarshaler, &models.StatedSchema{Type: "string", Format: "decimal"})
	})
}

// TestSchemaDirectiveBuildTaggedDeclarations pins the duplicate rule across
// build-tagged declarations of one type: an unstated declaration states
// nothing, and two different Stated schemas warn and fall back.
func TestSchemaDirectiveBuildTaggedDeclarations(t *testing.T) {
	variant := func(tag, directive string) string {
		return "//go:build " + tag + "\n\npackage mod\n\n" + directive + "type Count int\n"
	}
	const (
		countMethod = "func (c Count) MarshalJSON() ([]byte, error) { return nil, nil }\n"
		countField  = "type Holder struct {\n\tC Count `json:\"c\"`\n}\n"
	)
	linux := filepath.Join("mod", "count_linux.go")
	other := filepath.Join("mod", "count_other.go")
	t.Run("stated beside unstated", func(t *testing.T) {
		a := analyzeStated(t, countMethod+countField, map[string]string{
			linux: variant("linux", "//openapi:schema integer\n"),
			other: variant("!linux", ""),
		})
		assert.Empty(t, a.Warnings(t.Context()))
		assertStated(t, a, "c", models.ShapeMarshaler, &models.StatedSchema{Type: "integer"})
	})
	t.Run("different", func(t *testing.T) {
		linuxSrc, otherSrc := variant("linux", "//openapi:schema integer\n"), variant("!linux", "//openapi:schema string\n")
		a := analyzeStated(t, countMethod+countField, map[string]string{linux: linuxSrc, other: otherSrc})
		const msg = "directive schema has no effect — the build-tagged declarations of Count state different schemas, so none applies"
		warnings := a.Warnings(t.Context())
		assert.Contains(t, warnings, fileLoc(t, linux, linuxSrc, "//openapi:schema")+msg)
		assert.Contains(t, warnings, fileLoc(t, other, otherSrc, "//openapi:schema")+msg)
		assert.True(t, hasWarningPrefix(warnings, "field C at "), "Count falls back with its Marshaler warning: %v", warnings)
		assertStated(t, a, "c", models.ShapeMarshaler, nil)
	})
}

// TestSchemaDirectiveNoInheritance pins that a defined type over Money and a
// struct promoting Money's method are documented exactly as without the
// directive, while a plain alias of Money uses Money's Stated schema.
func TestSchemaDirectiveNoInheritance(t *testing.T) {
	body := stringDecimal + moneyDecl + "type M2 Money\n\ntype W struct{ Money }\n\ntype MA = Money\n\n" +
		"type Holder struct {\n\tM2 M2 `json:\"m2\"`\n\tW W `json:\"w\"`\n\tMA MA `json:\"ma\"`\n}\n"
	a := analyzeStated(t, body, nil)
	assertStated(t, a, "m2", models.ShapeRef, nil)
	assertStated(t, a, "w", models.ShapeMarshaler, nil)
	assertStated(t, a, "ma", models.ShapeMarshaler, &models.StatedSchema{Type: "string", Format: "decimal"})
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 1, "only W's promoted-method warning: %v", warnings)
	assert.Contains(t, warnings[0], "field W at ")
}

// TestSchemaDirectiveOnPromotingStruct pins that a struct promoting a
// Marshaler method, a Marshaler type itself, takes its own directive.
func TestSchemaDirectiveOnPromotingStruct(t *testing.T) {
	body := moneyDecl + "//openapi:schema integer\ntype W struct{ Money }\n\ntype Holder struct {\n\tW W `json:\"w\"`\n}\n"
	a := analyzeStated(t, body, nil)
	assert.Empty(t, a.Warnings(t.Context()))
	assertStated(t, a, "w", models.ShapeMarshaler, &models.StatedSchema{Type: "integer"})
}

// TestSchemaDirectiveAliasReceiver pins that an alias which is itself a
// marshal-method receiver takes the directive, and its target is unchanged.
func TestSchemaDirectiveAliasReceiver(t *testing.T) {
	body := "type Status int\n\n//openapi:schema string\ntype SA = Status\n\nfunc (SA) MarshalJSON() ([]byte, error) { return nil, nil }\n\n" +
		"type Holder struct {\n\tSA SA `json:\"sa\"`\n\tS Status `json:\"s\"`\n}\n"
	a := analyzeStated(t, body, nil)
	assert.Empty(t, a.Warnings(t.Context()))
	assertStated(t, a, "sa", models.ShapeMarshaler, &models.StatedSchema{Type: "string"})
	assertStated(t, a, "s", models.ShapePrimitive, nil)
}

// TestSchemaDirectiveQualifiedGroupedType pins that a grouped declaration in
// another in-module package applies to a qualified use of it.
func TestSchemaDirectiveQualifiedGroupedType(t *testing.T) {
	holder := "type Holder struct {\n\tR ledger.Rate `json:\"r\"`\n\tP *ledger.Rate `json:\"p\"`\n}\n"
	ledgerSrc := "package ledger\n\ntype (\n\t//openapi:schema number double\n\tRate struct{ BP int }\n)\n\n" +
		"func (r Rate) MarshalJSON() ([]byte, error) { return nil, nil }\n"
	src := strings.Replace(statedModHead, "import (\n", "import (\n\t\"github.com/example/app/ledger\"\n", 1) + holder
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                            resolveGoMod,
		filepath.Join("mod", statedModFile): src,
		filepath.Join("ledger", "rate.go"):  ledgerSrc,
	})
	assert.Empty(t, a.Warnings(t.Context()))
	want := &models.StatedSchema{Type: "number", Format: "double"}
	assertStated(t, a, "r", models.ShapeMarshaler, want)
	assertStated(t, a, "p", models.ShapeMarshaler, want)
}

// TestSchemaDirectivePayloadAndRequest pins the payload and request roots: a
// Stated schema types them, so the routes count as typed, with no warning.
func TestSchemaDirectivePayloadAndRequest(t *testing.T) {
	src := resolveModuleHead + "\n" + stringDecimal + moneyDecl + `
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/pay", m.pay)
	server.GET(hr, r, "/ptr", m.ptr)
}

func (m *Module) pay(req Money, ctx server.HandlerContext) (server.Result[Money], server.IAPIError) {
	return server.Result[Money]{}, nil
}

func (m *Module) ptr(ctx server.HandlerContext) (server.Result[*Money], server.IAPIError) {
	return server.Result[*Money]{}, nil
}
`
	a, routes := analyzeSingleModule(t, src)
	assert.Empty(t, a.Warnings(t.Context()))
	require.Len(t, routes, 2)
	want := &models.StatedSchema{Type: "string", Format: "decimal"}
	for _, r := range routes {
		assert.True(t, IsTypedPayload(r.Response), r.Path)
		leaf := r.Response.ResolvedShape()
		if leaf.Kind == models.ShapePointer {
			leaf = leaf.Elem
		}
		assert.Equal(t, want, leaf.Stated, r.Path)
	}
	pay := routes[0]
	if pay.Path != "/pay" {
		pay = routes[1]
	}
	require.NotNil(t, pay.Request.Resolution)
	assert.Equal(t, want, pay.Request.Resolution.Stated)
	assert.True(t, IsTypedPayload(pay.Request))
}

// TestSchemaDirectiveNonStructRequest pins that a Stated schema does not
// reach a non-struct request: go-bricks binds requests by struct fields, so
// such a request keeps its own warning and gets no body, directive or not.
func TestSchemaDirectiveNonStructRequest(t *testing.T) {
	src := resolveModuleHead + `
//openapi:schema integer
type Count int

func (c Count) MarshalJSON() ([]byte, error) { return nil, nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/count", m.count)
}

func (m *Module) count(req Count, ctx server.HandlerContext) (server.Result[Count], server.IAPIError) {
	return server.Result[Count]{}, nil
}
`
	a, routes := analyzeSingleModule(t, src)
	warnings := a.Warnings(t.Context())
	require.Len(t, warnings, 1, "%v", warnings)
	assert.True(t, strings.HasPrefix(warnings[0], "request type Count is not a struct"), warnings[0])
	require.Len(t, routes, 1)
	assert.True(t, IsTypedPayload(routes[0].Response), "the payload still takes the Stated schema")
}

// TestStatedSpectestFixtureHasNoWarnings pins that the stated_schema golden
// fixture analyzes with zero warnings.
func TestStatedSpectestFixtureHasNoWarnings(t *testing.T) {
	a := New(filepath.Join("..", "spectest", "testdata", "stated_schema"))
	_, err := a.AnalyzeProject()
	require.NoError(t, err)
	assert.Empty(t, a.Warnings(t.Context()))
}

func TestMarshalerMethodsPtrEncode(t *testing.T) {
	var m marshalerMethods
	assert.Empty(t, m.ptrEncode())
	m = m.with(methodUnmarshalJSON, true)
	assert.Empty(t, m.ptrEncode(), "a decode-side method on the pointer is fine: a decode target is addressable")
	m = m.with(methodMarshalJSON, false)
	assert.Empty(t, m.ptrEncode())
	m = m.with(methodAppendText, true)
	assert.Equal(t, methodAppendText, m.ptrEncode())
	m = m.with(methodMarshalText, true)
	assert.Equal(t, methodMarshalText, m.ptrEncode(), "named in marshalerMethodOrder")
}
