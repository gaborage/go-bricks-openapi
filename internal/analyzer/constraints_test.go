package analyzer

import (
	"testing"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// Test-only TypeShape builders, mirroring what the analyzer's decoder stamps.
func prim(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePrimitive, Name: name}
}
func named(name string) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeNamed, Name: name}
}
func unknownShape() models.TypeShape { return models.TypeShape{Kind: models.ShapeUnknown} }
func ptrOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapePointer, Elem: &s}
}
func sliceOf(s models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeSlice, Elem: &s}
}
func mapOf(k, v models.TypeShape) models.TypeShape {
	return models.TypeShape{Kind: models.ShapeMap, Key: &k, Elem: &v}
}

func TestMapConstraintToOpenAPI(t *testing.T) {
	tests := []struct {
		name           string
		shape          models.TypeShape
		underlyingKind string
		constraints    map[string]string
		expected       []OpenAPIConstraint
		description    string
	}{
		{
			name:        "email format",
			shape:       prim("string"),
			constraints: map[string]string{"email": "true"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "email"}},
			description: "should map email to format constraint",
		},
		{
			name:        "url format",
			shape:       prim("string"),
			constraints: map[string]string{"url": "true"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "uri"}},
			description: "should map url to uri format",
		},
		{
			name:        "uuid format",
			shape:       prim("string"),
			constraints: map[string]string{"uuid": "true"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "uuid"}},
			description: "should map uuid to format constraint",
		},
		{
			name:        "date format",
			shape:       prim("string"),
			constraints: map[string]string{"date": "true"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "date"}},
			description: "should map date to format constraint",
		},
		{
			name:        "datetime format",
			shape:       prim("string"),
			constraints: map[string]string{validatorDatetime: "true"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "date-time"}},
			description: "should map datetime to date-time format",
		},
		{
			name:        "string min length",
			shape:       prim("string"),
			constraints: map[string]string{"min": "5"},
			expected:    []OpenAPIConstraint{{Name: "minLength", Value: 5}},
			description: "should map min to minLength for strings",
		},
		{
			name:        "string max length",
			shape:       prim("string"),
			constraints: map[string]string{"max": "100"},
			expected:    []OpenAPIConstraint{{Name: "maxLength", Value: 100}},
			description: "should map max to maxLength for strings",
		},
		{
			name:        "string exact length",
			shape:       prim("string"),
			constraints: map[string]string{"len": "10"},
			expected: []OpenAPIConstraint{
				{Name: "minLength", Value: 10},
				{Name: "maxLength", Value: 10},
			},
			description: "should map len to both minLength and maxLength",
		},
		{
			name:        "integer minimum",
			shape:       prim("int"),
			constraints: map[string]string{"min": "18"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(18)}},
			description: "should map min to minimum for integers",
		},
		{
			name:        "integer maximum",
			shape:       prim("int"),
			constraints: map[string]string{"max": "120"},
			expected:    []OpenAPIConstraint{{Name: "maximum", Value: int64(120)}},
			description: "should map max to maximum for integers",
		},
		{
			name:        "int64 minimum",
			shape:       prim("int64"),
			constraints: map[string]string{"min": "1000"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(1000)}},
			description: "should handle int64 type",
		},
		{
			name:        "float minimum",
			shape:       prim("float64"),
			constraints: map[string]string{"min": "0.5"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: 0.5}},
			description: "should map min to minimum for floats",
		},
		{
			name:        "float maximum",
			shape:       prim("float64"),
			constraints: map[string]string{"max": "99.9"},
			expected:    []OpenAPIConstraint{{Name: "maximum", Value: 99.9}},
			description: "should map max to maximum for floats",
		},
		{
			name:        "greater than (exclusive minimum)",
			shape:       prim("int"),
			constraints: map[string]string{"gt": "0"},
			expected: []OpenAPIConstraint{
				{Name: "minimum", Value: int64(0)},
				{Name: constraintExclusiveMinimum, Value: true},
			},
			description: "should map gt to exclusive minimum",
		},
		{
			name:        "greater than or equal",
			shape:       prim("int"),
			constraints: map[string]string{"gte": "1"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(1)}},
			description: "should map gte to minimum",
		},
		{
			name:        "less than (exclusive maximum)",
			shape:       prim("int"),
			constraints: map[string]string{"lt": "100"},
			expected: []OpenAPIConstraint{
				{Name: "maximum", Value: int64(100)},
				{Name: "exclusiveMaximum", Value: true},
			},
			description: "should map lt to exclusive maximum",
		},
		{
			name:        "less than or equal",
			shape:       prim("int"),
			constraints: map[string]string{"lte": "99"},
			expected:    []OpenAPIConstraint{{Name: "maximum", Value: int64(99)}},
			description: "should map lte to maximum",
		},
		{
			name:        "oneof enum",
			shape:       prim("string"),
			constraints: map[string]string{"oneof": "red green blue"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{"red", "green", "blue"}}},
			description: "should map oneof to enum array",
		},
		{
			name:        "oneof numeric enum int",
			shape:       prim("int"),
			constraints: map[string]string{"oneof": "1 2 3"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{int64(1), int64(2), int64(3)}}},
			description: "should map oneof to numeric enum for int type",
		},
		{
			name:        "oneof numeric enum float64",
			shape:       prim("float64"),
			constraints: map[string]string{"oneof": "1.5 2.5 3.5"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{1.5, 2.5, 3.5}}},
			description: "should map oneof to numeric enum for float64 type",
		},
		{
			name:        "oneof pointer numeric type",
			shape:       ptrOf(prim("int")),
			constraints: map[string]string{"oneof": "10 20 30"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{int64(10), int64(20), int64(30)}}},
			description: "should handle pointer numeric types correctly",
		},
		{
			name:        "regexp pattern",
			shape:       prim("string"),
			constraints: map[string]string{"regexp": "^[A-Z]+$"},
			expected:    []OpenAPIConstraint{{Name: "pattern", Value: "^[A-Z]+$"}},
			description: "should map regexp to pattern",
		},
		{
			name:  "required constraint skipped",
			shape: prim("string"),
			constraints: map[string]string{
				"required": "true",
				"email":    "true",
			},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "email"}},
			description: "should skip required constraint (handled at schema level)",
		},
		{
			name:  "multiple string constraints",
			shape: prim("string"),
			constraints: map[string]string{
				"required": "true",
				"email":    "true",
				"min":      "5",
				"max":      "100",
			},
			expected: []OpenAPIConstraint{
				{Name: "format", Value: "email"},
				{Name: "minLength", Value: 5},
				{Name: "maxLength", Value: 100},
			},
			description: "should map multiple string constraints",
		},
		{
			name:  "multiple integer constraints",
			shape: prim("int"),
			constraints: map[string]string{
				"required": "true",
				"min":      "1",
				"max":      "1000",
			},
			expected: []OpenAPIConstraint{
				{Name: "minimum", Value: int64(1)},
				{Name: "maximum", Value: int64(1000)},
			},
			description: "should map multiple integer constraints",
		},
		{
			name:        "pointer type stripped",
			shape:       ptrOf(prim("string")),
			constraints: map[string]string{"min": "5"},
			expected:    []OpenAPIConstraint{{Name: "minLength", Value: 5}},
			description: "should strip pointer prefix from type",
		},
		{
			name:        "empty constraints",
			shape:       prim("string"),
			constraints: map[string]string{},
			expected:    []OpenAPIConstraint{},
			description: "should return empty array for no constraints",
		},
		// --- PR11: named-numeric via UnderlyingKind ---
		{
			name: "named integer min/max via UnderlyingKind", shape: named("Cents"), underlyingKind: kindInteger,
			constraints: map[string]string{"min": "100", "max": "1000"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(100)}, {Name: "maximum", Value: int64(1000)}},
			description: "type Cents int64 must map numeric constraints (was dropped)",
		},
		{
			name: "time.Duration gte via UnderlyingKind", shape: named("time.Duration"), underlyingKind: kindInteger,
			constraints: map[string]string{"gte": "1"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(1)}},
			description: "time.Duration maps numeric constraints",
		},
		{
			name: "named integer gt via UnderlyingKind", shape: named("Cents"), underlyingKind: kindInteger,
			constraints: map[string]string{"gt": "0"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(0)}, {Name: constraintExclusiveMinimum, Value: true}},
			description: "gt on named numeric emits minimum + exclusiveMinimum",
		},
		{
			name: "named integer oneof via UnderlyingKind", shape: named("Status"), underlyingKind: kindInteger,
			constraints: map[string]string{"oneof": "1 2 3"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{int64(1), int64(2), int64(3)}}},
			description: "oneof on named numeric yields numeric enum",
		},
		// --- PR11: string length comparisons ---
		{
			name: "string gt -> minLength+1", shape: prim("string"),
			constraints: map[string]string{"gt": "3"},
			expected:    []OpenAPIConstraint{{Name: "minLength", Value: 4}},
			description: "gt on string constrains length",
		},
		{
			name: "string gt=0 non-empty idiom", shape: prim("string"),
			constraints: map[string]string{"gt": "0"},
			expected:    []OpenAPIConstraint{{Name: "minLength", Value: 1}},
			description: "gt=0 -> minLength 1",
		},
		{
			name: "string lt -> maxLength-1", shape: prim("string"),
			constraints: map[string]string{"lt": "10"},
			expected:    []OpenAPIConstraint{{Name: "maxLength", Value: 9}},
			description: "lt on string constrains length",
		},
		{
			name: "string gt negative clamps to 0", shape: prim("string"),
			constraints: map[string]string{"gt": "-5"},
			expected:    []OpenAPIConstraint{{Name: "minLength", Value: 0}},
			description: "negative length clamps to non-negative",
		},
		// --- PR11: slice cardinality ---
		{
			name: "slice min -> minItems", shape: sliceOf(prim("string")),
			constraints: map[string]string{"min": "1"},
			expected:    []OpenAPIConstraint{{Name: "minItems", Value: 1}},
			description: "min on []T maps to minItems",
		},
		{
			name: "slice len -> minItems+maxItems", shape: sliceOf(prim("int")),
			constraints: map[string]string{"len": "3"},
			expected:    []OpenAPIConstraint{{Name: "minItems", Value: 3}, {Name: "maxItems", Value: 3}},
			description: "len on []T maps to minItems == maxItems",
		},
		{
			name: "pointer slice min -> minItems", shape: ptrOf(sliceOf(prim("string"))),
			constraints: map[string]string{"min": "2"},
			expected:    []OpenAPIConstraint{{Name: "minItems", Value: 2}},
			description: "pointer-to-slice stripped, cardinality applies",
		},
		{
			name: "byte slice is not an array", shape: sliceOf(prim("byte")),
			constraints: map[string]string{"min": "10"},
			expected:    []OpenAPIConstraint{},
			description: "[]byte is a base64 string, not a cardinality-bearing array",
		},
		// --- PR11: oneof quoting, eq, ne, datetime, formats, patterns ---
		{
			name: "oneof quoted multi-word", shape: prim("string"),
			constraints: map[string]string{"oneof": "'New York' 'Los Angeles'"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{"New York", "Los Angeles"}}},
			description: "quote-aware oneof keeps spaces",
		},
		{
			name: "oneof unquoted still splits", shape: prim("string"),
			constraints: map[string]string{"oneof": "active pending"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{"active", "pending"}}},
			description: "unquoted oneof splits on spaces",
		},
		{
			name: "eq -> single-element enum", shape: prim("string"),
			constraints: map[string]string{"eq": "GOLD"},
			expected:    []OpenAPIConstraint{{Name: "enum", Value: []any{"GOLD"}}},
			description: "eq maps to a single-value enum",
		},
		{
			name: "ne -> nothing", shape: prim("string"),
			constraints: map[string]string{"ne": "X"},
			expected:    []OpenAPIConstraint{},
			description: "ne has no clean OpenAPI representation",
		},
		{
			name: "datetime date-only layout -> date", shape: prim("string"),
			constraints: map[string]string{validatorDatetime: "2006-01-02"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "date"}},
			description: "date-only layout maps to format date",
		},
		{
			name: "datetime with clock -> date-time", shape: prim("string"),
			constraints: map[string]string{validatorDatetime: "2006-01-02T15:04:05Z07:00"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "date-time"}},
			description: "layout with clock tokens maps to date-time",
		},
		{
			name: "ipv4 format", shape: prim("string"),
			constraints: map[string]string{formatIPv4: "true"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: formatIPv4}},
			description: "ipv4 maps to format ipv4",
		},
		{
			name: "base64 -> byte format", shape: prim("string"),
			constraints: map[string]string{"base64": "true"},
			expected:    []OpenAPIConstraint{{Name: "format", Value: "byte"}},
			description: "base64 maps to OpenAPI byte format",
		},
		{
			name: "alpha -> anchored pattern", shape: prim("string"),
			constraints: map[string]string{"alpha": "true"},
			expected:    []OpenAPIConstraint{{Name: "pattern", Value: `^[a-zA-Z]+$`}},
			description: "alpha maps to a letter-only pattern",
		},
		{
			name: "startswith -> anchored escaped pattern", shape: prim("string"),
			constraints: map[string]string{"startswith": "a.b"},
			expected:    []OpenAPIConstraint{{Name: "pattern", Value: `^a\.b`}},
			description: "startswith escapes metacharacters and anchors at start",
		},
		{
			name: "bare ip -> no format/pattern (documented)", shape: prim("string"),
			constraints: map[string]string{"ip": "true"},
			expected:    []OpenAPIConstraint{},
			description: "bare ip has no single clean OpenAPI format; left unconstrained by design",
		},
		{
			name: "contains -> unanchored escaped pattern", shape: prim("string"),
			constraints: map[string]string{"contains": "a.b"},
			expected:    []OpenAPIConstraint{{Name: "pattern", Value: `a\.b`}},
			description: "contains escapes metacharacters, no anchors",
		},
		{
			name: "endswith -> end-anchored escaped pattern", shape: prim("string"),
			constraints: map[string]string{"endswith": ".json"},
			expected:    []OpenAPIConstraint{{Name: "pattern", Value: `\.json$`}},
			description: "endswith escapes and anchors at end",
		},
		{
			name: "string gte -> minLength", shape: prim("string"),
			constraints: map[string]string{"gte": "3"},
			expected:    []OpenAPIConstraint{{Name: "minLength", Value: 3}},
			description: "gte on string is a length floor",
		},
		{
			name: "string lte -> maxLength", shape: prim("string"),
			constraints: map[string]string{"lte": "10"},
			expected:    []OpenAPIConstraint{{Name: "maxLength", Value: 10}},
			description: "lte on string is a length ceiling",
		},
		{
			name: "slice max -> maxItems", shape: sliceOf(prim("string")),
			constraints: map[string]string{"max": "5"},
			expected:    []OpenAPIConstraint{{Name: "maxItems", Value: 5}},
			description: "max on []T maps to maxItems",
		},
		// --- map cardinality (issue #3) ---
		{
			name: "map min -> minProperties", shape: mapOf(prim("string"), prim("string")),
			constraints: map[string]string{"min": "1"},
			expected:    []OpenAPIConstraint{{Name: "minProperties", Value: 1}},
			description: "min on map[string]T maps to minProperties",
		},
		{
			name: "map max -> maxProperties", shape: mapOf(prim("string"), prim("string")),
			constraints: map[string]string{"max": "10"},
			expected:    []OpenAPIConstraint{{Name: "maxProperties", Value: 10}},
			description: "max on map[string]T maps to maxProperties",
		},
		{
			name: "map len -> minProperties+maxProperties", shape: mapOf(prim("string"), prim("string")),
			constraints: map[string]string{"len": "3"},
			expected:    []OpenAPIConstraint{{Name: "minProperties", Value: 3}, {Name: "maxProperties", Value: 3}},
			description: "len on map[string]T maps to minProperties == maxProperties",
		},
		{
			name: "pointer map min -> minProperties", shape: ptrOf(mapOf(prim("string"), prim("int"))),
			constraints: map[string]string{"min": "2"},
			expected:    []OpenAPIConstraint{{Name: "minProperties", Value: 2}},
			description: "pointer-to-map stripped, cardinality applies",
		},
		{
			name: "struct-valued map cardinality", shape: mapOf(prim("string"), unknownShape()),
			constraints: map[string]string{"min": "1", "max": "5"},
			expected:    []OpenAPIConstraint{{Name: "minProperties", Value: 1}, {Name: "maxProperties", Value: 5}},
			description: "cardinality is independent of the map value type",
		},
		// --- Issue #2: most-restrictive bound when multiple rules collapse to one keyword ---
		{
			name: "numeric min+gte -> larger minimum", shape: prim("int"),
			constraints: map[string]string{"min": "1", "gte": "10"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(10)}},
			description: "validator enforces both; effective minimum is the larger (10)",
		},
		{
			name: "numeric max+lt -> smaller exclusive maximum", shape: prim("int"),
			constraints: map[string]string{"max": "100", "lt": "50"},
			expected: []OpenAPIConstraint{
				{Name: "maximum", Value: int64(50)},
				{Name: "exclusiveMaximum", Value: true},
			},
			description: "effective maximum is the smaller (50); lt is exclusive",
		},
		{
			name: "string min+gte -> larger minLength", shape: prim("string"),
			constraints: map[string]string{"min": "2", "gte": "5"},
			expected:    []OpenAPIConstraint{{Name: "minLength", Value: 5}},
			description: "effective minLength is the larger (5)",
		},
		{
			name: "string max+lte -> smaller maxLength", shape: prim("string"),
			constraints: map[string]string{"max": "20", "lte": "8"},
			expected:    []OpenAPIConstraint{{Name: "maxLength", Value: 8}},
			description: "effective maxLength is the smaller (8)",
		},
		{
			name: "numeric gt+gte equal magnitudes -> inclusive binds", shape: prim("int"),
			constraints: map[string]string{"gt": "5", "gte": "10"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(10)}},
			description: "gte=10 (>gt=5) binds; inclusive 10 wins, no exclusiveMinimum",
		},
		{
			name: "numeric gte+gt equal values -> exclusive wins", shape: prim("int"),
			constraints: map[string]string{"gte": "10", "gt": "10"},
			expected: []OpenAPIConstraint{
				{Name: "minimum", Value: int64(10)},
				{Name: "exclusiveMinimum", Value: true},
			},
			description: "equal value, exclusive (gt) is more restrictive than inclusive (gte)",
		},
		{
			name: "slice min+len -> larger minItems plus maxItems", shape: sliceOf(prim("string")),
			constraints: map[string]string{"min": "1", "len": "3"},
			expected: []OpenAPIConstraint{
				{Name: "minItems", Value: 3},
				{Name: "maxItems", Value: 3},
			},
			description: "min=1 and len=3 both touch minItems; max(1,3)=3, maxItems=3 from len",
		},
		{
			// 9007199254740992 = 2^53, 9007199254740993 = 2^53+1. Both are exact
			// int64 but collapse to the same float64, so a float-based compare would
			// wrongly treat them as equal and keep the smaller (first-seen) bound.
			name: "int64 minimum overlap above 2^53 keeps the exact larger bound", shape: prim("int64"),
			constraints: map[string]string{"gte": "9007199254740992", "min": "9007199254740993"},
			expected:    []OpenAPIConstraint{{Name: "minimum", Value: int64(9007199254740993)}},
			description: "distinct int64 bounds above 2^53 compare as int64, not float64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapConstraintToOpenAPI(tt.shape, tt.underlyingKind, tt.constraints)

			if len(result) != len(tt.expected) {
				t.Errorf("%s: expected %d constraints, got %d", tt.description, len(tt.expected), len(result))
				t.Logf("Expected: %+v", tt.expected)
				t.Logf("Got: %+v", result)
				return
			}

			resultMap := make(map[string]any, len(result))
			for _, c := range result {
				resultMap[c.Name] = c.Value
			}
			assertConstraintsMatch(t, tt.description, tt.expected, resultMap)
		})
	}
}

// TestMapConstraintToOpenAPIDeterministicOverlap guards emission when two distinct
// validator keys collapse to the SAME OpenAPI keyword. validator/v10 enforces ALL
// rules, so the effective bound is the MOST-RESTRICTIVE one: the larger value for a
// lower bound (minimum/minLength) and the smaller for an upper bound (maximum).
// resolveMostRestrictive collapses the duplicates so the emitted value no longer
// depends on map-iteration order — it is the binding constraint on every run.
func TestMapConstraintToOpenAPIDeterministicOverlap(t *testing.T) {
	cases := []struct {
		name        string
		shape       models.TypeShape
		constraints map[string]string
		keyword     string
		wantValue   any
	}{
		{
			name:        "min and gte both map to minimum",
			shape:       prim("int"),
			constraints: map[string]string{"min": "1", "gte": "10"},
			keyword:     "minimum",
			// both enforced; effective minimum is the larger bound, 10.
			wantValue: int64(10),
		},
		{
			name:        "max and lt both map to maximum",
			shape:       prim("int"),
			constraints: map[string]string{"max": "100", "lt": "50"},
			keyword:     "maximum",
			// both enforced; effective maximum is the smaller bound, 50.
			wantValue: int64(50),
		},
		{
			name:        "min and len both map to minLength on string",
			shape:       prim("string"),
			constraints: map[string]string{"min": "3", "len": "8"},
			keyword:     "minLength",
			// both enforced; effective minLength is the larger bound, 8.
			wantValue: 8,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertDeterministicConstraint(t, tc.shape, tc.constraints, tc.keyword, tc.wantValue)
		})
	}
}

// assertDeterministicConstraint asserts MapConstraintToOpenAPI emits keyword with
// wantValue on every run. It repeats many times because a single run can
// coincidentally match even when the underlying iteration order is nondeterministic.
func assertDeterministicConstraint(t *testing.T, shape models.TypeShape, constraints map[string]string, keyword string, wantValue any) {
	t.Helper()
	for i := 0; i < 100; i++ {
		got, found := constraintByName(MapConstraintToOpenAPI(shape, "", constraints), keyword)
		if !found {
			t.Fatalf("iteration %d: keyword %q not emitted", i, keyword)
		}
		if got != wantValue {
			t.Fatalf("iteration %d: %q = %v (%T), want %v (%T) — nondeterministic emission",
				i, keyword, got, got, wantValue, wantValue)
		}
	}
}

// constraintByName returns the value emitted for the given OpenAPI keyword, if any.
// It returns the LAST match, mirroring the generator's last-writer-wins overwrite
// when several validator keys collapse to the same keyword (e.g. min & gte both
// emit a "minimum" entry) — which is the value that actually lands in the spec.
func constraintByName(result []OpenAPIConstraint, name string) (any, bool) {
	var value any
	found := false
	for _, c := range result {
		if c.Name == name {
			value = c.Value
			found = true
		}
	}
	return value, found
}

// assertConstraintsMatch verifies every expected constraint is present in
// resultMap with the expected value. Enum constraints are compared element-wise
// since they are slices; scalar constraints are compared via direct equality.
func assertConstraintsMatch(t *testing.T, description string, expected []OpenAPIConstraint, resultMap map[string]any) {
	t.Helper()
	for _, exp := range expected {
		gotValue, ok := resultMap[exp.Name]
		if !ok {
			t.Errorf("%s: missing constraint %q", description, exp.Name)
			continue
		}
		if exp.Name == "enum" {
			assertEnumConstraint(t, description, exp.Value, gotValue)
			continue
		}
		if gotValue != exp.Value {
			t.Errorf("%s: constraint %q: expected %v (type %T), got %v (type %T)",
				description, exp.Name, exp.Value, exp.Value, gotValue, gotValue)
		}
	}
}

// assertEnumConstraint compares two enum constraint values element-wise after
// asserting both are []any slices.
func assertEnumConstraint(t *testing.T, description string, expected, got any) {
	t.Helper()
	expectedArr, ok1 := expected.([]any)
	gotArr, ok2 := got.([]any)
	if !ok1 || !ok2 {
		t.Errorf("%s: enum values are not arrays", description)
		return
	}
	if len(expectedArr) != len(gotArr) {
		t.Errorf("%s: enum array length mismatch: expected %d, got %d", description, len(expectedArr), len(gotArr))
		return
	}
	for i := range expectedArr {
		if expectedArr[i] != gotArr[i] {
			t.Errorf("%s: enum[%d]: expected %v, got %v", description, i, expectedArr[i], gotArr[i])
		}
	}
}

func TestIsStringType(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"string", true},
		{"int", false},
		{"float64", false},
		{"bool", false},
		{"[]string", false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := isStringType(tt.typeName)
			if result != tt.expected {
				t.Errorf("isStringType(%q): expected %v, got %v", tt.typeName, tt.expected, result)
			}
		})
	}
}

func TestIsNumericType(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"int", true},
		{"int8", true},
		{"int16", true},
		{"int32", true},
		{"int64", true},
		{"uint", true},
		{"uint8", true},
		{"uint16", true},
		{"uint32", true},
		{"uint64", true},
		{"float32", true},
		{"float64", true},
		{"string", false},
		{"bool", false},
		{"[]int", false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := isNumericType(tt.typeName)
			if result != tt.expected {
				t.Errorf("isNumericType(%q): expected %v, got %v", tt.typeName, tt.expected, result)
			}
		})
	}
}

func TestParseNumeric(t *testing.T) {
	tests := []struct {
		value       string
		expected    any
		shouldError bool
		description string
	}{
		{
			value:       "42",
			expected:    int64(42),
			shouldError: false,
			description: "should parse integer",
		},
		{
			value:       "3.14",
			expected:    3.14,
			shouldError: false,
			description: "should parse float",
		},
		{
			value:       "0",
			expected:    int64(0),
			shouldError: false,
			description: "should parse zero",
		},
		{
			value:       "-10",
			expected:    int64(-10),
			shouldError: false,
			description: "should parse negative integer",
		},
		{
			value:       "-2.5",
			expected:    -2.5,
			shouldError: false,
			description: "should parse negative float",
		},
		{
			value:       "invalid",
			shouldError: true,
			description: "should error on invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result, err := parseNumeric(tt.value)

			if tt.shouldError {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				}
				return
			}
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}
			if result != tt.expected {
				t.Errorf("%s: expected %v (type %T), got %v (type %T)",
					tt.description, tt.expected, tt.expected, result, result)
			}
		})
	}
}
