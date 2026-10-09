package spectest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// parityDecls declares the named types of the named_payloads and
// named_payload_fallback fixtures, plus the func and chan types of U4.
const parityDecls = `package parity

import (
	"encoding/json"
	"time"
)

type Cents int64
type Chain Cents
type Alias = int64
type Dur time.Duration
type Flag byte
type Status string
type Ratio float64
type Enabled bool
type Blob []byte
type Raw json.RawMessage
type Tags []string
type UserList []User
type Stamp = time.Time
type RawA = json.RawMessage
type AnyD any
type PC *int64
type Pair [2]int

type User struct {
	ID int64 ` + "`json:\"id\"`" + `
}

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

type Tier int

func (t Tier) MarshalText() ([]byte, error) { return nil, nil }

type PTier int

func (t *PTier) MarshalText() ([]byte, error) { return nil, nil }

type FlagV byte

func (f FlagV) MarshalJSON() ([]byte, error) { return nil, nil }

type Tree map[string]Tree
type Addr uintptr
type StampD time.Time
type Cx complex128
type Fn func()
type Ch chan int

type Level int

func (l Level) MarshalText() ([]byte, error) { return nil, nil }
func (l *Level) UnmarshalText(b []byte) error { return nil }

type MoneyT struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m MoneyT) MarshalText() ([]byte, error) { return nil, nil }
func (m *MoneyT) UnmarshalText(b []byte) error { return nil }

type WrapLevel struct {
	Level
	Note string ` + "`json:\"note\"`" + `
}

type Money struct {
	Amount int64 ` + "`json:\"amount\"`" + `
}

func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

type Stamped struct {
	time.Time
	Note string ` + "`json:\"note\"`" + `
}
`

// parityRow is one payload and the field type whose schema it must emit:
// the payload's type, or its pointed-to type when the payload or its
// resolved form is a pointer. wrap is "r" (Result), "m" (ResultWithMeta) or
// "b" (a bare return); raw registers the route WithRawResponse().
type parityRow struct {
	payload, field, wrap string
	raw                  bool
}

// same is a Result[T] row compared with a field of type T.
func same(types ...string) []parityRow {
	rows := make([]parityRow, 0, len(types))
	for _, ty := range types {
		rows = append(rows, parityRow{payload: ty, field: ty, wrap: "r"})
	}
	return rows
}

func parityRows() []parityRow {
	rows := same(
		// P rows.
		"Cents", "Chain", "Alias", "Dur", "Flag", "Status", "Ratio", "Enabled", "Width",
		"[]Cents", "[]*Cents", "[]Enabled", "[2]Cents", "Blob", "[]Flag", "Raw",
		"Tags", "b.Tags", "[]Tags", "[][]string", "UserList", "Stamp", "RawA", "AnyD", "Pair", "b.Cents",
		"map[string]int64", "map[string]Cents", "map[string][]Cents", "[]map[string]int64", "[][]Cents",
		"map[string]Address", "[]*byte", "[]*Flag", "[4]Flag",
		// F rows.
		"Tier", "PTier", "b.Tier", "[]Tier", "[]FlagV", "map[string]Tier", "Tree", "Addr", "uintptr",
		"map[string]uintptr", "[]uintptr", "decimal.Decimal", "t.Time", "[]decimal.Decimal",
		"map[string]decimal.Decimal", "StampD", "[]StampD", "Cx", "complex128", "[]error",
		// U4 rows.
		"map[string]Fn", "[][]Fn", "map[string]Ch",
		// Marshaler types (#111).
		"Level", "[]Level", "MoneyT", "WrapLevel", "Money", "[]Money", "Stamped",
	)
	for payload, field := range map[string]string{
		"*Cents": "Cents", "PC": "int64", "*PC": "int64", "*Tags": "[]string", "*UserList": "UserList",
		"*[]string": "[]string", "*[]byte": "[]byte", "*[4]byte": "[4]byte", "*interface{}": "interface{}",
		"*Tier": "Tier", "*Missing": "Missing", "*[]Fn": "[]Fn", "*MoneyT": "MoneyT", "*Money": "Money",
	} {
		rows = append(rows, parityRow{payload: payload, field: field, wrap: "r"})
	}
	for _, ty := range []string{"Cents", "UserList", "map[string]int64"} {
		rows = append(rows,
			parityRow{payload: ty, field: ty, wrap: "m"},
			parityRow{payload: ty, field: ty, wrap: "r", raw: true},
			parityRow{payload: ty, field: ty, wrap: "b"})
	}
	return rows
}

// writeParityProject writes one route /p<i> per row and a /fields route whose
// Fields struct holds each row's field type as F<i>.
func writeParityProject(t *testing.T, dir string, rows []parityRow) {
	t.Helper()
	var reg, handlers, fields strings.Builder
	for i, r := range rows {
		opt := ""
		if r.raw {
			opt = ", server.WithRawResponse()"
		}
		fmt.Fprintf(&reg, "\tserver.GET(hr, r, \"/p%d\", m.h%d%s)\n", i, i, opt)
		result := map[string]string{"r": "server.Result[%s]", "m": "server.ResultWithMeta[%s]", "b": "%s"}[r.wrap]
		result = fmt.Sprintf(result, r.payload)
		fmt.Fprintf(&handlers, "func (m *Module) h%d(ctx server.HandlerContext) (%s, server.IAPIError) {\n\tvar v %s\n\treturn v, nil\n}\n\n",
			i, result, result)
		fmt.Fprintf(&fields, "\tF%d %s `json:\"f%d\"`\n", i, r.field, i)
	}
	module := `package parity

import (
	t "time"

	"github.com/example/parity/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

var _ decimal.Decimal
var _ t.Time
var _ b.Cents

type Module struct{}

func (m *Module) Name() string                    { return "parity" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Fields struct {
` + fields.String() + `}

func (m *Module) fields(ctx server.HandlerContext) (server.Result[Fields], server.IAPIError) {
	return server.Result[Fields]{}, nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/fields", m.fields)
` + reg.String() + "}\n\n" + handlers.String()
	files := map[string]string{
		"go.mod":     "module github.com/example/parity\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks v0.53.0\n",
		"types.go":   parityDecls,
		"width_a.go": "//go:build linux\n\npackage parity\n\ntype Width int32\n",
		"width_b.go": "//go:build !linux\n\npackage parity\n\ntype Width int64\n",
		"module.go":  module,
		"b/b.go":     "package b\n\ntype Cents int64\n\ntype Tags []string\n\ntype Tier int\n\nfunc (t Tier) MarshalText() ([]byte, error) { return nil, nil }\n",
	}
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	}
}

// dig walks nested YAML maps by key.
func dig(t *testing.T, v any, keys ...string) any {
	t.Helper()
	for _, k := range keys {
		m, ok := v.(map[string]any)
		require.True(t, ok, "not a map at %q", k)
		v, ok = m[k]
		require.True(t, ok, "missing key %q", k)
	}
	return v
}

// TestPayloadFieldParity is #110's acceptance oracle: every payload (wrapped,
// ResultWithMeta, WithRawResponse or bare) emits exactly the schema of a
// struct field of its type (the pointed-to type for a pointer payload),
// ignoring the envelope's description, and no named non-struct type becomes a
// component.
func TestPayloadFieldParity(t *testing.T) {
	dir := t.TempDir()
	rows := parityRows()
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

	for _, name := range []string{"Cents", "Tags", "UserList", "Tier", "PTier", "Tree", "Addr", "StampD", "Cx", "Fn", "Ch"} {
		assert.NotContains(t, schemas, name)
	}
	assert.Contains(t, schemas, "User")
	assert.Contains(t, schemas, "Address")
}
