package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestNetTextTypesAreStrings pins net.IP, netip.Addr, netip.Prefix and
// netip.AddrPort as JSON strings with no type-derived format, as
// encoding/json writes them through their value MarshalText: bare, behind a
// pointer, as slice items and as map values.
func TestNetTextTypesAreStrings(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	str := &OpenAPIProperty{Type: typeString}
	for _, name := range []string{goTypeNetIP, goTypeNetipAddr, goTypeNetipPrefix, goTypeNetipAddrPort} {
		wk, ok := wellKnownShape(named(name))
		require.True(t, ok, name)
		assert.Equal(t, wellKnownType{typ: typeString}, wk, name)
		assert.Equal(t, str, gen.fieldInfoToProperty(&models.FieldInfo{Shape: named(name), JSONName: "v"}), name)
		assert.Equal(t, &OpenAPIProperty{Type: typeArray, Items: str},
			gen.fieldInfoToProperty(&models.FieldInfo{Shape: sliceOf(named(name)), JSONName: "v"}), name)
		assert.Equal(t, &OpenAPIProperty{Type: typeObject, AdditionalProperties: str},
			gen.fieldInfoToProperty(&models.FieldInfo{Shape: mapOf(prim(goTypeString), named(name)), JSONName: "v"}), name)
	}
}

// TestNetTextTypeValidateAsUUID pins that validate rules treat the text
// types as they treat uuid.UUID: length bounds are dropped (a named leaf has
// no effective kind), while format, enum and pattern tags still stamp their
// keyword, on the field and on dive items, and an example is kept as a
// string.
func TestNetTextTypeValidateAsUUID(t *testing.T) {
	gen := New(defaultTitle, defaultVersion, defaultDescription)
	field := func(name string, rules map[string]string) *models.FieldInfo {
		return &models.FieldInfo{Shape: named(name), JSONName: "v", Constraints: rules}
	}
	assert.Equal(t, &OpenAPIProperty{Type: typeString},
		gen.fieldInfoToProperty(field(goTypeNetIP, map[string]string{"min": "7", "max": "15"})), "length bounds dropped")
	assert.Equal(t, &OpenAPIProperty{Type: typeString, Format: formatIPv4},
		gen.fieldInfoToProperty(field(goTypeNetIP, map[string]string{formatIPv4: ""})), "a format tag stamps its format")
	oneOf := gen.fieldInfoToProperty(field(goTypeNetipAddr, map[string]string{validatorOneOf: "1.2.3.4 5.6.7.8"}))
	assert.Equal(t, []any{"1.2.3.4", "5.6.7.8"}, oneOf.Enum)
	ten := gen.fieldInfoToProperty(field(goTypeNetipPrefix, map[string]string{"startswith": "10."}))
	assert.Equal(t, `^10\.`, ten.Pattern)

	ex := gen.fieldInfoToProperty(&models.FieldInfo{Shape: named(goTypeNetIP), JSONName: "v", Example: "1.2.3.4"})
	assert.Equal(t, "1.2.3.4", ex.Example)

	items := gen.fieldInfoToProperty(&models.FieldInfo{
		Shape: sliceOf(named(goTypeNetipAddr)), JSONName: "v",
		Constraints: map[string]string{"max": "2"}, ElementConstraints: map[string]string{"ipv6": ""},
	})
	require.NotNil(t, items.Items)
	assert.Equal(t, &OpenAPIProperty{Type: typeString, Format: "ipv6"}, items.Items)
	require.NotNil(t, items.MaxItems)
	assert.Equal(t, 2, *items.MaxItems)
}
