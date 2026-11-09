package spectest

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/analyzer"
)

// TestGroupPathJoinRoutes checks the group_path_join fixture at the analyzer
// level: every route keeps the path go-bricks serves it at (none is dropped),
// and the analyzer raises no warning. (The raw Add route's "no resolved
// request/response type" note comes from the generate command, not here.)
func TestGroupPathJoinRoutes(t *testing.T) {
	a := analyzer.New(filepath.Join(testdataDir, "group_path_join"))
	project, err := a.AnalyzeProject()
	require.NoError(t, err)

	var got []string
	for i := range project.Modules {
		for j := range project.Modules[i].Routes {
			got = append(got, project.Modules[i].Routes[j].Path)
		}
	}
	sort.Strings(got)
	assert.Equal(t, []string{
		"/", "/api", "/api//x", "/api/add", "/api/apix", "/api/grp", "/api/v1/s",
		"/api/v3/w", "/api/x", "/api/x2", "/b", "/c", "/nolead", "/sg", "/v2/y",
	}, got)

	assert.Empty(t, a.Warnings(t.Context()))
}
