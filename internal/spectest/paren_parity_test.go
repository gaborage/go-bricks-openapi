package spectest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// parenParityRows are the payload rows of issues 160 and 177: a payload
// written in parentheses, against a field of the same type without them (a
// pointer payload against its pointed-to type: a payload root is never
// nullable). (Address) is also checked in the ResultWithMeta and bare
// carriers.
func parenParityRows() []parityRow {
	pairs := [][2]string{
		{"(Address)", "Address"}, {"*(Address)", "Address"}, {"*(*Address)", "Address"},
		{"(*Address)", "Address"}, {"*([]Address)", "[]Address"}, {"(map[string]int64)", "map[string]int64"},
		{"[](Address)", "[]Address"}, {"(interface{})", "interface{}"}, {"(string)", "string"},
	}
	rows := make([]parityRow, 0, len(pairs)+2)
	for _, p := range pairs {
		rows = append(rows, parityRow{payload: p[0], field: p[1], wrap: "r"})
	}
	return append(rows,
		parityRow{payload: "(Address)", field: "Address", wrap: "m"},
		parityRow{payload: "(Address)", field: "Address", wrap: "b"})
}

// TestParenPayloadFieldParity extends #110's parity oracle to parenthesized
// payloads: each, in every carrier, emits exactly the schema of a field of
// its type written without the parentheses.
func TestParenPayloadFieldParity(t *testing.T) {
	dir := t.TempDir()
	rows := parenParityRows()
	writeParityProject(t, dir, rows)

	spec, err := Generate(t.Context(), dir)
	require.NoError(t, err)
	require.NoError(t, Validate(t.Context(), []byte(spec)))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	props, ok := dig(t, doc, "components", "schemas", "Fields", "properties").(map[string]any)
	require.True(t, ok)

	for i, r := range rows {
		keys := []string{"paths", fmt.Sprintf("/p%d", i), "get", "responses", "200", "content", "application/json", "schema", "properties", "data"}
		payload, ok := dig(t, doc, keys...).(map[string]any)
		require.True(t, ok, r.payload)
		delete(payload, "description")
		assert.Equal(t, props[fmt.Sprintf("f%d", i)], payload, "payload %s (%s) vs field %s", r.payload, r.wrap, r.field)
	}
}
