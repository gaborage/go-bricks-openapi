package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

const (
	jsCents = "Cents"
	jsPI    = "PI"
)

// quotedField is a body field whose json tag carries ",string" (#115), with
// the syntactic Shape s resolved to r.
func quotedField(s, r models.TypeShape) *models.FieldInfo {
	return &models.FieldInfo{Name: "F", JSONName: "f", JSONString: true, Shape: s, Resolution: &r}
}

// TestJSONStringOptionQuotesScalars pins the retyped shapes (#115):
// encoding/json writes a bool, integer, float or json.Number tagged ",string"
// as a JSON string, through at most one pointer, so the field is a bare
// {type: string} with no format and no unsigned floor. The type is read from
// the Resolution: a named scalar (Cents), a well-known number and a kind-only
// scalar count by what they resolve to. A pointer written in the field's own
// declaration (*int64, *Cents) is quoted by both engines and stays nullable.
func TestJSONStringOptionQuotesScalars(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	str := &OpenAPIProperty{Type: typeString}
	nullableStr := &OpenAPIProperty{Type: typeString, Nullable: true}
	i64 := prim(goTypeInt64)
	cases := map[string]struct {
		f    *models.FieldInfo
		want *OpenAPIProperty
	}{
		"int64":                  {quotedField(i64, i64), str},
		"uint64 (minimum 0 too)": {quotedField(prim(goTypeUint64), prim(goTypeUint64)), str},
		"uint8 (int32 format)":   {quotedField(prim(goTypeUint8), prim(goTypeUint8)), str},
		"bool":                   {quotedField(prim(goTypeBool), prim(goTypeBool)), str},
		"float64":                {quotedField(prim(goTypeFloat64), prim(goTypeFloat64)), str},
		"Cents":                  {quotedField(named(jsCents), i64), str},
		"time.Duration":          {quotedField(named(goTypeTimeDuration), named(goTypeTimeDuration)), str},
		"json.Number":            {quotedField(named(goTypeJSONNumber), named(goTypeJSONNumber)), str},
		"kind-only integer":      {quotedField(named("Code"), kindOnlyLeaf(typeInteger)), str},
		"*int64":                 {quotedField(ptrOf(i64), ptrOf(i64)), nullableStr},
		"*Cents":                 {quotedField(ptrOf(named(jsCents)), ptrOf(i64)), nullableStr},
		"*json.Number":           {quotedField(ptrOf(named(goTypeJSONNumber)), ptrOf(named(goTypeJSONNumber))), nullableStr},
	}
	for name, tc := range cases {
		assert.Equal(t, tc.want, gen.fieldInfoToProperty(tc.f), name)
	}
}

// TestJSONStringOptionKeywords pins what a retyped field keeps, by analogy
// with a text Marshaler leaf (#111): its description, its example as a string
// and its `required` membership; its validate keywords constrain the Go value,
// not the quoted text, so they drop.
func TestJSONStringOptionKeywords(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	i64 := prim(goTypeInt64)
	f := quotedField(i64, i64)
	f.Description = "amount in cents"
	f.Example = "5"
	f.Constraints = map[string]string{"required": "true", "min": "1", "max": "9", "oneof": "1 5 9"}
	f.Required = true
	assert.Equal(t, &OpenAPIProperty{Type: typeString, Description: "amount in cents", Example: "5"}, gen.fieldInfoToProperty(f))

	b := quotedField(prim(goTypeBool), prim(goTypeBool))
	b.Example = "true"
	assert.Equal(t, &OpenAPIProperty{Type: typeString, Example: "true"}, gen.fieldInfoToProperty(b))

	schema := gen.typeInfoToSchema(&models.TypeInfo{Name: "Req", Fields: []models.FieldInfo{*f}})
	require.NotNil(t, schema)
	assert.Equal(t, []string{"f"}, schema.Required)
	assert.Equal(t, typeString, schema.Properties["f"].Type)
}

// TestJSONStringOptionLeavesOtherShapes pins every shape the option does not
// retype: each emits exactly what it emits without the option. encoding/json
// ignores the option on a string (it double-encodes, still a string), a
// slice, a map, an untyped value and a Marshaler type; on a type reached
// through two pointers it writes the value unquoted. A path, query or header
// parameter is bound by its own tag. A named pointer type (type PI *int64,
// type PC *Cents) is the toolchain-dependent case: the classic encoding/json
// (Go 1.26, the go.mod floor) writes it unquoted and Go 1.27's default
// engine quotes it, so it keeps its unquoted schema.
func TestJSONStringOptionLeavesOtherShapes(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	i64 := prim(goTypeInt64)
	strField := quotedField(prim(goTypeString), prim(goTypeString))
	strField.Constraints = map[string]string{"min": "1"}
	param := quotedField(i64, i64)
	param.ParamType = paramTypeHeader
	param.Constraints = map[string]string{"min": "1"}
	cases := map[string]*models.FieldInfo{
		"string":             strField,
		"[]int64":            quotedField(sliceOf(i64), sliceOf(i64)),
		"[2]int64":           quotedField(arrayOf(i64), arrayOf(i64)),
		"map[string]int64":   quotedField(mapOf(prim(goTypeString), i64), mapOf(prim(goTypeString), i64)),
		"any":                quotedField(prim(goTypeAny), prim(goTypeAny)),
		"time.Time":          quotedField(named(goTypeTimeTime), named(goTypeTimeTime)),
		"text Marshaler":     quotedField(named(mtLevel), textLeaf(mtLevel)),
		"Marshaler":          quotedField(named("M"), marshalerOf("M", i64)),
		"struct":             quotedField(named("Addr"), refTo("Addr")),
		"**int64":            quotedField(ptrOf(ptrOf(i64)), ptrOf(ptrOf(i64))),
		"*PI":                quotedField(ptrOf(named(jsPI)), ptrOf(ptrOf(i64))),
		"PI (named pointer)": quotedField(named(jsPI), ptrOf(i64)),
		"PC (named pointer)": quotedField(named("PC"), ptrOf(i64)),
		"header parameter":   param,
	}
	for name, f := range cases {
		plain := *f
		plain.JSONString = false
		assert.Equal(t, gen.fieldInfoToProperty(&plain), gen.fieldInfoToProperty(f), name)
	}

	// Pin the named-pointer side concretely, so a change to the unquoted
	// baseline cannot hide behind the comparison above.
	assert.Equal(t, &OpenAPIProperty{Type: typeInteger, Format: formatInt64, Nullable: true},
		gen.fieldInfoToProperty(quotedField(named(jsPI), ptrOf(i64))))
}

// TestJSONStringOptionExampleFitsScalar pins the example rule for a retyped
// field (#115): encoding/json decodes the quoted text as the Go scalar, so an
// example is kept exactly when the same field without ",string" keeps it
// (its type, int32 range, uint floor and validate bounds), and then in the
// form the server writes and reads back. One the scalar cannot hold is
// dropped, as it is on the unquoted field.
func TestJSONStringOptionExampleFitsScalar(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	i64 := prim(goTypeInt64)
	withExample := func(f *models.FieldInfo, ex string, constraints map[string]string) *models.FieldInfo {
		f.Example = ex
		f.Constraints = constraints
		return f
	}
	cases := map[string]struct {
		f    *models.FieldInfo
		want any
	}{
		"int64 kept":                {withExample(quotedField(i64, i64), "5", nil), "5"},
		"int64 canonical":           {withExample(quotedField(i64, i64), "+05", nil), "5"},
		"int64 not a number":        {withExample(quotedField(i64, i64), "abc", nil), nil},
		"int64 below validate min":  {withExample(quotedField(i64, i64), "0", map[string]string{"min": "1"}), nil},
		"int8 past the int32 range": {withExample(quotedField(prim(goTypeInt8), prim(goTypeInt8)), "3000000000", nil), nil},
		"uint64 negative":           {withExample(quotedField(prim(goTypeUint64), prim(goTypeUint64)), "-1", nil), nil},
		"bool kept":                 {withExample(quotedField(prim(goTypeBool), prim(goTypeBool)), "true", nil), "true"},
		"bool canonical":            {withExample(quotedField(prim(goTypeBool), prim(goTypeBool)), "t", nil), "true"},
		"bool not a bool":           {withExample(quotedField(prim(goTypeBool), prim(goTypeBool)), "yes", nil), nil},
		"float64 canonical":         {withExample(quotedField(prim(goTypeFloat64), prim(goTypeFloat64)), "2.50", nil), "2.5"},
		"time.Duration as text":     {withExample(quotedField(named(goTypeTimeDuration), named(goTypeTimeDuration)), "5s", nil), nil},
		"*Cents kept":               {withExample(quotedField(ptrOf(named(jsCents)), ptrOf(i64)), "7", nil), "7"},
	}
	for name, tc := range cases {
		assert.Equal(t, tc.want, gen.fieldInfoToProperty(tc.f).Example, name)
	}

	// A field the option does not retype keeps the unquoted coercion: a string
	// keeps any text, a named pointer type drops text its integer cannot hold.
	assert.Equal(t, "abc", gen.fieldInfoToProperty(withExample(quotedField(prim(goTypeString), prim(goTypeString)), "abc", nil)).Example)
	assert.Nil(t, gen.fieldInfoToProperty(withExample(quotedField(named(jsPI), ptrOf(i64)), "abc", nil)).Example)
	assert.Equal(t, int64(5), gen.fieldInfoToProperty(withExample(quotedField(named(jsPI), ptrOf(i64)), "5", nil)).Example)
}
