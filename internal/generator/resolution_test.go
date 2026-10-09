package generator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// resolved builds a body field whose syntactic Shape is shape and whose
// Resolution is res.
func resolved(shape, res models.TypeShape) *models.FieldInfo {
	return withResolution(&models.FieldInfo{Name: "F", JSONName: "f", Shape: shape}, res)
}

// TestSetTypeAndFormatResolutionLeaves pins the four Resolution-only leaves
// at depth: a ref, a kind-only scalar, and the {} of a Marshaler or recursive
// leaf (never descending into a Marshaler's Elem).
func TestSetTypeAndFormatResolutionLeaves(t *testing.T) {
	ref := &OpenAPIProperty{Ref: refPath("User")}
	arr := func(items *OpenAPIProperty) *OpenAPIProperty { return &OpenAPIProperty{Type: typeArray, Items: items} }
	obj := func(v *OpenAPIProperty) *OpenAPIProperty {
		return &OpenAPIProperty{Type: typeObject, AdditionalProperties: v}
	}
	cases := []struct {
		name  string
		shape models.TypeShape
		want  *OpenAPIProperty
	}{
		{"[][]$User", sliceOf(sliceOf(refTo("User"))), arr(arr(ref))},
		{"map[string][][]$User", mapOf(prim("string"), sliceOf(sliceOf(refTo("User")))), obj(arr(arr(ref)))},
		{"[]map[string]$User", sliceOf(mapOf(prim("string"), refTo("User"))), arr(obj(ref))},
		{"map[string]kind:integer", mapOf(prim("string"), kindOnlyLeaf(typeInteger)), obj(&OpenAPIProperty{Type: typeInteger})},
		{"[][]kind:number", sliceOf(sliceOf(kindOnlyLeaf(typeNumber))), arr(arr(&OpenAPIProperty{Type: typeNumber}))},
		{"marshaler over $User", marshalerOf("Users", sliceOf(refTo("User"))), &OpenAPIProperty{}},
		{"recursive", recursiveLeaf("Tree"), &OpenAPIProperty{}},
		{"text", textLeaf("Level"), &OpenAPIProperty{Type: typeString}},
		{"[]text", sliceOf(textLeaf("Level")), arr(&OpenAPIProperty{Type: typeString})},
		{"**int64", ptrOf(ptrOf(prim("int64"))), &OpenAPIProperty{Type: typeInteger, Format: formatInt64}},
	}
	for _, tc := range cases {
		prop := &OpenAPIProperty{}
		setTypeAndFormat(prop, tc.shape)
		assert.Equal(t, tc.want, prop, tc.name)
	}
}

// TestBuildFieldPropertyReadsResolution pins that the generator types a field
// from its Resolution, and from its Shape only when it has none.
func TestBuildFieldPropertyReadsResolution(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	assert.Equal(t, &OpenAPIProperty{Type: typeInteger, Format: formatInt64, Nullable: true},
		gen.fieldInfoToProperty(resolved(named("PC"), ptrOf(prim("int64")))), "type PC *int64")
	assert.Equal(t, &OpenAPIProperty{Type: typeObject, AllOf: []*OpenAPIProperty{{Ref: refPath("Address")}}, Nullable: true},
		gen.fieldInfoToProperty(resolved(named("PAddr"), ptrOf(refTo("Address")))), "type PAddr *Address")
	assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{Type: typeString}},
		gen.fieldInfoToProperty(resolved(ptrOf(named("Tags")), ptrOf(sliceOf(prim("string"))))), "*Tags: no nullable")
	assert.Equal(t, &OpenAPIProperty{Type: typeString, Nullable: true},
		gen.fieldInfoToProperty(&models.FieldInfo{Shape: ptrOf(prim("string"))}), "a Shape-only field behaves as before")
}

// TestMarshalerLeafTakesNoKeywords pins that no validate keyword lands on a
// Marshaler leaf (format, enum and pattern ignore kind), while its example
// is kept and other leaves keep today's handling.
func TestMarshalerLeafTakesNoKeywords(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	status := marshalerOf("Status", prim("int"))
	for _, c := range []map[string]string{
		{"min": "1", "max": "3"}, {"oneof": "1 2"}, {"email": ""}, {"e164": ""}, {validatorRegexp: "^a$"},
	} {
		f := resolved(named("Status"), status)
		f.Constraints = c
		assert.Equal(t, &OpenAPIProperty{}, gen.fieldInfoToProperty(f), "%v", c)
	}

	f := resolved(named("Status"), status)
	f.Example = "active"
	assert.Equal(t, &OpenAPIProperty{Example: "active"}, gen.fieldInfoToProperty(f))

	assert.Equal(t, &OpenAPIProperty{}, gen.fieldInfoToProperty(resolved(ptrOf(named("Status")), ptrOf(status))), "*Status: no nullable")

	f = resolved(sliceOf(named("Status")), sliceOf(status))
	f.Constraints = map[string]string{"min": "1"}
	f.ElementConstraints = map[string]string{"oneof": "1 2"}
	assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: &OpenAPIProperty{}, MinItems: intPtr(1)}, gen.fieldInfoToProperty(f))

	f = resolved(named("json.RawMessage"), named("json.RawMessage"))
	f.Constraints = map[string]string{"oneof": "a b"}
	assert.NotEmpty(t, gen.fieldInfoToProperty(f).Enum, "a non-Marshaler leaf keeps its keywords")
}

// TestAddFieldSchemaRefsWalksResolution pins the component scan: refs at any
// depth are collected, refs under a Marshaler leaf are not, and a field with
// only a Shape adds nothing.
func TestAddFieldSchemaRefsWalksResolution(t *testing.T) {
	ti := &models.TypeInfo{Name: "Rows", Fields: []models.FieldInfo{
		*resolved(named("A"), mapOf(prim("string"), sliceOf(ptrOf(refTo("Deep"))))),
		*resolved(named("B"), arrayOf(refTo("Arr"))),
		*resolved(named("C"), marshalerOf("C", sliceOf(refTo("Hidden")))),
		*resolved(named("D"), recursiveLeaf("D")),
		*resolved(named("T"), sliceOf(textLeaf("T"))),
		{Name: "E", JSONName: "-", Shape: named("Excluded")},
	}}
	out := map[string]bool{}
	addFieldSchemaRefs(out, map[string]*models.TypeInfo{"Rows": ti, "nil": nil})
	assert.Equal(t, map[string]bool{"Deep": true, "Arr": true}, out)
}

// TestMapOfStructTakesCardinality pins R17: a struct-valued map takes
// minProperties like any other map.
func TestMapOfStructTakesCardinality(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	f := resolved(mapOf(prim("string"), named("User")), mapOf(prim("string"), refTo("User")))
	f.Constraints = map[string]string{"min": "1"}
	assert.Equal(t, &OpenAPIProperty{
		Type:                 typeObject,
		AdditionalProperties: &OpenAPIProperty{Ref: refPath("User")},
		MinProperties:        intPtr(1),
	}, gen.fieldInfoToProperty(f))
}

// positionShapes wraps leaf in the seven field positions the warned rows are
// pinned in.
func positionShapes(leaf models.TypeShape) map[string]models.TypeShape {
	str := prim("string")
	return map[string]models.TypeShape{
		"%s":              leaf,
		"*%s":             ptrOf(leaf),
		"[]%s":            sliceOf(leaf),
		"map[string]%s":   mapOf(str, leaf),
		"[][]%s":          sliceOf(sliceOf(leaf)),
		"map[string][]%s": mapOf(str, sliceOf(leaf)),
		"[]map[string]%s": sliceOf(mapOf(str, leaf)),
	}
}

// TestFallbackLeavesInEveryPosition pins the emitted side of the analyzer's
// TestWarnedRowsInEveryPosition: each fallback leaf emits its schema L in
// every position, wrapped in the position's containers.
func TestFallbackLeavesInEveryPosition(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	leaves := []struct {
		leaf  models.TypeShape
		typed bool // L is {type: object}; else L is {}
	}{
		{marshalerOf("Status", prim("int")), false},
		{recursiveLeaf("Tree"), false},
		{named("decimal.Decimal"), true},
		{prim("complex128"), true},
		{prim(goTypeUintptr), true},
	}
	for _, l := range leaves {
		leafProp := func() *OpenAPIProperty {
			if l.typed {
				return &OpenAPIProperty{Type: typeObject}
			}
			return &OpenAPIProperty{}
		}
		arr := func(items *OpenAPIProperty) *OpenAPIProperty { return &OpenAPIProperty{Type: typeArray, Items: items} }
		obj := func(v *OpenAPIProperty) *OpenAPIProperty {
			return &OpenAPIProperty{Type: typeObject, AdditionalProperties: v}
		}
		ptr := leafProp()
		if l.typed {
			ptr.Nullable = true
		}
		want := map[string]*OpenAPIProperty{
			"%s":              leafProp(),
			"*%s":             ptr,
			"[]%s":            arr(leafProp()),
			"map[string]%s":   obj(leafProp()),
			"[][]%s":          arr(arr(leafProp())),
			"map[string][]%s": obj(arr(leafProp())),
			"[]map[string]%s": arr(obj(leafProp())),
		}
		res := positionShapes(l.leaf)
		for pos, shape := range positionShapes(named("X")) {
			got := gen.fieldInfoToProperty(resolved(shape, res[pos]))
			assert.Equal(t, want[pos], got, "%s in %s", l.leaf.Name, fmt.Sprintf(pos, "X"))
		}
	}
}

// TestMultiPointerMatchesSinglePointer pins #120's field rows: a field whose
// Resolution differs from another's only in pointer depth emits the same
// schema, bounds, example and param form included.
func TestMultiPointerMatchesSinglePointer(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	str := prim("string")
	targets := map[string]models.TypeShape{
		"int64":                prim("int64"),
		"string":               str,
		"time.Time":            named("time.Time"),
		"[]byte":               sliceOf(prim("byte")),
		"[]int64":              sliceOf(prim("int64")),
		"map[string]int64":     mapOf(str, prim("int64")),
		"map[string]$Address":  mapOf(str, refTo("Address")),
		"[]$Address":           sliceOf(refTo("Address")),
		"$Address":             refTo("Address"),
		"[]**$Address":         sliceOf(ptrOf(ptrOf(refTo("Address")))),
		"map[string]**$Addr":   mapOf(str, ptrOf(ptrOf(refTo("Address")))),
		"[]**int64 (dive)":     sliceOf(ptrOf(ptrOf(prim("int64")))),
		"map[string]**int64":   mapOf(str, ptrOf(ptrOf(prim("int64")))),
		"int64 (bounds, ex)":   prim("int64"),
		"int64 (query param)":  prim("int64"),
		"[]$Address (min=1)":   sliceOf(refTo("Address")),
		"kind:integer (min=1)": kindOnlyLeaf(typeInteger),
	}
	tag := func(name string, f *models.FieldInfo) *models.FieldInfo {
		switch name {
		case "int64 (bounds, ex)":
			f.Constraints = map[string]string{"min": "1", "max": "9"}
			f.Example = "3"
		case "int64 (query param)":
			f.ParamType, f.ParamName = "query", "f"
		case "[]**int64 (dive)":
			f.ElementConstraints = map[string]string{"min": "1"}
		case "[]$Address (min=1)", "kind:integer (min=1)":
			f.Constraints = map[string]string{"min": "1"}
		}
		return f
	}
	for name, target := range targets {
		one := gen.fieldInfoToProperty(tag(name, resolved(named("X"), ptrOf(target))))
		for _, depth := range []int{2, 3} {
			res := target
			for range depth {
				res = ptrOf(res)
			}
			got := gen.fieldInfoToProperty(tag(name, resolved(named("X"), res)))
			assert.Equal(t, one, got, "%s at pointer depth %d", name, depth)
		}
	}

	// #120's "correct today" rows equal origin/main's output for the same
	// fields (pinned from the 514e417 binary).
	addrRef := &OpenAPIProperty{Ref: refPath("Address")}
	assert.Equal(t, &OpenAPIProperty{Type: typeObject, AllOf: []*OpenAPIProperty{addrRef}, Nullable: true},
		gen.fieldInfoToProperty(resolved(named("X"), ptrOf(ptrOf(refTo("Address"))))), "**Address")
	assert.Equal(t, &OpenAPIProperty{Type: typeInteger, Format: formatInt64, Nullable: true},
		gen.fieldInfoToProperty(resolved(named("X"), ptrOf(ptrOf(prim("int64"))))), "**Cents")
	assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: addrRef},
		gen.fieldInfoToProperty(resolved(named("X"), sliceOf(ptrOf(ptrOf(refTo("Address")))))), "[]**Address")
	assert.Equal(t, &OpenAPIProperty{Type: typeObject, AdditionalProperties: addrRef},
		gen.fieldInfoToProperty(resolved(named("X"), mapOf(str, ptrOf(ptrOf(refTo("Address")))))), "map[string]**Address")
	q := tag("int64 (query param)", resolved(named("X"), ptrOf(ptrOf(prim("int64")))))
	assert.Equal(t, &OpenAPIProperty{Type: typeInteger, Format: formatInt64}, gen.fieldInfoToProperty(q), "query **int64")
}

// TestUntypedBuiltinsFallToObject pins parity with models.UntypedBuiltinNames:
// each of them, and uintptr, falls to setBasicTypeAndFormat's object default,
// and no other builtin does.
func TestUntypedBuiltinsFallToObject(t *testing.T) {
	names := make([]string, 0, 1+len(models.UntypedBuiltinNames))
	names = append(names, goTypeUintptr)
	for n := range models.UntypedBuiltinNames {
		names = append(names, n)
	}
	for _, n := range names {
		prop := &OpenAPIProperty{}
		setBasicTypeAndFormat(prop, n)
		assert.Equal(t, &OpenAPIProperty{Type: typeObject}, prop, n)
	}
	for _, n := range []string{"bool", "string", "byte", "rune", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "any", "interface{}"} {
		prop := &OpenAPIProperty{}
		setBasicTypeAndFormat(prop, n)
		assert.NotEqual(t, typeObject, prop.Type, n)
	}
	require.NotEmpty(t, models.UntypedBuiltinNames)
}
