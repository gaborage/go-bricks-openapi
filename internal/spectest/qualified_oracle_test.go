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

// qualifiedOracleModule adds the /qualified route to the oracle project.
const qualifiedOracleModule = `package oracle

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

func (m *Module) getQualified(req QualifiedQuery, ctx server.HandlerContext) (server.Result[Qualified], server.IAPIError) {
	return server.NewResult(http.StatusOK, Qualified{}), nil
}
`

// writeQualifiedOracleProject writes the oracle project plus a copy of its
// declarations as package q, and a Qualified struct holding each oracle case
// as q.<named> (Q<i>P<j>) beside its local twin (L<i>P<j>) in each of the
// case's positions, and once each as a query parameter.
func writeQualifiedOracleProject(t *testing.T, dir string) {
	t.Helper()
	writeOracleProject(t, dir)
	var body, query strings.Builder
	for i, c := range oracleCases {
		positions := c.positions
		if positions == nil {
			positions = oraclePositions
		}
		for j, p := range positions {
			fmt.Fprintf(&body, "\tQ%dP%d %s `json:\"q%dp%d\"%s`\n", i, j, fmt.Sprintf(p, "q."+c.named), i, j, c.tag)
			fmt.Fprintf(&body, "\tL%dP%d %s `json:\"l%dp%d\"%s`\n", i, j, fmt.Sprintf(p, c.named), i, j, c.tag)
		}
		fmt.Fprintf(&query, "\tQ%d q.%s `query:\"q%d\"`\n\tL%d %s `query:\"l%d\"`\n", i, c.named, i, i, c.named, i)
	}
	files := map[string]string{
		filepath.Join("q", "types.go"): strings.Replace(oracleDecls, "package oracle", "package q", 1),
		"qualified.go": "package oracle\n\nimport \"github.com/example/oracle/q\"\n\n" +
			"type Qualified struct {\n" + body.String() + "}\n\ntype QualifiedQuery struct {\n" + query.String() + "}\n",
		"qualified_module.go": qualifiedOracleModule,
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "q"), 0o750))
	for name, src := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600), name)
	}
	mod := filepath.Join(dir, "module.go")
	src, err := os.ReadFile(mod)
	require.NoError(t, err)
	const get = `server.GET(hr, r, "/oracle", m.get)`
	patched := strings.Replace(string(src), get, get+"\n\tserver.GET(hr, r, \"/qualified\", m.getQualified)", 1)
	require.NotEqual(t, string(src), patched, "the oracle module registers /oracle")
	require.NoError(t, os.WriteFile(mod, []byte(patched), 0o600))
}

// derefOnce replaces every {$ref: X} in v with component X, so fields that
// reference equal components under different names (q's User is QUser beside
// the oracle's own User) compare equal: a struct leaf's component name is the
// one allowed difference between a qualified type and its local twin.
func derefOnce(v any, schemas map[string]any) any {
	switch t := v.(type) {
	case map[string]any:
		if ref, ok := t["$ref"].(string); ok {
			return schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
		}
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = derefOnce(e, schemas)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = derefOnce(e, schemas)
		}
		return out
	default:
		return v
	}
}

// TestQualifiedNamedResolutionOracle is #100's parity guard: each
// Marshaler-free named type declared in another package of the module (q, a
// copy of the oracle's declarations) emits what the same declaration made
// locally emits, in every position and as a query parameter, apart from a
// struct leaf's component name, with no warning.
func TestQualifiedNamedResolutionOracle(t *testing.T) {
	dir := t.TempDir()
	writeQualifiedOracleProject(t, dir)

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
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(spec), &doc))
	schemas := doc.Components.Schemas
	qualified, ok := schemas["Qualified"].(map[string]any)
	require.True(t, ok, "Qualified component missing:\n%s", spec)
	props, ok := qualified["properties"].(map[string]any)
	require.True(t, ok)
	params := map[string]any{}
	for _, p := range doc.Paths["/qualified"]["get"].Parameters {
		params[p.Name] = p.Schema
	}

	for i, c := range oracleCases {
		positions := c.positions
		if positions == nil {
			positions = oraclePositions
		}
		for j, p := range positions {
			q, l := fmt.Sprintf("q%dp%d", i, j), fmt.Sprintf("l%dp%d", i, j)
			require.Contains(t, props, q)
			require.Contains(t, props, l)
			assert.Equal(t, derefOnce(props[l], schemas), derefOnce(props[q], schemas), "%s%s in %s", c.named, c.tag, p)
		}
		q, l := fmt.Sprintf("q%d", i), fmt.Sprintf("l%d", i)
		require.Contains(t, params, q)
		assert.Equal(t, derefOnce(params[l], schemas), derefOnce(params[q], schemas), "%s as a query parameter", c.named)
	}

	a := analyzer.New(dir)
	_, err = a.AnalyzeProject()
	require.NoError(t, err)
	assert.Empty(t, a.Warnings(t.Context()))
}
