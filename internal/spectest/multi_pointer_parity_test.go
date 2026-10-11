package spectest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// multiPointerParityRows are #120's payload rows: a written root of two or
// more pointer levels, in every carrier, against a field of its fully
// pointed-to type (a payload root is never nullable).
func multiPointerParityRows() []parityRow {
	pairs := [][2]string{
		{"**int64", "int64"}, {"***int64", "int64"}, {"**string", "string"}, {"**Cents", "Cents"},
		{"**b.Cents", "b.Cents"}, {"**PC", "int64"}, {"**Address", "Address"}, {"***Address", "Address"},
		{"**[]string", "[]string"}, {"**[]Address", "[]Address"}, {"**map[string]int64", "map[string]int64"},
		{"**[2]Cents", "[2]Cents"}, {"**interface{}", "interface{}"}, {"**Tier", "Tier"}, {"**Missing", "Missing"},
	}
	rows := make([]parityRow, 0, len(pairs)+3)
	for _, p := range pairs {
		rows = append(rows, parityRow{payload: p[0], field: p[1], wrap: "r"})
	}
	return append(rows,
		parityRow{payload: "**int64", field: "int64", wrap: "m"},
		parityRow{payload: "**int64", field: "int64", wrap: "b"},
		parityRow{payload: "**Address", field: "Address", wrap: "r", raw: true})
}

// TestMultiPointerPayloadFieldParity extends #110's parity oracle to #120: a
// **T payload, in every carrier, emits exactly the schema of a field of type
// T, and no named non-struct type becomes a component.
func TestMultiPointerPayloadFieldParity(t *testing.T) {
	dir := t.TempDir()
	rows := multiPointerParityRows()
	writeParityProject(t, dir, rows)

	spec, err := Generate(t.Context(), dir)
	require.NoError(t, err)
	require.NoError(t, Validate(t.Context(), []byte(spec)))

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	schemas, ok := dig(t, doc, "components", "schemas").(map[string]any)
	require.True(t, ok)
	props, ok := dig(t, schemas, "Fields", "properties").(map[string]any)
	require.True(t, ok)

	for i, r := range rows {
		keys := []string{"paths", fmt.Sprintf("/p%d", i), "get", "responses", "200", "content", "application/json", "schema"}
		if !r.raw {
			keys = append(keys, "properties", "data")
		}
		payload, ok := dig(t, doc, keys...).(map[string]any)
		require.True(t, ok, r.payload)
		delete(payload, "description")
		assert.Equal(t, props[fmt.Sprintf("f%d", i)], payload, "payload %s (%s, raw %v) vs field %s", r.payload, r.wrap, r.raw, r.field)
	}
	for _, name := range []string{"Cents", "PC", "Tier"} {
		assert.NotContains(t, schemas, name)
	}
}
