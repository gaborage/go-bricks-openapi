package generator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestDatetimeLayoutFormat pins which validate:"datetime=<layout>" layouts
// map to an OpenAPI format. go-playground's datetime runs time.Parse(layout,
// v), so a format is emitted only when every value that layout accepts is
// what the format describes: `date` for exactly time.DateOnly, `date-time`
// for exactly time.RFC3339 or time.RFC3339Nano. Every other layout, strict
// subsets of RFC 3339 included, gets no format. A bare `datetime` parses with
// the empty layout (accepting only "") and datetime=true with the literal
// layout "true"; the analyzer stores both as "true", and neither gets a format.
func TestDatetimeLayoutFormat(t *testing.T) {
	tests := []struct {
		layout string
		want   string
	}{
		{time.DateOnly, formatDate},
		{time.RFC3339, formatDateTime},
		{time.RFC3339Nano, formatDateTime},
		// Date layouts that are not a full-date.
		{"2006-01", ""},
		{"01/02/2006", ""},
		{"2006", ""},
		{"02 Jan 2006", ""},
		// Clock layouts that are not RFC 3339.
		{"15:04", ""},
		{"3:04PM", ""},
		{time.DateTime, ""},
		{time.UnixDate, ""},
		// Strict subsets of RFC 3339: a literal Z rejects an offset, a fixed
		// .000 rejects a value with no fraction, -07:00 rejects Z.
		{"2006-01-02T15:04:05Z", ""},
		{"2006-01-02T15:04:05.000Z07:00", ""},
		{"2006-01-02T15:04:05-07:00", ""},
		// Bare datetime (the analyzer stores it as "true") and an empty value.
		{boolTrueString, ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.layout, func(t *testing.T) {
			var prop OpenAPIProperty
			constraintsFor(prim(goTypeString), map[string]string{validatorDatetime: tt.layout}).applyTo(&prop)
			assert.Equal(t, OpenAPIProperty{Format: tt.want}, prop)
		})
	}
}

// TestApplyFormatConstraintClaimsDatetime pins that datetime claims its key
// even when it has no format to write, so applyScalarRule never hands it to a
// later handler.
func TestApplyFormatConstraintClaimsDatetime(t *testing.T) {
	for _, value := range []string{"", boolTrueString, "15:04", "2006-01"} {
		var s constraintSet
		assert.True(t, applyFormatConstraint(&s, validatorDatetime, value), "datetime=%q must be claimed", value)
		assert.Empty(t, s.format, "datetime=%q must write no format", value)
	}
}

// TestDatetimeLayoutKeepsOtherFormat pins that a datetime layout with no
// format writes nothing, so it does not erase a format an earlier sorted key
// (here `date`) already set.
func TestDatetimeLayoutKeepsOtherFormat(t *testing.T) {
	var prop OpenAPIProperty
	constraintsFor(prim(goTypeString), map[string]string{formatDate: boolTrueString, validatorDatetime: "15:04"}).applyTo(&prop)
	assert.Equal(t, formatDate, prop.Format)
}

// TestFieldInfoToPropertyDatetimeLayoutExample pins the end-to-end effect
// through the fieldInfoToProperty wrapper: a field whose layout gets no format
// keeps an example that layout accepts, on the property and on dive items.
func TestFieldInfoToPropertyDatetimeLayoutExample(t *testing.T) {
	gen := New(defaultTitle, "1.0.0", defaultDescription)
	tests := []struct {
		name  string
		field models.FieldInfo
	}{
		{"time-only layout", models.FieldInfo{Shape: prim(goTypeString), Constraints: map[string]string{validatorDatetime: "15:04"}, Example: "10:30"}},
		{"year-month layout", models.FieldInfo{Shape: prim(goTypeString), Constraints: map[string]string{validatorDatetime: "2006-01"}, Example: "2024-01"}},
		// A bare datetime and datetime=true both reach the generator as "true".
		// With no format the example is not checked: it stays verbatim.
		{"bare datetime or datetime=true", models.FieldInfo{Shape: prim(goTypeString), Constraints: map[string]string{validatorDatetime: boolTrueString}, Example: "true"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop := gen.fieldInfoToProperty(&tt.field)
			assert.Equal(t, typeString, prop.Type)
			assert.Empty(t, prop.Format)
			assert.Equal(t, tt.field.Example, prop.Example)
		})
	}

	t.Run("dive items", func(t *testing.T) {
		field := models.FieldInfo{
			Shape:              sliceOf(prim(goTypeString)),
			ElementConstraints: map[string]string{validatorDatetime: "01/02/2006"},
		}
		prop := gen.fieldInfoToProperty(&field)
		require.NotNil(t, prop.Items)
		assert.Equal(t, typeString, prop.Items.Type)
		assert.Empty(t, prop.Items.Format)
	})
}

// TestFieldInfoToPropertyTimeTimeDatetimeLayout pins a time.Time field, pointer
// or dive item tagged with a layout that gets no format: the datetime rule
// writes nothing, so the field keeps its pre-stamped date-time and its example
// is checked as RFC 3339 (the old clock-token scan stamped date over it). The
// validator's datetime panics on non-string kinds, so the tag is unusable on
// such a field anyway.
func TestFieldInfoToPropertyTimeTimeDatetimeLayout(t *testing.T) {
	gen := New(defaultTitle, "1.0.0", defaultDescription)
	ym := map[string]string{validatorDatetime: "2006-01"}
	tests := []struct {
		name        string
		field       models.FieldInfo
		wantExample any
	}{
		{"time.Time", models.FieldInfo{Shape: named(goTypeTimeTime), Constraints: ym}, nil},
		{"*time.Time", models.FieldInfo{Shape: ptrOf(named(goTypeTimeTime)), Constraints: ym}, nil},
		{"RFC 3339 example kept", models.FieldInfo{Shape: named(goTypeTimeTime), Constraints: ym, Example: "2024-01-02T10:30:00Z"}, "2024-01-02T10:30:00Z"},
		{"full-date example dropped", models.FieldInfo{Shape: named(goTypeTimeTime), Constraints: ym, Example: "2024-01-02"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop := gen.fieldInfoToProperty(&tt.field)
			assert.Equal(t, formatDateTime, prop.Format)
			assert.Equal(t, tt.wantExample, prop.Example)
		})
	}

	t.Run("dive items", func(t *testing.T) {
		field := models.FieldInfo{Shape: sliceOf(named(goTypeTimeTime)), ElementConstraints: ym}
		prop := gen.fieldInfoToProperty(&field)
		require.NotNil(t, prop.Items)
		assert.Equal(t, formatDateTime, prop.Items.Format)
	})
}
