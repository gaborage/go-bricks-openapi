package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildTaggedAgreeingChainIsLinear pins that comparing build-tagged
// copies does not re-walk a chain once per copy at every hop: eight
// mutually exclusive files each declaring T0 = T1 … T7 = T8 and the struct
// T8 agree, so T0 is a $ref with no warning, and the analysis takes the
// time of a few walks of the chain, not 8^9 of them.
func TestBuildTaggedAgreeingChainIsLinear(t *testing.T) {
	files := map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): btagModule("\tT T0 `json:\"t\"`\n",
			[][2]string{{"/resp", "server.Result[Resp]"}, {"/t", "server.Result[T0]"}}, "T0"),
	}
	var chain strings.Builder
	for i := range 8 {
		fmt.Fprintf(&chain, "type T%d = T%d\n\n", i, i+1)
	}
	chain.WriteString("type T8 struct {\n\tA int `json:\"a\"`\n}\n")
	for i := range 8 {
		files[filepath.Join("mod", fmt.Sprintf("t%d.go", i))] = fmt.Sprintf("//go:build tag%d\n\npackage mod\n\n%s", i, chain.String())
	}
	start := time.Now()
	a, routes := analyzeProjectRoutes(t, files)
	assert.Less(t, time.Since(start), 10*time.Second)
	assert.Equal(t, "$T0", renders(t, a.typeRegistry["Resp"])["t"])
	assert.Empty(t, a.Warnings(t.Context()))
	resp := routeByPath(t, routes, "/t").Response
	require.NotNil(t, resp)
	assert.Equal(t, "T0", resp.Name)
}

// TestBuildTaggedCopiesSkipExcludedFiles pins that a declaration in a file
// no build compiles (a name starting with _ or ., or //go:build ignore) is
// no build-tagged copy: beside it, Item, a struct, keeps its component with
// no warning, as it did before copies were compared. A constraint other
// than ignore is not evaluated, so a //go:build linux copy still counts.
func TestBuildTaggedCopiesSkipExcludedFiles(t *testing.T) {
	const item = "package mod\n\ntype Item struct {\n\tA int `json:\"a\"`\n}\n\ntype Other struct {\n\tB int `json:\"b\"`\n}\n"
	const copyDecl = "package mod\n\ntype Item Other\n"
	cases := []struct {
		name, file, src, want string
		warned                bool
	}{
		{"underscore file", "_gen.go", copyDecl, "$Item", false},
		{"dot file", ".tmp.go", copyDecl, "$Item", false},
		{"ignore-tagged file", "gen.go", "//go:build ignore\n\n" + copyDecl, "$Item", false},
		{"linux-tagged file", "item_linux.go", "//go:build linux\n\n" + copyDecl, "Item", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, _ := analyzeProjectRoutes(t, map[string]string{
				"go.mod":                     resolveGoMod,
				filepath.Join("mod", "a.go"): item,
				filepath.Join("mod", c.file): c.src,
				filepath.Join("mod", "module.go"): btagModule("\tI Item `json:\"i\"`\n",
					[][2]string{{"/resp", "server.Result[Resp]"}}, ""),
			})
			assert.Equal(t, c.want, renders(t, a.typeRegistry["Resp"])["i"])
			if !c.warned {
				assert.Empty(t, a.Warnings(t.Context()))
				assert.Contains(t, a.typeRegistry, "Item")
				return
			}
			assert.Len(t, a.Warnings(t.Context()), 1)
		})
	}
}

// TestLocalTypeDeclsKeepsResolvedFile pins that the file a name is resolved
// from is always searched, even when no build compiles it.
func TestLocalTypeDeclsKeepsResolvedFile(t *testing.T) {
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "_x.go", "package x\n\ntype W struct{}\n")
	decls := a.localTypeDecls("W", file, path)
	require.Len(t, decls, 1)
	assert.Equal(t, path, decls[0].path)
}
