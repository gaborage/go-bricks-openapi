package generator

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// --- Task 3: numericBound + constraintSet merge semantics ---

func TestConstraintSetBoundsKeepMostRestrictive(t *testing.T) {
	var s constraintSet
	s.setMinLength(1)
	s.setMinLength(10)
	s.setMinLength(5)
	if *s.minLength != 10 {
		t.Errorf("minLength = %d, want 10 (largest floor)", *s.minLength)
	}
	s.setMaxLength(100)
	s.setMaxLength(20)
	s.setMaxLength(50)
	if *s.maxLength != 20 {
		t.Errorf("maxLength = %d, want 20 (smallest ceiling)", *s.maxLength)
	}
}

func TestConstraintSetNumericTieExclusiveWins(t *testing.T) {
	var s constraintSet
	s.setMinimum(numericBound{intVal: 5})                  // gte=5
	s.setMinimum(numericBound{intVal: 5, exclusive: true}) // gt=5
	if !s.minimum.exclusive || s.minimum.intVal != 5 {
		t.Errorf("tie must keep exclusive: %+v", *s.minimum)
	}
	// A strictly larger inclusive floor beats a smaller exclusive one and drops
	// its exclusivity (exclusivity travels with the winning value).
	s.setMinimum(numericBound{intVal: 7})
	if s.minimum.exclusive || s.minimum.intVal != 7 {
		t.Errorf("larger inclusive floor must win outright: %+v", *s.minimum)
	}
}

func TestConstraintSetNumericInt64Precision(t *testing.T) {
	var s constraintSet
	s.setMinimum(numericBound{intVal: 9007199254740993}) // 2^53 + 1
	s.setMinimum(numericBound{intVal: 9007199254740992}) // 2^53
	if s.minimum.intVal != 9007199254740993 {
		t.Errorf("int64 bounds must compare as int64, got %d", s.minimum.intVal)
	}
	var f constraintSet
	f.setMaximum(numericBound{floatVal: 2.5, isFloat: true})
	f.setMaximum(numericBound{intVal: 3})
	if !f.maximum.isFloat || f.maximum.floatVal != 2.5 {
		t.Errorf("mixed compare must fall back to float64 and keep the winner's type: %+v", *f.maximum)
	}
}

func TestConstraintSetLastSetWinsForFormatPatternEnum(t *testing.T) {
	var s constraintSet
	s.setFormat("email")
	s.setFormat("uuid")
	s.setPattern("^a")
	s.setPattern("^b")
	s.setEnum([]any{"x"})
	s.setEnum([]any{"y"})
	if s.format != "uuid" || s.pattern != "^b" || len(s.enum) != 1 || s.enum[0] != "y" {
		t.Errorf("last set must win: %+v", s)
	}
}

func TestConstraintSetApplyToWritesOnlySetFields(t *testing.T) {
	prop := &OpenAPIProperty{Minimum: floatPtr(0)} // the uint pre-stamp
	var empty constraintSet
	empty.applyTo(prop)
	if prop.Minimum == nil || *prop.Minimum != 0 {
		t.Error("empty set must not clear the uint minimum:0 pre-stamp")
	}
	s := constraintSet{minimum: &numericBound{intVal: 5, exclusive: true}, minLength: intPtr(3), enum: []any{"a"}}
	s.applyTo(prop)
	if *prop.Minimum != 5 || prop.ExclusiveMinimum == nil || !*prop.ExclusiveMinimum || *prop.MinLength != 3 || len(prop.Enum) != 1 {
		t.Errorf("applyTo did not write set fields: %+v", prop)
	}
	if prop.ExclusiveMaximum != nil || prop.Maximum != nil {
		t.Error("applyTo must not write unset fields")
	}
}

// --- Task 5: constraintsFor's corpus, converted to property-out assertions ---

func TestConstraintsFor(t *testing.T) {
	tests := []struct {
		name           string
		shape          models.TypeShape
		underlyingKind string
		constraints    map[string]string
		want           OpenAPIProperty
		description    string
	}{
		{
			name:        "email format",
			shape:       prim("string"),
			constraints: map[string]string{"email": "true"},
			want:        OpenAPIProperty{Format: "email"},
			description: "should map email to format constraint",
		},
		{
			name:        "url format",
			shape:       prim("string"),
			constraints: map[string]string{"url": "true"},
			want:        OpenAPIProperty{Format: "uri"},
			description: "should map url to uri format",
		},
		{
			name:        "uuid format",
			shape:       prim("string"),
			constraints: map[string]string{"uuid": "true"},
			want:        OpenAPIProperty{Format: "uuid"},
			description: "should map uuid to format constraint",
		},
		{
			name:        "date format",
			shape:       prim("string"),
			constraints: map[string]string{"date": "true"},
			want:        OpenAPIProperty{Format: "date"},
			description: "should map date to format constraint",
		},
		{
			name:        "datetime format",
			shape:       prim("string"),
			constraints: map[string]string{validatorDatetime: "true"},
			want:        OpenAPIProperty{Format: "date-time"},
			description: "should map datetime to date-time format",
		},
		{
			name:        "string min length",
			shape:       prim("string"),
			constraints: map[string]string{"min": "5"},
			want:        OpenAPIProperty{MinLength: intPtr(5)},
			description: "should map min to minLength for strings",
		},
		{
			name:        "string max length",
			shape:       prim("string"),
			constraints: map[string]string{"max": "100"},
			want:        OpenAPIProperty{MaxLength: intPtr(100)},
			description: "should map max to maxLength for strings",
		},
		{
			name:        "string exact length",
			shape:       prim("string"),
			constraints: map[string]string{"len": "10"},
			want:        OpenAPIProperty{MinLength: intPtr(10), MaxLength: intPtr(10)},
			description: "should map len to both minLength and maxLength",
		},
		{
			name:        "integer minimum",
			shape:       prim("int"),
			constraints: map[string]string{"min": "18"},
			want:        OpenAPIProperty{Minimum: floatPtr(18)},
			description: "should map min to minimum for integers",
		},
		{
			name:        "integer maximum",
			shape:       prim("int"),
			constraints: map[string]string{"max": "120"},
			want:        OpenAPIProperty{Maximum: floatPtr(120)},
			description: "should map max to maximum for integers",
		},
		{
			name:        "int64 minimum",
			shape:       prim("int64"),
			constraints: map[string]string{"min": "1000"},
			want:        OpenAPIProperty{Minimum: floatPtr(1000)},
			description: "should handle int64 type",
		},
		{
			name:        "float minimum",
			shape:       prim("float64"),
			constraints: map[string]string{"min": "0.5"},
			want:        OpenAPIProperty{Minimum: floatPtr(0.5)},
			description: "should map min to minimum for floats",
		},
		{
			name:        "float maximum",
			shape:       prim("float64"),
			constraints: map[string]string{"max": "99.9"},
			want:        OpenAPIProperty{Maximum: floatPtr(99.9)},
			description: "should map max to maximum for floats",
		},
		{
			name:        "greater than (exclusive minimum)",
			shape:       prim("int"),
			constraints: map[string]string{"gt": "0"},
			want:        OpenAPIProperty{Minimum: floatPtr(0), ExclusiveMinimum: boolPtr(true)},
			description: "should map gt to exclusive minimum",
		},
		{
			name:        "greater than or equal",
			shape:       prim("int"),
			constraints: map[string]string{"gte": "1"},
			want:        OpenAPIProperty{Minimum: floatPtr(1)},
			description: "should map gte to minimum",
		},
		{
			name:        "less than (exclusive maximum)",
			shape:       prim("int"),
			constraints: map[string]string{"lt": "100"},
			want:        OpenAPIProperty{Maximum: floatPtr(100), ExclusiveMaximum: boolPtr(true)},
			description: "should map lt to exclusive maximum",
		},
		{
			name:        "less than or equal",
			shape:       prim("int"),
			constraints: map[string]string{"lte": "99"},
			want:        OpenAPIProperty{Maximum: floatPtr(99)},
			description: "should map lte to maximum",
		},
		{
			name:        "oneof enum",
			shape:       prim("string"),
			constraints: map[string]string{"oneof": "red green blue"},
			want:        OpenAPIProperty{Enum: []any{"red", "green", "blue"}},
			description: "should map oneof to enum array",
		},
		{
			name:        "oneof numeric enum int",
			shape:       prim("int"),
			constraints: map[string]string{"oneof": "1 2 3"},
			want:        OpenAPIProperty{Enum: []any{int64(1), int64(2), int64(3)}},
			description: "should map oneof to numeric enum for int type",
		},
		{
			name:        "oneof numeric enum float64",
			shape:       prim("float64"),
			constraints: map[string]string{"oneof": "1.5 2.5 3.5"},
			want:        OpenAPIProperty{Enum: []any{1.5, 2.5, 3.5}},
			description: "should map oneof to numeric enum for float64 type",
		},
		{
			name:        "oneof pointer numeric type",
			shape:       ptrOf(prim("int")),
			constraints: map[string]string{"oneof": "10 20 30"},
			want:        OpenAPIProperty{Enum: []any{int64(10), int64(20), int64(30)}},
			description: "should handle pointer numeric types correctly",
		},
		{
			name:        "regexp pattern",
			shape:       prim("string"),
			constraints: map[string]string{"regexp": "^[A-Z]+$"},
			want:        OpenAPIProperty{Pattern: "^[A-Z]+$"},
			description: "should map regexp to pattern",
		},
		{
			name:  "required constraint skipped",
			shape: prim("string"),
			constraints: map[string]string{
				"required": "true",
				"email":    "true",
			},
			want:        OpenAPIProperty{Format: "email"},
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
			want:        OpenAPIProperty{Format: "email", MinLength: intPtr(5), MaxLength: intPtr(100)},
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
			want:        OpenAPIProperty{Minimum: floatPtr(1), Maximum: floatPtr(1000)},
			description: "should map multiple integer constraints",
		},
		{
			name:        "pointer type stripped",
			shape:       ptrOf(prim("string")),
			constraints: map[string]string{"min": "5"},
			want:        OpenAPIProperty{MinLength: intPtr(5)},
			description: "should strip pointer prefix from type",
		},
		{
			name:        "empty constraints",
			shape:       prim("string"),
			constraints: map[string]string{},
			want:        OpenAPIProperty{},
			description: "should return empty array for no constraints",
		},
		// --- PR11: named-numeric via UnderlyingKind ---
		{
			name: "named integer min/max via UnderlyingKind", shape: named("Cents"), underlyingKind: typeInteger,
			constraints: map[string]string{"min": "100", "max": "1000"},
			want:        OpenAPIProperty{Minimum: floatPtr(100), Maximum: floatPtr(1000)},
			description: "type Cents int64 must map numeric constraints (was dropped)",
		},
		{
			name: "time.Duration gte via UnderlyingKind", shape: named("time.Duration"), underlyingKind: typeInteger,
			constraints: map[string]string{"gte": "1"},
			want:        OpenAPIProperty{Minimum: floatPtr(1)},
			description: "time.Duration maps numeric constraints",
		},
		{
			name: "named integer gt via UnderlyingKind", shape: named("Cents"), underlyingKind: typeInteger,
			constraints: map[string]string{"gt": "0"},
			want:        OpenAPIProperty{Minimum: floatPtr(0), ExclusiveMinimum: boolPtr(true)},
			description: "gt on named numeric emits minimum + exclusiveMinimum",
		},
		{
			name: "named integer oneof via UnderlyingKind", shape: named("Status"), underlyingKind: typeInteger,
			constraints: map[string]string{"oneof": "1 2 3"},
			want:        OpenAPIProperty{Enum: []any{int64(1), int64(2), int64(3)}},
			description: "oneof on named numeric yields numeric enum",
		},
		// --- PR11: string length comparisons ---
		{
			name: "string gt -> minLength+1", shape: prim("string"),
			constraints: map[string]string{"gt": "3"},
			want:        OpenAPIProperty{MinLength: intPtr(4)},
			description: "gt on string constrains length",
		},
		{
			name: "string gt=0 non-empty idiom", shape: prim("string"),
			constraints: map[string]string{"gt": "0"},
			want:        OpenAPIProperty{MinLength: intPtr(1)},
			description: "gt=0 -> minLength 1",
		},
		{
			name: "string lt -> maxLength-1", shape: prim("string"),
			constraints: map[string]string{"lt": "10"},
			want:        OpenAPIProperty{MaxLength: intPtr(9)},
			description: "lt on string constrains length",
		},
		{
			name: "string gt negative clamps to 0", shape: prim("string"),
			constraints: map[string]string{"gt": "-5"},
			want:        OpenAPIProperty{MinLength: intPtr(0)},
			description: "negative length clamps to non-negative",
		},
		// --- PR11: slice cardinality ---
		{
			name: "slice min -> minItems", shape: sliceOf(prim("string")),
			constraints: map[string]string{"min": "1"},
			want:        OpenAPIProperty{MinItems: intPtr(1)},
			description: "min on []T maps to minItems",
		},
		{
			name: "slice len -> minItems+maxItems", shape: sliceOf(prim("int")),
			constraints: map[string]string{"len": "3"},
			want:        OpenAPIProperty{MinItems: intPtr(3), MaxItems: intPtr(3)},
			description: "len on []T maps to minItems == maxItems",
		},
		{
			name: "pointer slice min -> minItems", shape: ptrOf(sliceOf(prim("string"))),
			constraints: map[string]string{"min": "2"},
			want:        OpenAPIProperty{MinItems: intPtr(2)},
			description: "pointer-to-slice stripped, cardinality applies",
		},
		{
			name: "byte slice is not an array", shape: sliceOf(prim("byte")),
			constraints: map[string]string{"min": "10"},
			want:        OpenAPIProperty{},
			description: "[]byte is a base64 string, not a cardinality-bearing array",
		},
		// --- PR11: oneof quoting, eq, ne, datetime, formats, patterns ---
		{
			name: "oneof quoted multi-word", shape: prim("string"),
			constraints: map[string]string{"oneof": "'New York' 'Los Angeles'"},
			want:        OpenAPIProperty{Enum: []any{"New York", "Los Angeles"}},
			description: "quote-aware oneof keeps spaces",
		},
		{
			name: "oneof unquoted still splits", shape: prim("string"),
			constraints: map[string]string{"oneof": "active pending"},
			want:        OpenAPIProperty{Enum: []any{"active", "pending"}},
			description: "unquoted oneof splits on spaces",
		},
		{
			name: "eq -> single-element enum", shape: prim("string"),
			constraints: map[string]string{"eq": "GOLD"},
			want:        OpenAPIProperty{Enum: []any{"GOLD"}},
			description: "eq maps to a single-value enum",
		},
		{
			name: "ne -> nothing", shape: prim("string"),
			constraints: map[string]string{"ne": "X"},
			want:        OpenAPIProperty{},
			description: "ne has no clean OpenAPI representation",
		},
		{
			name: "datetime date-only layout -> date", shape: prim("string"),
			constraints: map[string]string{validatorDatetime: "2006-01-02"},
			want:        OpenAPIProperty{Format: "date"},
			description: "date-only layout maps to format date",
		},
		{
			name: "datetime with clock -> date-time", shape: prim("string"),
			constraints: map[string]string{validatorDatetime: "2006-01-02T15:04:05Z07:00"},
			want:        OpenAPIProperty{Format: "date-time"},
			description: "layout with clock tokens maps to date-time",
		},
		{
			name: "ipv4 format", shape: prim("string"),
			constraints: map[string]string{formatIPv4: "true"},
			want:        OpenAPIProperty{Format: formatIPv4},
			description: "ipv4 maps to format ipv4",
		},
		{
			name: "base64 -> byte format", shape: prim("string"),
			constraints: map[string]string{"base64": "true"},
			want:        OpenAPIProperty{Format: "byte"},
			description: "base64 maps to OpenAPI byte format",
		},
		{
			name: "alpha -> anchored pattern", shape: prim("string"),
			constraints: map[string]string{"alpha": "true"},
			want:        OpenAPIProperty{Pattern: `^[a-zA-Z]+$`},
			description: "alpha maps to a letter-only pattern",
		},
		{
			name: "startswith -> anchored escaped pattern", shape: prim("string"),
			constraints: map[string]string{"startswith": "a.b"},
			want:        OpenAPIProperty{Pattern: `^a\.b`},
			description: "startswith escapes metacharacters and anchors at start",
		},
		{
			name: "bare ip -> no format/pattern (documented)", shape: prim("string"),
			constraints: map[string]string{"ip": "true"},
			want:        OpenAPIProperty{},
			description: "bare ip has no single clean OpenAPI format; left unconstrained by design",
		},
		{
			name: "contains -> unanchored escaped pattern", shape: prim("string"),
			constraints: map[string]string{"contains": "a.b"},
			want:        OpenAPIProperty{Pattern: `a\.b`},
			description: "contains escapes metacharacters, no anchors",
		},
		{
			name: "endswith -> end-anchored escaped pattern", shape: prim("string"),
			constraints: map[string]string{"endswith": ".json"},
			want:        OpenAPIProperty{Pattern: `\.json$`},
			description: "endswith escapes and anchors at end",
		},
		{
			name: "string gte -> minLength", shape: prim("string"),
			constraints: map[string]string{"gte": "3"},
			want:        OpenAPIProperty{MinLength: intPtr(3)},
			description: "gte on string is a length floor",
		},
		{
			name: "string lte -> maxLength", shape: prim("string"),
			constraints: map[string]string{"lte": "10"},
			want:        OpenAPIProperty{MaxLength: intPtr(10)},
			description: "lte on string is a length ceiling",
		},
		{
			name: "slice max -> maxItems", shape: sliceOf(prim("string")),
			constraints: map[string]string{"max": "5"},
			want:        OpenAPIProperty{MaxItems: intPtr(5)},
			description: "max on []T maps to maxItems",
		},
		// --- map cardinality (issue #3) ---
		{
			name: "map min -> minProperties", shape: mapOf(prim("string"), prim("string")),
			constraints: map[string]string{"min": "1"},
			want:        OpenAPIProperty{MinProperties: intPtr(1)},
			description: "min on map[string]T maps to minProperties",
		},
		{
			name: "map max -> maxProperties", shape: mapOf(prim("string"), prim("string")),
			constraints: map[string]string{"max": "10"},
			want:        OpenAPIProperty{MaxProperties: intPtr(10)},
			description: "max on map[string]T maps to maxProperties",
		},
		{
			name: "map len -> minProperties+maxProperties", shape: mapOf(prim("string"), prim("string")),
			constraints: map[string]string{"len": "3"},
			want:        OpenAPIProperty{MinProperties: intPtr(3), MaxProperties: intPtr(3)},
			description: "len on map[string]T maps to minProperties == maxProperties",
		},
		{
			name: "pointer map min -> minProperties", shape: ptrOf(mapOf(prim("string"), prim("int"))),
			constraints: map[string]string{"min": "2"},
			want:        OpenAPIProperty{MinProperties: intPtr(2)},
			description: "pointer-to-map stripped, cardinality applies",
		},
		{
			// The pre-Shape spelling of this row was map[string]struct{}, an
			// anonymous struct the decoder does not model — hence the unknown
			// value shape. The named-struct row below is the case the old name
			// claimed; both must route identically.
			name: "unmodeled-value map cardinality", shape: mapOf(prim("string"), unknownShape()),
			constraints: map[string]string{"min": "1", "max": "5"},
			want:        OpenAPIProperty{MinProperties: intPtr(1), MaxProperties: intPtr(5)},
			description: "cardinality is independent of the map value type",
		},
		{
			name: "struct-valued map cardinality", shape: mapOf(prim("string"), named("Address")),
			constraints: map[string]string{"min": "1", "max": "5"},
			want:        OpenAPIProperty{MinProperties: intPtr(1), MaxProperties: intPtr(5)},
			description: "cardinality is independent of the map value type",
		},
		// --- Issue #2: most-restrictive bound when multiple rules collapse to one keyword ---
		{
			name: "numeric min+gte -> larger minimum", shape: prim("int"),
			constraints: map[string]string{"min": "1", "gte": "10"},
			want:        OpenAPIProperty{Minimum: floatPtr(10)},
			description: "validator enforces both; effective minimum is the larger (10)",
		},
		{
			name: "numeric max+lt -> smaller exclusive maximum", shape: prim("int"),
			constraints: map[string]string{"max": "100", "lt": "50"},
			want:        OpenAPIProperty{Maximum: floatPtr(50), ExclusiveMaximum: boolPtr(true)},
			description: "effective maximum is the smaller (50); lt is exclusive",
		},
		{
			name: "string min+gte -> larger minLength", shape: prim("string"),
			constraints: map[string]string{"min": "2", "gte": "5"},
			want:        OpenAPIProperty{MinLength: intPtr(5)},
			description: "effective minLength is the larger (5)",
		},
		{
			name: "string max+lte -> smaller maxLength", shape: prim("string"),
			constraints: map[string]string{"max": "20", "lte": "8"},
			want:        OpenAPIProperty{MaxLength: intPtr(8)},
			description: "effective maxLength is the smaller (8)",
		},
		{
			name: "numeric gt+gte equal magnitudes -> inclusive binds", shape: prim("int"),
			constraints: map[string]string{"gt": "5", "gte": "10"},
			want:        OpenAPIProperty{Minimum: floatPtr(10)},
			description: "gte=10 (>gt=5) binds; inclusive 10 wins, no exclusiveMinimum",
		},
		{
			name: "numeric gte+gt equal values -> exclusive wins", shape: prim("int"),
			constraints: map[string]string{"gte": "10", "gt": "10"},
			want:        OpenAPIProperty{Minimum: floatPtr(10), ExclusiveMinimum: boolPtr(true)},
			description: "equal value, exclusive (gt) is more restrictive than inclusive (gte)",
		},
		{
			name: "slice min+len -> larger minItems plus maxItems", shape: sliceOf(prim("string")),
			constraints: map[string]string{"min": "1", "len": "3"},
			want:        OpenAPIProperty{MinItems: intPtr(3), MaxItems: intPtr(3)},
			description: "min=1 and len=3 both touch minItems; max(1,3)=3, maxItems=3 from len",
		},
		{
			// gt=2^53 (exclusive) and gte=2^53+1 (inclusive) collapse onto minimum.
			// int64 compare treats 2^53+1 as strictly larger, so the larger
			// INCLUSIVE floor wins outright and its exclusivity is dropped. A
			// float64-based compare would round both to the same float64 value,
			// treat them as a TIE, and wrongly OR in the exclusive flag from gt.
			// The emitted Minimum itself can't distinguish the two internal values
			// (both round to the same float64), so this row asserts on the
			// ExclusiveMinimum flag's absence, not on the numeric value.
			name: "int64 minimum overlap above 2^53 keeps the exact larger bound", shape: prim("int64"),
			constraints: map[string]string{"gt": "9007199254740992", "gte": "9007199254740993"},
			want:        OpenAPIProperty{Minimum: floatPtr(9007199254740992)},
			description: "distinct int64 bounds above 2^53 compare as int64, not float64",
		},
		// --- parse-failure rows: an unparsable value drops the rule silently ---
		{
			name: "slice min=abc parse failure", shape: sliceOf(prim("string")),
			constraints: map[string]string{"min": "abc"},
			want:        OpenAPIProperty{},
			description: "non-numeric min on a slice drops silently",
		},
		{
			name: "map max=x parse failure", shape: mapOf(prim("string"), prim("string")),
			constraints: map[string]string{"max": "x"},
			want:        OpenAPIProperty{},
			description: "non-numeric max on a map drops silently",
		},
		{
			name: "string gt=abc parse failure", shape: prim("string"),
			constraints: map[string]string{"gt": "abc"},
			want:        OpenAPIProperty{},
			description: "non-numeric gt on a string drops silently",
		},
		{
			name: "int lt=abc parse failure", shape: prim("int"),
			constraints: map[string]string{"lt": "abc"},
			want:        OpenAPIProperty{},
			description: "non-numeric lt on an int drops silently",
		},
		{
			name: "string len=abc parse failure", shape: prim("string"),
			constraints: map[string]string{"len": "abc"},
			want:        OpenAPIProperty{},
			description: "non-numeric len on a string drops silently",
		},
		{
			name: "oneof empty value parse failure", shape: prim("string"),
			constraints: map[string]string{"oneof": ""},
			want:        OpenAPIProperty{},
			description: "an empty oneof tokenizes to nothing and drops silently",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got OpenAPIProperty
			constraintsFor(tt.shape, tt.underlyingKind, tt.constraints).applyTo(&got)
			assert.Equal(t, tt.want, got, tt.description)
		})
	}
}

// TestConstraintsForRepeatedKeywordLastSortedKeyWins pins the Q10 precedence
// rule: when several validator keys map to the SAME non-bound keyword
// (format/pattern/enum), the one that sorts LAST wins — matching the retired
// applicators' last-writer-wins overwrite.
func TestConstraintsForRepeatedKeywordLastSortedKeyWins(t *testing.T) {
	var got OpenAPIProperty
	// sorted: alphanum < email < eq < oneof < regexp < uuid4 ->
	// email < uuid4 so uuid wins; alphanum < regexp so regexp's pattern wins;
	// eq < oneof so oneof's enum wins. Matches the retired applicators'
	// last-writer-wins.
	constraintsFor(prim("string"), "", map[string]string{
		"email": "true", "uuid4": "true",
		"alphanum": "true", "regexp": "x",
		"eq": "a", "oneof": "b c",
	}).applyTo(&got)
	if got.Format != "uuid" || got.Pattern != "x" || !reflect.DeepEqual(got.Enum, []any{"b", "c"}) {
		t.Errorf("precedence drift: %+v", got)
	}
}

// TestConstraintsForDeterministicOverlap guards emission when two distinct
// validator keys collapse to the SAME OpenAPI keyword. validator/v10 enforces ALL
// rules, so the effective bound is the MOST-RESTRICTIVE one: the larger value for a
// lower bound (minimum/minLength) and the smaller for an upper bound (maximum).
// constraintSet's merge-at-set-time semantics make the emitted value independent
// of map-iteration order — it is the binding constraint on every run.
func TestConstraintsForDeterministicOverlap(t *testing.T) {
	cases := []struct {
		name        string
		shape       models.TypeShape
		constraints map[string]string
		want        OpenAPIProperty
	}{
		{
			name:        "min and gte both map to minimum",
			shape:       prim("int"),
			constraints: map[string]string{"min": "1", "gte": "10"},
			// both enforced; effective minimum is the larger bound, 10.
			want: OpenAPIProperty{Minimum: floatPtr(10)},
		},
		{
			name:        "max and lt both map to maximum",
			shape:       prim("int"),
			constraints: map[string]string{"max": "100", "lt": "50"},
			// both enforced; effective maximum is the smaller bound, 50 (from lt, exclusive).
			want: OpenAPIProperty{Maximum: floatPtr(50), ExclusiveMaximum: boolPtr(true)},
		},
		{
			name:        "min and len both map to minLength on string",
			shape:       prim("string"),
			constraints: map[string]string{"min": "3", "len": "8"},
			// both enforced; effective minLength is the larger bound, 8 (len also sets maxLength).
			want: OpenAPIProperty{MinLength: intPtr(8), MaxLength: intPtr(8)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertDeterministicConstraint(t, tc.shape, tc.constraints, &tc.want)
		})
	}
}

// assertDeterministicConstraint asserts constraintsFor emits want on every run. It
// repeats many times because a single run can coincidentally match even when the
// underlying iteration order is nondeterministic. The loop is redundant now that
// constraintsFor sorts its keys, but it is harmless and keeps the guard explicit.
// want is taken by pointer (gocritic hugeParam: OpenAPIProperty is 256 B) — the
// callee only reads it.
func assertDeterministicConstraint(t *testing.T, shape models.TypeShape, constraints map[string]string, want *OpenAPIProperty) {
	t.Helper()
	for i := 0; i < 100; i++ {
		var got OpenAPIProperty
		constraintsFor(shape, "", constraints).applyTo(&got)
		require.Equal(t, *want, got, "iteration %d", i)
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
