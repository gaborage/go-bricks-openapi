package spectest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// anonParityDecls declares the named types over anonymous structs the
// anonymous payload parity rows use. It is its own file because the parity
// module imports time only under the alias t.
const anonParityDecls = `package parity

import "time"

type ARows []struct {
	A int ` + "`json:\"a\"`" + `
}

type MRows []struct {
	time.Time
	Z int
}
`

const anonParityA = "struct{ A int `json:\"a\"` }"

// anonParityRows are #110's payload parity rows for issue 118: each
// anonymous struct payload, wrapped, bare, raw or behind pointers, against a
// field of its type (its pointed-to type for a pointer payload).
func anonParityRows() []parityRow {
	rows := same(
		anonParityA, "[]"+anonParityA, "map[string]"+anonParityA, "[2]struct{ A int }", "[]*struct{ A int }",
		"struct{}", "struct{ In struct{ B string `json:\"b\"` } `json:\"in\"` }",
		"struct{ U User `json:\"u\"`; P *Address `json:\"p\"` }", "ARows", "MRows",
	)
	rows = append(rows,
		parityRow{payload: "*" + anonParityA, field: anonParityA, wrap: "r"},
		parityRow{payload: "**struct{ A int }", field: "struct{ A int }", wrap: "r"},
	)
	for _, ty := range []string{anonParityA, "[]struct{ A int }"} {
		rows = append(rows,
			parityRow{payload: ty, field: ty, wrap: "m"},
			parityRow{payload: ty, field: ty, wrap: "r", raw: true},
			parityRow{payload: ty, field: ty, wrap: "b"})
	}
	return rows
}

// TestAnonymousPayloadFieldParity extends #110's acceptance oracle to
// anonymous struct payloads: each emits exactly the inline schema of a
// struct field of its type, ignoring the envelope's description.
func TestAnonymousPayloadFieldParity(t *testing.T) {
	dir := t.TempDir()
	rows := anonParityRows()
	writeParityProject(t, dir, rows)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "anon.go"), []byte(anonParityDecls), 0o600))

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
	assert.Contains(t, schemas, "User")
	assert.Contains(t, schemas, "Address")
	assert.NotContains(t, schemas, "ARows")
	assert.NotContains(t, schemas, "MRows")
}
