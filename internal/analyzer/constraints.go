package analyzer

import (
	"cmp"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// OpenAPIConstraint represents a single OpenAPI schema constraint
type OpenAPIConstraint struct {
	Name  string // OpenAPI property name (e.g., "minLength", "minimum", "format")
	Value any    // Constraint value (string, int, bool, []string for enum)
}

const (
	// OpenAPI constraint property names
	constraintFormat           = "format"
	constraintMinLength        = "minLength"
	constraintMaxLength        = "maxLength"
	constraintMinimum          = "minimum"
	constraintMaximum          = "maximum"
	constraintMinItems         = "minItems"
	constraintMaxItems         = "maxItems"
	constraintMinProperties    = "minProperties"
	constraintMaxProperties    = "maxProperties"
	constraintEnum             = "enum"
	constraintPattern          = "pattern"
	constraintExclusiveMinimum = "exclusiveMinimum"
	constraintExclusiveMaximum = "exclusiveMaximum"

	// Validator tag values
	validatorOneOf    = "oneof"
	validatorDatetime = "datetime"

	// OpenAPI format values
	formatEmail    = "email"
	formatUUID     = "uuid"
	formatDate     = "date"
	formatDateTime = "date-time"
	formatByte     = "byte"
	formatIPv4     = "ipv4"

	// OpenAPI 3-way kind values (also the values of FieldInfo.UnderlyingKind).
	kindInteger = "integer"
	kindNumber  = "number"

	// Go primitive type names referenced for type discrimination
	goTypeInt64   = "int64"
	goTypeFloat32 = "float32"
	goTypeFloat64 = "float64"
)

// MapConstraintToOpenAPI converts validation constraints to OpenAPI schema properties
// Takes the field type and constraints map, returns OpenAPI-compatible constraints
func MapConstraintToOpenAPI(fieldType, underlyingKind string, constraints map[string]string) []OpenAPIConstraint {
	var result []OpenAPIConstraint

	baseType := strings.TrimPrefix(fieldType, "*")
	// []byte/[]uint8 are well-known base64 string types, not arrays — treat them as
	// scalars (the well-known mapper already types them string/binary), not slices.
	// Their min/max are byte counts, which do NOT equal the base64-encoded character
	// length, so we deliberately drop length bounds on them rather than emit a wrong
	// minLength/maxLength. Map types are handled by a parallel branch below: their
	// effectiveKind is "" (neither string nor numeric), so min/max/len route to
	// minProperties/maxProperties (entry-count cardinality) rather than being dropped.
	isSlice := isSliceType(fieldType) && !isByteSlice(baseType)
	isMap := isMapType(fieldType)
	effKind := effectiveKind(baseType, underlyingKind)

	// Iterate keys in sorted order so the emitted constraints are deterministic.
	// Go map iteration is randomized, and distinct validator keys can collapse to
	// the SAME OpenAPI keyword (min & gte -> minimum; max & lt -> maximum; min/len/gt
	// -> minLength; etc.). The generator's applyConstraint overwrites the scalar prop
	// field last-writer-wins, so a random range made the emitted minimum/maximum
	// nondeterministic across runs for fields like `validate:"min=1,gte=10"`. Sorting
	// fixes a stable precedence (and stable golden output).
	for _, key := range sortedKeys(constraints) {
		if key == constraintRequired {
			continue // handled at schema level
		}
		value := constraints[key]

		// Slice fields: only cardinality (min/max/len -> minItems/maxItems) applies
		// to the array itself; element rules arrive via ElementConstraints + dive.
		if isSlice {
			result = append(result, handleSliceCardinality(key, value)...)
			continue
		}
		// Map fields: only entry-count cardinality (min/max/len -> minProperties/
		// maxProperties) applies to the map itself. A map type never matches isSlice
		// ("map[" does not start with "[]"), so the two branches are mutually exclusive.
		if isMap {
			result = append(result, handleMapCardinality(key, value)...)
			continue
		}
		result = append(result, dispatchScalarConstraint(key, value, effKind)...)
	}

	return resolveMostRestrictive(result)
}

// lowerBoundKeywords are OpenAPI floor keywords. When several validator rules
// collapse to the same one, the MOST-RESTRICTIVE (largest) value binds.
var lowerBoundKeywords = map[string]bool{
	constraintMinimum: true, constraintMinLength: true,
	constraintMinItems: true, constraintMinProperties: true,
}

// upperBoundKeywords are OpenAPI ceiling keywords. When several validator rules
// collapse to the same one, the MOST-RESTRICTIVE (smallest) value binds.
var upperBoundKeywords = map[string]bool{
	constraintMaximum: true, constraintMaxLength: true,
	constraintMaxItems: true, constraintMaxProperties: true,
}

// resolveMostRestrictive collapses duplicate bound keywords to the single
// most-restrictive entry. validator/v10 enforces ALL rules, so when distinct
// validator keys map to the same OpenAPI keyword (e.g. min & gte -> minimum), the
// binding bound is the largest floor / smallest ceiling — not whichever the map
// iteration emitted last. Non-bound constraints pass through unchanged in input
// order; each resolved bound is anchored at its first occurrence so the slice
// stays deterministic and stable.
func resolveMostRestrictive(in []OpenAPIConstraint) []OpenAPIConstraint {
	winners := map[string]boundState{}
	var order []string // bound keywords in first-seen order
	var out []OpenAPIConstraint

	for i := 0; i < len(in); i++ {
		c := in[i]
		if !isBoundKeyword(c.Name) {
			out = append(out, c) // pass-through (format, pattern, enum, exclusive flags w/o partner…)
			continue
		}
		cand, consumed := readBound(in, i)
		i += consumed - 1 // advance past a consumed exclusive partner
		if _, seen := winners[c.Name]; !seen {
			order = append(order, c.Name)
			out = append(out, OpenAPIConstraint{Name: boundPlaceholder, Value: c.Name}) // reserve slot
		}
		winners[c.Name] = mergeBound(winners[c.Name], cand, isLowerBound(c.Name))
	}
	return materializeBounds(out, winners, order)
}

// boundState is the running winner for one bound keyword: its (typed) value and
// whether the binding bound is exclusive (couples exclusiveMinimum/Maximum).
type boundState struct {
	value     any
	exclusive bool
	set       bool
}

// boundPlaceholder marks a reserved slot in the pass-through slice that is later
// replaced by the resolved bound (plus its exclusive flag, if any).
const boundPlaceholder = "\x00bound"

func isLowerBound(name string) bool   { return lowerBoundKeywords[name] }
func isUpperBound(name string) bool   { return upperBoundKeywords[name] }
func isBoundKeyword(name string) bool { return isLowerBound(name) || isUpperBound(name) }

// readBound reads the bound at index i, consuming a trailing exclusiveMinimum/
// exclusiveMaximum:true partner that handleNumericComparison emits immediately
// after a minimum/maximum. It returns the candidate bound and the number of input
// entries it spans (1, or 2 when an exclusive partner is consumed).
func readBound(in []OpenAPIConstraint, i int) (cand boundState, spanned int) {
	cand = boundState{value: in[i].Value, set: true}
	partner := exclusivePartner(in[i].Name)
	if partner != "" && i+1 < len(in) && in[i+1].Name == partner {
		cand.exclusive = true
		return cand, 2
	}
	return cand, 1
}

// exclusivePartner returns the exclusive-flag keyword coupled to a numeric bound
// (minimum -> exclusiveMinimum, maximum -> exclusiveMaximum), else "".
func exclusivePartner(name string) string {
	switch name {
	case constraintMinimum:
		return constraintExclusiveMinimum
	case constraintMaximum:
		return constraintExclusiveMaximum
	}
	return ""
}

// mergeBound folds a candidate into the running winner. For a lower bound the
// larger value wins; for an upper bound the smaller. On EQUAL value, exclusive
// beats inclusive (exclusive is strictly more restrictive).
func mergeBound(cur, cand boundState, lower bool) boundState {
	if !cur.set {
		return cand
	}
	ord := compareBoundValues(cand.value, cur.value)
	if ord == 0 {
		cur.exclusive = cur.exclusive || cand.exclusive
		return cur
	}
	if (lower && ord > 0) || (!lower && ord < 0) {
		return cand
	}
	return cur
}

// compareBoundValues orders two bound values numerically (minimum/maximum carry
// int64/float64; the length/cardinality keywords carry int). Returns -1/0/1.
// Integer bounds are compared as int64 so two distinct int64 values above 2^53
// cannot collapse to one float64 and mis-resolve the most-restrictive winner;
// only a fractional (float64) bound falls back to float64 comparison.
func compareBoundValues(a, b any) int {
	if ai, aok := boundInt64(a); aok {
		if bi, bok := boundInt64(b); bok {
			return cmp.Compare(ai, bi)
		}
	}
	return cmp.Compare(boundFloat64(a), boundFloat64(b))
}

// boundInt64 reports a bound value as an int64 when it is an integer kind (int or
// int64). A float64 bound returns ok=false so the caller compares as float64.
func boundInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}

// boundFloat64 converts a bound value (int, int64 or float64) to float64 for
// comparison only; the original typed value is preserved when re-emitted.
func boundFloat64(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	}
	return 0
}

// materializeBounds replaces each reserved placeholder with its resolved bound,
// re-emitting the exclusive flag only when the winning bound is exclusive.
func materializeBounds(out []OpenAPIConstraint, winners map[string]boundState, order []string) []OpenAPIConstraint {
	idx := 0 // next bound keyword (in first-seen order) to emit
	final := make([]OpenAPIConstraint, 0, len(out))
	for _, c := range out {
		if c.Name != boundPlaceholder {
			final = append(final, c)
			continue
		}
		name := order[idx]
		idx++
		w := winners[name]
		final = append(final, OpenAPIConstraint{Name: name, Value: w.value})
		if w.exclusive {
			final = append(final, OpenAPIConstraint{Name: exclusivePartner(name), Value: true})
		}
	}
	return final
}

// sortedKeys returns the keys of m in lexicographic order so callers can iterate
// deterministically (Go map iteration order is randomized).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// dispatchScalarConstraint routes a single (non-slice) constraint key to the first
// matching handler. Order matters: format/pattern tags are tried before the
// numeric/string handlers so a key is claimed by exactly one handler.
func dispatchScalarConstraint(key, value, effKind string) []OpenAPIConstraint {
	handlers := []func() []OpenAPIConstraint{
		func() []OpenAPIConstraint { return handleFormatConstraint(key, value) },
		func() []OpenAPIConstraint { return handlePatternFormatConstraint(key, value) },
		func() []OpenAPIConstraint { return handleMinConstraint(key, value, effKind) },
		func() []OpenAPIConstraint { return handleMaxConstraint(key, value, effKind) },
		func() []OpenAPIConstraint { return handleLenConstraint(key, value, effKind) },
		func() []OpenAPIConstraint { return handleNumericComparison(key, value, effKind) },
		func() []OpenAPIConstraint { return handleEnumConstraint(key, value, effKind) },
		func() []OpenAPIConstraint { return handleEqConstraint(key, value, effKind) },
		func() []OpenAPIConstraint { return handlePatternConstraint(key, value) },
	}
	for _, h := range handlers {
		if c := h(); c != nil {
			return c
		}
	}
	return nil
}

// effectiveKind resolves the OpenAPI 3-way kind to drive string-vs-numeric
// decisions: the analyzer-resolved UnderlyingKind (for named scalars like
// `type Cents int64` / time.Duration) wins; otherwise it is derived from the
// builtin base type. Empty when the type is neither string nor numeric.
func effectiveKind(baseType, underlyingKind string) string {
	if underlyingKind != "" {
		return underlyingKind
	}
	switch {
	case isStringType(baseType):
		return goTypeString
	case isIntegerType(baseType):
		return kindInteger
	case isFloatType(baseType):
		return kindNumber
	}
	return ""
}

func isEffectiveString(k string) bool  { return k == goTypeString }
func isEffectiveNumeric(k string) bool { return k == kindInteger || k == kindNumber }

// isByteSlice reports whether t is []byte/[]uint8 (a base64 string, not an array).
func isByteSlice(t string) bool { return t == "[]byte" || t == "[]uint8" }

// isSliceType reports whether t denotes a slice (after an optional leading
// pointer). Twin of generator.isSliceType — keep in sync.
func isSliceType(t string) bool {
	return strings.HasPrefix(strings.TrimPrefix(t, "*"), "[]")
}

// isMapType reports whether t denotes a map (after an optional leading pointer),
// e.g. "map[string]int" or "*map[string]int". Mirrors isSliceType.
func isMapType(t string) bool {
	return strings.HasPrefix(strings.TrimPrefix(t, "*"), "map[")
}

// formatTagMap maps boolean validator format tags to their OpenAPI `format`.
var formatTagMap = map[string]string{
	formatEmail: formatEmail,
	"url":       "uri",
	"uri":       "uri",
	formatUUID:  formatUUID,
	"uuid4":     formatUUID,
	formatDate:  formatDate,
	formatIPv4:  formatIPv4,
	"ipv6":      "ipv6",
	"hostname":  "hostname",
	"base64":    formatByte,
}

// handleFormatConstraint maps boolean format tags to OpenAPI `format`. datetime is
// value-aware: a date-only layout maps to `date`, otherwise `date-time`.
func handleFormatConstraint(key, value string) []OpenAPIConstraint {
	if key == validatorDatetime {
		if value == "" || value == boolTrueString {
			return []OpenAPIConstraint{{Name: constraintFormat, Value: formatDateTime}}
		}
		return []OpenAPIConstraint{{Name: constraintFormat, Value: datetimeFormat(value)}}
	}
	if format, ok := formatTagMap[key]; ok {
		return []OpenAPIConstraint{{Name: constraintFormat, Value: format}}
	}
	return nil
}

// stringContentPatterns maps boolean string-content tags to a canonical anchored
// regex. JSON-Schema `pattern` is documentation-grade here (kin-openapi/redocly
// accept arbitrary regex), so these mirror validator/v10 semantics closely.
var stringContentPatterns = map[string]string{
	"alpha":    `^[a-zA-Z]+$`,
	"alphanum": `^[a-zA-Z0-9]+$`,
	"numeric":  `^[-+]?[0-9]+(?:\.[0-9]+)?$`,
	"hexcolor": `^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`,
	"e164":     `^\+[1-9]\d{1,14}$`,
}

// handlePatternFormatConstraint maps string-content tags to a `pattern`. Boolean
// tags (alpha/alphanum/numeric/hexcolor/e164) use a fixed anchored regex;
// value-bearing tags (contains/startswith/endswith) build an (un)anchored pattern
// from the QuoteMeta-escaped literal. Bare `ip` is intentionally left
// unconstrained: there is no single OpenAPI format for "IPv4 or IPv6" and a
// correct combined regex is huge/error-prone, so we document the type as string
// with no pattern rather than over- or mis-constrain it (covered by a test).
func handlePatternFormatConstraint(key, value string) []OpenAPIConstraint {
	if p, ok := stringContentPatterns[key]; ok {
		return []OpenAPIConstraint{{Name: constraintPattern, Value: p}}
	}
	if value == "" {
		return nil
	}
	switch key {
	case "contains":
		return []OpenAPIConstraint{{Name: constraintPattern, Value: regexp.QuoteMeta(value)}}
	case "startswith":
		return []OpenAPIConstraint{{Name: constraintPattern, Value: "^" + regexp.QuoteMeta(value)}}
	case "endswith":
		return []OpenAPIConstraint{{Name: constraintPattern, Value: regexp.QuoteMeta(value) + "$"}}
	}
	return nil
}

// handleMinConstraint maps 'min' to minLength (strings) or minimum (numbers).
func handleMinConstraint(key, value, effKind string) []OpenAPIConstraint {
	if key != "min" {
		return nil
	}
	if isEffectiveString(effKind) {
		if length, err := strconv.Atoi(value); err == nil {
			return []OpenAPIConstraint{{Name: constraintMinLength, Value: length}}
		}
	} else if isEffectiveNumeric(effKind) {
		//nolint:S8148 // NOSONAR: invalid validation tag values are silently skipped
		if minVal, err := parseNumeric(value); err == nil {
			return []OpenAPIConstraint{{Name: constraintMinimum, Value: minVal}}
		}
	}
	return nil
}

// handleMaxConstraint maps 'max' to maxLength (strings) or maximum (numbers).
func handleMaxConstraint(key, value, effKind string) []OpenAPIConstraint {
	if key != "max" {
		return nil
	}
	if isEffectiveString(effKind) {
		if length, err := strconv.Atoi(value); err == nil {
			return []OpenAPIConstraint{{Name: constraintMaxLength, Value: length}}
		}
	} else if isEffectiveNumeric(effKind) {
		//nolint:S8148 // NOSONAR: invalid validation tag values are silently skipped
		if maxVal, err := parseNumeric(value); err == nil {
			return []OpenAPIConstraint{{Name: constraintMaximum, Value: maxVal}}
		}
	}
	return nil
}

// handleLenConstraint maps 'len' on a string to an exact length (minLength == maxLength).
func handleLenConstraint(key, value, effKind string) []OpenAPIConstraint {
	if key != "len" || !isEffectiveString(effKind) {
		return nil
	}
	length, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return []OpenAPIConstraint{
		{Name: constraintMinLength, Value: length},
		{Name: constraintMaxLength, Value: length},
	}
}

// handleNumericComparison maps gt/gte/lt/lte: range constraints for numerics, and
// minLength/maxLength for strings (a string comparison constrains its length).
func handleNumericComparison(key, value, effKind string) []OpenAPIConstraint {
	if isEffectiveString(effKind) {
		return handleStringLengthComparison(key, value)
	}
	if !isEffectiveNumeric(effKind) {
		return nil
	}
	numVal, err := parseNumeric(value)
	if err != nil {
		return nil
	}
	switch key {
	case "gt":
		return []OpenAPIConstraint{{Name: constraintMinimum, Value: numVal}, {Name: constraintExclusiveMinimum, Value: true}}
	case "gte":
		return []OpenAPIConstraint{{Name: constraintMinimum, Value: numVal}}
	case "lt":
		return []OpenAPIConstraint{{Name: constraintMaximum, Value: numVal}, {Name: constraintExclusiveMaximum, Value: true}}
	case "lte":
		return []OpenAPIConstraint{{Name: constraintMaximum, Value: numVal}}
	}
	return nil
}

// handleStringLengthComparison maps gt/gte/lt/lte on a string field to
// minLength/maxLength (gt=N -> minLength N+1, lt=N -> maxLength N-1), clamped to
// non-negative so the emitted bound stays valid OpenAPI.
func handleStringLengthComparison(key, value string) []OpenAPIConstraint {
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	switch key {
	case "gt":
		return []OpenAPIConstraint{{Name: constraintMinLength, Value: clampNonNeg(n + 1)}}
	case "gte":
		return []OpenAPIConstraint{{Name: constraintMinLength, Value: clampNonNeg(n)}}
	case "lt":
		return []OpenAPIConstraint{{Name: constraintMaxLength, Value: clampNonNeg(n - 1)}}
	case "lte":
		return []OpenAPIConstraint{{Name: constraintMaxLength, Value: clampNonNeg(n)}}
	}
	return nil
}

func clampNonNeg(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// handleSliceCardinality maps min/max/len on a slice field to minItems/maxItems.
func handleSliceCardinality(key, value string) []OpenAPIConstraint {
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	switch key {
	case "min":
		return []OpenAPIConstraint{{Name: constraintMinItems, Value: n}}
	case "max":
		return []OpenAPIConstraint{{Name: constraintMaxItems, Value: n}}
	case "len":
		return []OpenAPIConstraint{{Name: constraintMinItems, Value: n}, {Name: constraintMaxItems, Value: n}}
	}
	return nil
}

// handleMapCardinality maps min/max/len on a map field to minProperties/
// maxProperties (entry-count cardinality). Mirrors handleSliceCardinality.
func handleMapCardinality(key, value string) []OpenAPIConstraint {
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	switch key {
	case "min":
		return []OpenAPIConstraint{{Name: constraintMinProperties, Value: n}}
	case "max":
		return []OpenAPIConstraint{{Name: constraintMaxProperties, Value: n}}
	case "len":
		return []OpenAPIConstraint{{Name: constraintMinProperties, Value: n}, {Name: constraintMaxProperties, Value: n}}
	}
	return nil
}

// handleEnumConstraint maps 'oneof' to an enum array, numeric-coercing values for
// numeric fields. Tokenization is quote-aware so single-quoted multi-word values
// (oneof='New York' 'Los Angeles') stay intact.
func handleEnumConstraint(key, value, effKind string) []OpenAPIConstraint {
	if key != validatorOneOf {
		return nil
	}
	tokens := tokenizeOneOf(value)
	if len(tokens) == 0 {
		return nil
	}
	return []OpenAPIConstraint{{Name: constraintEnum, Value: coerceEnum(tokens, effKind)}}
}

// handleEqConstraint maps 'eq=<v>' to a single-element enum (the cleanest OpenAPI
// expression of equality). 'ne' has no clean scalar representation and is dropped.
func handleEqConstraint(key, value, effKind string) []OpenAPIConstraint {
	if key != "eq" {
		return nil
	}
	return []OpenAPIConstraint{{Name: constraintEnum, Value: coerceEnum([]string{value}, effKind)}}
}

// coerceEnum converts enum tokens to []any, parsing numerics for numeric fields.
func coerceEnum(tokens []string, effKind string) []any {
	out := make([]any, len(tokens))
	for i, v := range tokens {
		if isEffectiveNumeric(effKind) {
			//nolint:S8148 // NOSONAR: non-numeric tokens fall back to the string value
			if num, err := parseNumeric(v); err == nil {
				out[i] = num
				continue
			}
		}
		out[i] = v
	}
	return out
}

// tokenizeOneOf splits a oneof value on spaces outside single quotes, stripping the
// quotes, so "'New York' 'Los Angeles'" -> ["New York", "Los Angeles"] and
// "active pending" -> ["active", "pending"].
func tokenizeOneOf(s string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\'':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			flush()
		default:
			b.WriteByte(c)
		}
	}
	flush()
	return out
}

// datetimeFormat inspects a Go time layout: a layout with no clock/time tokens is
// date-only (-> "date"); anything with a clock/zone token is "date-time".
func datetimeFormat(layout string) string {
	for _, tok := range []string{"15", "03", "3:", "04", "05", ".000", "PM", "pm", "Z07", "-07", "-0700"} {
		if strings.Contains(layout, tok) {
			return formatDateTime
		}
	}
	return formatDate
}

// handlePatternConstraint maps the 'regexp' tag to an OpenAPI pattern.
func handlePatternConstraint(key, value string) []OpenAPIConstraint {
	if key != "regexp" {
		return nil
	}
	return []OpenAPIConstraint{{Name: constraintPattern, Value: value}}
}

// isStringType checks if the type is a string type
func isStringType(typeName string) bool {
	return typeName == goTypeString
}

// isIntegerType reports whether the Go type name is a signed/unsigned integer.
func isIntegerType(typeName string) bool {
	switch typeName {
	case "int", "int8", "int16", "int32", goTypeInt64,
		"uint", "uint8", "uint16", "uint32", "uint64":
		return true
	}
	return false
}

// isFloatType reports whether the Go type name is a floating-point type.
func isFloatType(typeName string) bool {
	return typeName == goTypeFloat32 || typeName == goTypeFloat64
}

// isNumericType checks if the type is a numeric type (integer or float).
func isNumericType(typeName string) bool {
	return isIntegerType(typeName) || isFloatType(typeName)
}

// parseNumeric converts a string to a numeric value (int or float)
func parseNumeric(value string) (any, error) {
	// Try parsing as integer first
	//nolint:S8148 // NOSONAR: Parse error intentionally falls through to float parsing
	if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
		return intVal, nil
	}

	// Fall back to float
	return strconv.ParseFloat(value, 64)
}
