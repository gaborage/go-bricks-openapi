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

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
)

// oracleDecls declares the named types of the named_resolution fixture.
const oracleDecls = `package oracle

import (
	"encoding/json"
	"time"

	"github.com/example/oracle/domain"
	"github.com/google/uuid"
)

type User struct {
	Name string ` + "`json:\"name\"`" + `
}

type Address struct {
	City string ` + "`json:\"city\"`" + `
}

type Cents int64
type Flag byte
type U8 = uint8

type Tags []string
type TagsA = []string
type Chain Tags
type Attrs map[string]string
type Stamps []time.Time
type Ids []Cents

type UserList []User
type PAddr *Address
type PC *int64
type FlagB bool
type AnyD any
type Shaper interface{ Area() float64 }

type Pair [2]int
type Key [4]byte
type Arr [3]byte

type Blob []byte
type BlobA = []byte
type B2 Blob
type Flags []Flag
type Raw json.RawMessage
type RawOfRaw Raw

type Stamp = time.Time
type IDA = uuid.UUID
type RawA = json.RawMessage
type Amount json.Number

type Status int

func (s Status) MarshalText() ([]byte, error) { return nil, nil }

type StatusD Status

type Odd int

func (Odd) MarshalText() string { return "" }

type FlagU byte

func (f *FlagU) UnmarshalJSON(b []byte) error { return nil }

type Members []domain.Member
type PMember *domain.Member
type MemberMap map[string]domain.Member
`

const oracleModule = `package oracle

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "oracle" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) get(req OracleQuery, ctx server.HandlerContext) (server.Result[Oracle], server.IAPIError) {
	return server.NewResult(http.StatusOK, Oracle{}), nil
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/oracle", m.get)
}
`

// oracleCase is one Marshaler-free named type and its hand-expanded form.
type oracleCase struct {
	named, expanded string
	tag             string   // extra struct tag text (validate/example), shared by both fields
	positions       []string // nil: every oraclePositions entry
}

var oraclePositions = []string{"%s", "*%s", "[]%s", "map[string]%s", "[][]%s", "map[string][]%s", "[]map[string]%s"}

var oracleCases = []oracleCase{
	{named: "Tags", expanded: "[]string"},
	{named: "TagsA", expanded: "[]string"},
	{named: "Chain", expanded: "[]string"},
	{named: "Attrs", expanded: "map[string]string"},
	{named: "Stamps", expanded: "[]time.Time"},
	{named: "Ids", expanded: "[]int64"},
	{named: "Cents", expanded: "int64"},
	{named: "UserList", expanded: "[]User"},
	{named: "PAddr", expanded: "*Address"},
	{named: "PC", expanded: "*int64"},
	{named: "FlagB", expanded: "bool"},
	{named: "AnyD", expanded: "any"},
	{named: "Shaper", expanded: "interface{ Area() float64 }"},
	{named: "Pair", expanded: "[2]int"},
	{named: "Key", expanded: "[4]byte"},
	{named: "Arr", expanded: "[3]byte"},
	{named: "Flag", expanded: "byte"},
	{named: "U8", expanded: "uint8"},
	{named: "Blob", expanded: "[]byte"},
	{named: "BlobA", expanded: "[]byte"},
	{named: "B2", expanded: "[]byte"},
	{named: "Raw", expanded: "[]byte"},
	{named: "RawOfRaw", expanded: "[]byte"},
	{named: "Flags", expanded: "[]byte"},
	{named: "Stamp", expanded: "time.Time"},
	{named: "IDA", expanded: "uuid.UUID"},
	{named: "RawA", expanded: "json.RawMessage"},
	{named: "Amount", expanded: "string"},
	// A defined type drops Status's method, and Odd's MarshalText() string does
	// not count: both are Marshaler-free.
	{named: "StatusD", expanded: "int"},
	{named: "Odd", expanded: "int"},
	// The sibling-file context bug is exercised by the named_resolution fixture
	// and the analyzer tests; here the struct file imports domain for the
	// expanded fields, so the oracle checks equality only.
	{named: "Members", expanded: "[]domain.Member"},
	{named: "PMember", expanded: "*domain.Member"},
	{named: "MemberMap", expanded: "map[string]domain.Member"},
	{named: "Tags", expanded: "[]string", tag: ` validate:"min=1,dive,max=5"`},
	{named: "Attrs", expanded: "map[string]string", tag: ` validate:"min=1"`},
	{named: "Flags", expanded: "[]byte", tag: ` validate:"min=1,dive,max=3" example:"AQI="`},
	// FlagU only decodes through its own method, which clears it as a slice
	// element only (the byte-slice rule); elsewhere it is a Marshaler leaf
	// that warns, so it is a case only in the slice positions.
	{named: "FlagU", expanded: "byte", positions: []string{"[]%s", "[][]%s", "map[string][]%s"}},
}

// writeOracleProject writes the oracle project into dir: each case in each of
// its positions as a named field N<i>P<j> and an expanded field E<i>P<j>, and
// each case once as a named and an expanded query parameter.
func writeOracleProject(t *testing.T, dir string) {
	t.Helper()
	var body, query strings.Builder
	for i, c := range oracleCases {
		positions := c.positions
		if positions == nil {
			positions = oraclePositions
		}
		for j, p := range positions {
			fmt.Fprintf(&body, "\tN%dP%d %s `json:\"n%dp%d\"%s`\n", i, j, fmt.Sprintf(p, c.named), i, j, c.tag)
			fmt.Fprintf(&body, "\tE%dP%d %s `json:\"e%dp%d\"%s`\n", i, j, fmt.Sprintf(p, c.expanded), i, j, c.tag)
		}
		fmt.Fprintf(&query, "\tN%d %s `query:\"n%d\"`\n\tE%d %s `query:\"e%d\"`\n", i, c.named, i, i, c.expanded, i)
	}
	structs := "package oracle\n\nimport (\n\t\"encoding/json\"\n\t\"time\"\n\n" +
		"\t\"github.com/example/oracle/domain\"\n\t\"github.com/google/uuid\"\n)\n\n" +
		"var (\n\t_ json.RawMessage\n\t_ time.Time\n\t_ uuid.UUID\n\t_ domain.Member\n)\n\n" +
		"type Oracle struct {\n" + body.String() + "}\n\ntype OracleQuery struct {\n" + query.String() + "}\n"
	files := map[string]string{
		"go.mod":                          "module github.com/example/oracle\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks v0.53.0\n",
		"module.go":                       oracleModule,
		"types.go":                        oracleDecls,
		"oracle.go":                       structs,
		filepath.Join("domain", "dom.go"): "package domain\n\ntype Member struct {\n\tName string `json:\"name\"`\n}\n",
	}
	for name, src := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	}
}

// TestNamedResolutionOracle is the analyzer-plus-generator guard of #109: each
// Marshaler-free local named type emits exactly what its hand-expanded type
// emits, in every field position and as a query parameter, with no warning.
func TestNamedResolutionOracle(t *testing.T) {
	dir := t.TempDir()
	writeOracleProject(t, dir)

	spec, err := Generate(t.Context(), dir)
	require.NoError(t, err)
	require.NoError(t, Validate(t.Context(), []byte(spec)))

	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `yaml:"name"`
				Schema any    `yaml:"schema"`
			} `yaml:"parameters"`
		} `yaml:"paths"`
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	props := doc.Components.Schemas["Oracle"].Properties
	require.NotEmpty(t, props, "Oracle component missing:\n%s", spec)
	params := map[string]any{}
	for _, p := range doc.Paths["/oracle"]["get"].Parameters {
		params[p.Name] = p.Schema
	}

	for i, c := range oracleCases {
		positions := c.positions
		if positions == nil {
			positions = oraclePositions
		}
		for j, p := range positions {
			n, e := fmt.Sprintf("n%dp%d", i, j), fmt.Sprintf("e%dp%d", i, j)
			require.Contains(t, props, n)
			require.Contains(t, props, e)
			assert.Equal(t, props[e], props[n], "%s%s in %s: named %s vs expanded %s",
				c.named, c.tag, p, fmt.Sprintf(p, c.named), fmt.Sprintf(p, c.expanded))
		}
		n, e := fmt.Sprintf("n%d", i), fmt.Sprintf("e%d", i)
		require.Contains(t, params, n)
		assert.Equal(t, params[e], params[n], "%s as a query parameter", c.named)
	}

	a := analyzer.New(dir)
	_, err = a.AnalyzeProject()
	require.NoError(t, err)
	assert.Empty(t, a.Warnings(t.Context()))
}
