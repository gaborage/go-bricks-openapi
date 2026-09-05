package generator

import (
	"cmp"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

const (
	// Validator tag values
	validatorOneOf    = "oneof"
	validatorDatetime = "datetime"
	validatorRegexp   = "regexp"

	// OpenAPI format values
	formatEmail = "email"
	formatDate  = "date"
	formatByte  = "byte"
	formatIPv4  = "ipv4"

	// Validate-tag vocabulary the mapper reads. The analyzer keeps its own
	// copies; these are the generator-side values.
	constraintRequired = "required"
	boolTrueString     = "true"
)

// numericBound is one candidate or resolved numeric bound (minimum/maximum).
// Integer-valued bounds keep int64 precision so two distinct values above 2^53
// never collapse when compared; only a fractional bound compares as float64.
type numericBound struct {
	intVal    int64
	floatVal  float64
	isFloat   bool
	exclusive bool // the bound came from gt/lt (exclusiveMinimum/Maximum: true)
}

// boundFromParsed converts a parseNumeric result (int64 or float64) into a bound.
func boundFromParsed(v any, exclusive bool) numericBound {
	if f, ok := v.(float64); ok {
		return numericBound{floatVal: f, isFloat: true, exclusive: exclusive}
	}
	return numericBound{intVal: v.(int64), exclusive: exclusive} // parseNumeric only yields int64 or float64
}

func (b numericBound) float64Value() float64 {
	if b.isFloat {
		return b.floatVal
	}
	return float64(b.intVal)
}

// compareNumericBounds orders two bounds: int64 against int64 exactly, anything
// involving a float as float64. Returns -1/0/1.
func compareNumericBounds(a, b numericBound) int {
	if !a.isFloat && !b.isFloat {
		return cmp.Compare(a.intVal, b.intVal)
	}
	return cmp.Compare(a.float64Value(), b.float64Value())
}

// constraintSet is the typed image of one validate tag in OpenAPI vocabulary
// (CONTEXT.md: "Constraint set"). Zero value = no constraints. Bounds hold the
// most-restrictive candidate seen so far; format/pattern/enum hold the last
// value set (callers set them in sorted validator-key order, so precedence is
// "last sorted key wins", matching the retired last-writer-wins applicators).
type constraintSet struct {
	format                       string
	pattern                      string
	enum                         []any
	minLength, maxLength         *int
	minItems, maxItems           *int
	minProperties, maxProperties *int
	minimum, maximum             *numericBound
}

func (s *constraintSet) setFormat(v string)  { s.format = v }
func (s *constraintSet) setPattern(v string) { s.pattern = v }
func (s *constraintSet) setEnum(v []any)     { s.enum = v }

// keepLarger/keepSmaller implement most-restrictive for integer keywords:
// validator/v10 enforces ALL rules, so the binding floor is the largest and
// the binding ceiling the smallest when distinct validator keys collapse onto
// one OpenAPI keyword (min & gte -> minimum, min/len/gt -> minLength, ...).
func keepLarger(dst **int, n int) {
	if *dst == nil || n > **dst {
		v := n
		*dst = &v
	}
}

func keepSmaller(dst **int, n int) {
	if *dst == nil || n < **dst {
		v := n
		*dst = &v
	}
}

func (s *constraintSet) setMinLength(n int)     { keepLarger(&s.minLength, n) }
func (s *constraintSet) setMaxLength(n int)     { keepSmaller(&s.maxLength, n) }
func (s *constraintSet) setMinItems(n int)      { keepLarger(&s.minItems, n) }
func (s *constraintSet) setMaxItems(n int)      { keepSmaller(&s.maxItems, n) }
func (s *constraintSet) setMinProperties(n int) { keepLarger(&s.minProperties, n) }
func (s *constraintSet) setMaxProperties(n int) { keepSmaller(&s.maxProperties, n) }

// mergeNumeric folds a candidate into the running winner. lower selects the
// larger value; !lower the smaller. On EQUAL value, exclusive beats inclusive
// (strictly more restrictive) — exclusivity is OR-ed, value kept.
func mergeNumeric(dst **numericBound, cand numericBound, lower bool) {
	if *dst == nil {
		c := cand
		*dst = &c
		return
	}
	ord := compareNumericBounds(cand, **dst)
	switch {
	case ord == 0:
		(*dst).exclusive = (*dst).exclusive || cand.exclusive
	case (lower && ord > 0) || (!lower && ord < 0):
		c := cand
		*dst = &c
	}
}

func (s *constraintSet) setMinimum(b numericBound) { mergeNumeric(&s.minimum, b, true) }
func (s *constraintSet) setMaximum(b numericBound) { mergeNumeric(&s.maximum, b, false) }

// applyTo writes every set keyword onto prop and leaves unset ones alone — so
// a pre-stamped value (the uint minimum: 0) survives an empty set and is
// overwritten only by an explicit bound, exactly as the applicator loop did.
//
// The pattern != "" guard (as opposed to an unconditional write) is
// unobservable today because nothing pre-stamps Pattern; if a pre-stamp ever
// appears, an empty `regexp=` tag would no longer clear it.
func (s *constraintSet) applyTo(prop *OpenAPIProperty) {
	if s.format != "" {
		prop.Format = s.format
	}
	if s.pattern != "" {
		prop.Pattern = s.pattern
	}
	if s.enum != nil {
		prop.Enum = s.enum
	}
	copyInt(&prop.MinLength, s.minLength)
	copyInt(&prop.MaxLength, s.maxLength)
	copyInt(&prop.MinItems, s.minItems)
	copyInt(&prop.MaxItems, s.maxItems)
	copyInt(&prop.MinProperties, s.minProperties)
	copyInt(&prop.MaxProperties, s.maxProperties)
	if s.minimum != nil {
		prop.Minimum = floatPtr(s.minimum.float64Value())
		if s.minimum.exclusive {
			prop.ExclusiveMinimum = boolPtr(true)
		}
	}
	if s.maximum != nil {
		prop.Maximum = floatPtr(s.maximum.float64Value())
		if s.maximum.exclusive {
			prop.ExclusiveMaximum = boolPtr(true)
		}
	}
}

func copyInt(dst **int, src *int) {
	if src != nil {
		v := *src
		*dst = &v
	}
}

func boolPtr(b bool) *bool { return &b }

// constraintsFor converts validation constraints to a typed constraintSet.
// Takes the field type and constraints map, returns the OpenAPI-compatible set.
func constraintsFor(shape models.TypeShape, underlyingKind string, constraints map[string]string) *constraintSet {
	var set constraintSet

	// The old string form stripped exactly ONE leading "*" (TrimPrefix) — mirror it.
	base := shape
	if base.Kind == models.ShapePointer && base.Elem != nil {
		base = *base.Elem
	}
	// []byte/[]uint8 are well-known base64 string types, not arrays — treat them as
	// scalars (the well-known mapper already types them string/binary), not slices.
	// Their min/max are byte counts, which do NOT equal the base64-encoded character
	// length, so we deliberately drop length bounds on them rather than emit a wrong
	// minLength/maxLength. Map types are handled by a parallel branch below: their
	// effectiveKind is "" (neither string nor numeric), so min/max/len route to
	// minProperties/maxProperties (entry-count cardinality) rather than being dropped.
	byteSlice := base.Kind == models.ShapeSlice && base.Elem != nil &&
		(base.Elem.Name == goTypeByte || base.Elem.Name == goTypeUint8)
	isSlice := base.Kind == models.ShapeSlice && !byteSlice
	isMap := base.Kind == models.ShapeMap
	// base.Name is "" for every container, which effectiveKind classifies as
	// neither string nor numeric — exactly what the "[]string"/"map[..." strings
	// it used to receive did.
	effKind := effectiveKind(base.Name, underlyingKind)

	// Iterate keys in sorted order so the emitted constraints are deterministic.
	// Go map iteration is randomized, and distinct validator keys can collapse to
	// the SAME OpenAPI keyword (min & gte -> minimum; max & lt -> maximum; min/len/gt
	// -> minLength; etc.). Bound keywords (minimum/maximum/min*Length/*Items/
	// *Properties) resolve most-restrictive at set time, independent of processing
	// order. format/pattern/enum instead keep the LAST value constraintSet was
	// given, so a random map-iteration range would still make the emitted
	// format/pattern/enum nondeterministic for a field like
	// `validate:"email=true,uuid4=true"` without a stable key order. Sorting fixes
	// a stable precedence — "last sorted key wins" — for both cases (and stable
	// golden output).
	for _, key := range sortedKeys(constraints) {
		if key == constraintRequired {
			continue // handled at schema level
		}
		value := constraints[key]

		switch {
		case isSlice:
			applySliceCardinality(&set, key, value)
		case isMap:
			applyMapCardinality(&set, key, value)
		default:
			applyScalarConstraint(&set, key, value, effKind)
		}
	}

	return &set
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

// applyScalarConstraint routes a single (non-slice, non-map) constraint key to
// the first matching handler. Order matters: format/pattern tags are tried
// before the numeric/string handlers so a key is claimed by exactly one handler.
func applyScalarConstraint(s *constraintSet, key, value, effKind string) {
	handlers := []func() bool{
		func() bool { return applyFormatConstraint(s, key, value) },
		func() bool { return applyPatternFormatConstraint(s, key, value) },
		func() bool { return applyMinConstraint(s, key, value, effKind) },
		func() bool { return applyMaxConstraint(s, key, value, effKind) },
		func() bool { return applyLenConstraint(s, key, value, effKind) },
		func() bool { return applyNumericComparison(s, key, value, effKind) },
		func() bool { return applyEnumConstraint(s, key, value, effKind) },
		func() bool { return applyEqConstraint(s, key, value, effKind) },
		func() bool { return applyPatternConstraint(s, key, value) },
	}
	for _, h := range handlers {
		if h() {
			return
		}
	}
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
		return typeInteger
	case isFloatType(baseType):
		return typeNumber
	}
	return ""
}

func isEffectiveString(k string) bool  { return k == goTypeString }
func isEffectiveNumeric(k string) bool { return k == typeInteger || k == typeNumber }

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

// applyFormatConstraint maps boolean format tags to OpenAPI `format`. datetime is
// value-aware: a date-only layout maps to `date`, otherwise `date-time`.
func applyFormatConstraint(s *constraintSet, key, value string) bool {
	if key == validatorDatetime {
		if value == "" || value == boolTrueString {
			s.setFormat(formatDateTime)
		} else {
			s.setFormat(datetimeFormat(value))
		}
		return true
	}
	if format, ok := formatTagMap[key]; ok {
		s.setFormat(format)
		return true
	}
	return false
}

// patternAlpha is pulled out of the map below only because the generator package
// repeats this regex elsewhere; the other entries have no second occurrence.
const patternAlpha = `^[a-zA-Z]+$`

// stringContentPatterns maps boolean string-content tags to a canonical anchored
// regex. JSON-Schema `pattern` is documentation-grade here (kin-openapi/redocly
// accept arbitrary regex), so these mirror validator/v10 semantics closely.
var stringContentPatterns = map[string]string{
	"alpha":    patternAlpha,
	"alphanum": `^[a-zA-Z0-9]+$`,
	"numeric":  `^[-+]?[0-9]+(?:\.[0-9]+)?$`,
	"hexcolor": `^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`,
	"e164":     `^\+[1-9]\d{1,14}$`,
}

// applyPatternFormatConstraint maps string-content tags to a `pattern`. Boolean
// tags (alpha/alphanum/numeric/hexcolor/e164) use a fixed anchored regex;
// value-bearing tags (contains/startswith/endswith) build an (un)anchored pattern
// from the QuoteMeta-escaped literal. Bare `ip` is intentionally left
// unconstrained: there is no single OpenAPI format for "IPv4 or IPv6" and a
// correct combined regex is huge/error-prone, so we document the type as string
// with no pattern rather than over- or mis-constrain it (covered by a test).
func applyPatternFormatConstraint(s *constraintSet, key, value string) bool {
	if p, ok := stringContentPatterns[key]; ok {
		s.setPattern(p)
		return true
	}
	if value == "" {
		return false
	}
	switch key {
	case "contains":
		s.setPattern(regexp.QuoteMeta(value))
		return true
	case "startswith":
		s.setPattern("^" + regexp.QuoteMeta(value))
		return true
	case "endswith":
		s.setPattern(regexp.QuoteMeta(value) + "$")
		return true
	}
	return false
}

// applyMinConstraint maps 'min' to minLength (strings) or minimum (numbers).
func applyMinConstraint(s *constraintSet, key, value, effKind string) bool {
	if key != "min" {
		return false
	}
	if isEffectiveString(effKind) {
		if length, err := strconv.Atoi(value); err == nil {
			s.setMinLength(length)
			return true
		}
	} else if isEffectiveNumeric(effKind) {
		//nolint:S8148 // NOSONAR: invalid validation tag values are silently skipped
		if minVal, err := parseNumeric(value); err == nil {
			s.setMinimum(boundFromParsed(minVal, false))
			return true
		}
	}
	return false
}

// applyMaxConstraint maps 'max' to maxLength (strings) or maximum (numbers).
func applyMaxConstraint(s *constraintSet, key, value, effKind string) bool {
	if key != "max" {
		return false
	}
	if isEffectiveString(effKind) {
		if length, err := strconv.Atoi(value); err == nil {
			s.setMaxLength(length)
			return true
		}
	} else if isEffectiveNumeric(effKind) {
		//nolint:S8148 // NOSONAR: invalid validation tag values are silently skipped
		if maxVal, err := parseNumeric(value); err == nil {
			s.setMaximum(boundFromParsed(maxVal, false))
			return true
		}
	}
	return false
}

// applyLenConstraint maps 'len' on a string to an exact length (minLength == maxLength).
func applyLenConstraint(s *constraintSet, key, value, effKind string) bool {
	if key != "len" || !isEffectiveString(effKind) {
		return false
	}
	length, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	s.setMinLength(length)
	s.setMaxLength(length)
	return true
}

// applyNumericComparison maps gt/gte/lt/lte: range constraints for numerics, and
// minLength/maxLength for strings (a string comparison constrains its length).
func applyNumericComparison(s *constraintSet, key, value, effKind string) bool {
	if isEffectiveString(effKind) {
		return applyStringLengthComparison(s, key, value)
	}
	if !isEffectiveNumeric(effKind) {
		return false
	}
	numVal, err := parseNumeric(value)
	if err != nil {
		return false
	}
	switch key {
	case "gt":
		s.setMinimum(boundFromParsed(numVal, true))
	case "gte":
		s.setMinimum(boundFromParsed(numVal, false))
	case "lt":
		s.setMaximum(boundFromParsed(numVal, true))
	case "lte":
		s.setMaximum(boundFromParsed(numVal, false))
	default:
		return false
	}
	return true
}

// applyStringLengthComparison maps gt/gte/lt/lte on a string field to
// minLength/maxLength (gt=N -> minLength N+1, lt=N -> maxLength N-1), clamped to
// non-negative so the emitted bound stays valid OpenAPI.
func applyStringLengthComparison(s *constraintSet, key, value string) bool {
	n, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	switch key {
	case "gt":
		s.setMinLength(clampNonNeg(n + 1))
	case "gte":
		s.setMinLength(clampNonNeg(n))
	case "lt":
		s.setMaxLength(clampNonNeg(n - 1))
	case "lte":
		s.setMaxLength(clampNonNeg(n))
	default:
		return false
	}
	return true
}

func clampNonNeg(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// applySliceCardinality maps min/max/len on a slice field to minItems/maxItems.
func applySliceCardinality(s *constraintSet, key, value string) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return
	}
	switch key {
	case "min":
		s.setMinItems(n)
	case "max":
		s.setMaxItems(n)
	case "len":
		s.setMinItems(n)
		s.setMaxItems(n)
	}
}

// applyMapCardinality maps min/max/len on a map field to minProperties/
// maxProperties (entry-count cardinality). Mirrors applySliceCardinality.
func applyMapCardinality(s *constraintSet, key, value string) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return
	}
	switch key {
	case "min":
		s.setMinProperties(n)
	case "max":
		s.setMaxProperties(n)
	case "len":
		s.setMinProperties(n)
		s.setMaxProperties(n)
	}
}

// applyEnumConstraint maps 'oneof' to an enum array, numeric-coercing values for
// numeric fields. Tokenization is quote-aware so single-quoted multi-word values
// (oneof='New York' 'Los Angeles') stay intact.
func applyEnumConstraint(s *constraintSet, key, value, effKind string) bool {
	if key != validatorOneOf {
		return false
	}
	tokens := tokenizeOneOf(value)
	if len(tokens) == 0 {
		return false
	}
	s.setEnum(coerceEnum(tokens, effKind))
	return true
}

// applyEqConstraint maps 'eq=<v>' to a single-element enum (the cleanest OpenAPI
// expression of equality). 'ne' has no clean scalar representation and is dropped.
func applyEqConstraint(s *constraintSet, key, value, effKind string) bool {
	if key != "eq" {
		return false
	}
	s.setEnum(coerceEnum([]string{value}, effKind))
	return true
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

// applyPatternConstraint maps the 'regexp' tag to an OpenAPI pattern.
func applyPatternConstraint(s *constraintSet, key, value string) bool {
	if key != validatorRegexp {
		return false
	}
	s.setPattern(value)
	return true
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

// Go builtin type-name classification, kept generator-private. The analyzer
// holds the same lexical fact for the Shape decoder (builtins.go); the lists
// are duplicated rather than shared because they answer different questions
// and change only when Go's builtin type set does.

// isStringType checks if the type is a string type
func isStringType(typeName string) bool {
	return typeName == goTypeString
}

// isIntegerType reports whether the Go type name is a signed/unsigned integer.
func isIntegerType(typeName string) bool {
	switch typeName {
	case goTypeInt, goTypeInt8, goTypeInt16, goTypeInt32, goTypeInt64,
		goTypeUint, goTypeUint8, goTypeUint16, goTypeUint32, goTypeUint64:
		return true
	}
	return false
}

// isFloatType reports whether the Go type name is a floating-point type.
func isFloatType(typeName string) bool {
	return typeName == goTypeFloat32 || typeName == goTypeFloat64
}
