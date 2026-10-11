package generator

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Issue 118: the generator documents an anonymous struct inline.

// anonOf is an extracted anonymous struct holding fields, as the analyzer's
// resolver stamps it: its Fields are never nil, even with no fields.
func anonOf(fields ...models.FieldInfo) models.TypeShape {
	if fields == nil {
		fields = []models.FieldInfo{}
	}
	return models.TypeShape{Kind: models.ShapeStruct, Struct: &models.AnonymousStruct{Fields: fields}}
}

// unextracted is an anonymous struct the analyzer did not extract (a
// parameter's): its Fields are nil.
func unextracted() models.TypeShape {
	return models.TypeShape{Kind: models.ShapeStruct, Struct: &models.AnonymousStruct{}}
}

// bodyField is a body field named name with the JSON name json, typed by r.
func bodyField(name, json string, r models.TypeShape) models.FieldInfo {
	return *withResolution(&models.FieldInfo{Name: name, JSONName: json, Shape: r}, r)
}

// xObject is the inline object of struct{ X int `json:"x" validate:"required"` }.
func xObject() *OpenAPIProperty {
	return &OpenAPIProperty{
		Type:       typeObject,
		Properties: map[string]*OpenAPIProperty{"x": {Type: typeInteger, Format: formatInt64}},
		Required:   []string{"x"},
	}
}

// requiredX is the field X int `json:"x" validate:"required"`.
func requiredX() models.FieldInfo {
	f := bodyField("X", "x", prim(goTypeInt))
	f.Required = true
	return f
}

// TestInlineObjectSchema pins an anonymous struct's inline object in every
// position: direct, behind a pointer (nullable), as slice items, as a map
// value, nested, and empty.
func TestInlineObjectSchema(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	x := anonOf(requiredX())
	nullableX := xObject()
	nullableX.Nullable = true
	nested := &OpenAPIProperty{Type: typeObject, Properties: map[string]*OpenAPIProperty{"in": xObject()}, Required: []string{}}
	cases := []struct {
		name string
		r    models.TypeShape
		want *OpenAPIProperty
	}{
		{"direct", x, xObject()},
		{"pointer", ptrOf(x), nullableX},
		{"slice", sliceOf(x), &OpenAPIProperty{Type: typeArray, Items: xObject()}},
		{"array", arrayOf(ptrOf(x)), &OpenAPIProperty{Type: typeArray, Items: xObject()}},
		{"map", mapOf(prim(goTypeString), x), &OpenAPIProperty{Type: typeObject, AdditionalProperties: xObject()}},
		{"nested", anonOf(bodyField("In", "in", x)), nested},
		{"empty", anonOf(), &OpenAPIProperty{Type: typeObject}},
		{"unextracted", unextracted(), &OpenAPIProperty{Type: typeObject}},
	}
	for _, tc := range cases {
		f := bodyField("Meta", "meta", tc.r)
		assert.Equal(t, tc.want, gen.fieldInfoToProperty(&f), tc.name)
	}

	empty := bodyField("E", "e", anonOf())
	out, err := yaml.Marshal(gen.fieldInfoToProperty(&empty))
	require.NoError(t, err)
	assert.Equal(t, "type: object\n", string(out), "an empty inline object has no properties or required key")
}

// TestSetInlineObjectNilStruct pins the guard on a ShapeStruct carrying no
// AnonymousStruct.
func TestSetInlineObjectNilStruct(t *testing.T) {
	prop := &OpenAPIProperty{}
	setTypeAndFormat(prop, models.TypeShape{Kind: models.ShapeStruct})
	assert.Equal(t, &OpenAPIProperty{Type: typeObject}, prop)
}

// TestInlineObjectMatchesComponent pins that an inline object and a
// component of the same fields have the same properties and required list,
// the parameter, json:"-" and nameless skips included.
func TestInlineObjectMatchesComponent(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	param := bodyField("ID", "", prim(goTypeInt64))
	param.ParamType, param.ParamName = "path", "id"
	skip := bodyField("Skip", "-", prim(goTypeInt))
	fields := []models.FieldInfo{
		requiredX(), param, skip, bodyField("", "", prim(goTypeString)), bodyField("Untagged", "", prim(goTypeBool)),
		bodyField("Office", "office", refTo("Office")),
	}
	component := gen.typeInfoToSchema(&models.TypeInfo{Name: "C", Fields: fields})
	f := bodyField("Meta", "meta", anonOf(fields...))
	inline := gen.fieldInfoToProperty(&f)
	assert.Equal(t, component.Properties, inline.Properties)
	assert.Equal(t, component.Required, inline.Required)
	assert.Equal(t, []string{"office", "untagged", "x"}, slices.Sorted(maps.Keys(inline.Properties)))
}

// TestInlineObjectTakesNoValidateKeyword pins that, like a $ref, an inline
// object takes no validate keyword, while an enclosing slice keeps its
// cardinality and the inner fields keep theirs.
func TestInlineObjectTakesNoValidateKeyword(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	inner := bodyField("Mail", "mail", prim(goTypeString))
	inner.Constraints = map[string]string{"email": ""}
	x := anonOf(inner)

	direct := bodyField("Meta", "meta", x)
	direct.Constraints = map[string]string{"required": "true", "email": ""}
	prop := gen.fieldInfoToProperty(&direct)
	assert.Empty(t, prop.Format)
	assert.Equal(t, formatEmail, prop.Properties["mail"].Format, "the inner field keeps its own keywords")

	dive := bodyField("List", "list", sliceOf(x))
	dive.ElementConstraints = map[string]string{"email": ""}
	assert.Empty(t, gen.fieldInfoToProperty(&dive).Items.Format)

	card := bodyField("List", "list", sliceOf(x))
	card.Constraints = map[string]string{"min": "1"}
	assert.Equal(t, intPtr(1), gen.fieldInfoToProperty(&card).MinItems)
}

// TestUnextractedAnonymousStructKeepsKeywords pins that only an extracted
// anonymous struct's inline object refuses validate keywords. An unextracted
// one (a path, query or header parameter's) is a bare object that keeps the
// kind-agnostic keywords it took before anonymous structs were extracted, on
// itself and as slice items, while an extracted empty one takes none.
func TestUnextractedAnonymousStructKeepsKeywords(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	query := func(r models.TypeShape, rules, elemRules map[string]string) *OpenAPIProperty {
		f := bodyField("Q", "", r)
		f.ParamType, f.ParamName = "query", "q"
		f.Constraints, f.ElementConstraints = rules, elemRules
		return gen.fieldInfoToProperty(&f)
	}
	oneOf := map[string]string{"oneof": "a b"}
	email := map[string]string{"email": ""}

	assert.Equal(t, &OpenAPIProperty{Type: typeObject, Enum: []any{"a", "b"}}, query(unextracted(), oneOf, nil))
	assert.Equal(t, &OpenAPIProperty{Type: typeObject, Format: formatEmail}, query(ptrOf(unextracted()), email, nil))
	assert.Equal(t, []any{"a", "b"}, query(sliceOf(unextracted()), nil, oneOf).Items.Enum)

	assert.Equal(t, &OpenAPIProperty{Type: typeObject}, query(anonOf(), oneOf, nil), "an extracted literal takes none, even with no fields")
	assert.Empty(t, query(sliceOf(anonOf()), nil, email).Items.Format)

	assert.True(t, takesNoKeyword(anonOf()))
	assert.True(t, takesNoKeyword(ptrOf(anonOf(requiredX()))))
	assert.False(t, takesNoKeyword(unextracted()))
	assert.False(t, takesNoKeyword(models.TypeShape{Kind: models.ShapeStruct}))
}

// TestReferencedSchemaNamesDescendsInlineFields pins that a component
// reachable only through an anonymous struct's field is referenced, at any
// depth, while a Marshaler leaf's Elem is still not descended.
func TestReferencedSchemaNamesDescendsInlineFields(t *testing.T) {
	types := map[string]*models.TypeInfo{
		"Holder": {Name: "Holder", Fields: []models.FieldInfo{
			bodyField("In", "in", sliceOf(anonOf(
				bodyField("A", "a", refTo("Ack")),
				bodyField("D", "d", anonOf(bodyField("Deep", "deep", mapOf(prim(goTypeString), ptrOf(refTo("Deep")))))),
				bodyField("M", "m", marshalerOf("M", refTo("Hidden"))),
			))),
			bodyField("U", "u", models.TypeShape{Kind: models.ShapeStruct}),
		}},
	}
	got := referencedSchemaNames(nil, types)
	assert.True(t, got["Ack"])
	assert.True(t, got["Deep"])
	assert.False(t, got["Hidden"])
}

// TestBareFieldPropertyStampsNoExample pins the split the inline object
// path keeps: bareFieldProperty types a field without its example, and
// fieldInfoToProperty (fieldProperty) applies it, once, to the resolved type,
// inside an inline object as at the top level.
func TestBareFieldPropertyStampsNoExample(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	n := bodyField("N", "n", prim(goTypeInt))
	n.Example = "5"
	assert.Nil(t, bareFieldProperty(&n).Example)
	assert.Equal(t, int64(5), gen.fieldInfoToProperty(&n).Example)

	holder := bodyField("Meta", "meta", anonOf(n))
	assert.Equal(t, int64(5), gen.fieldInfoToProperty(&holder).Properties["n"].Example)
}
