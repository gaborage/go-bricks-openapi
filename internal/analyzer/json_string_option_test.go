package analyzer

import (
	"go/ast"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// TestParseStructTagsJSONStringOption pins how the json tag's ",string"
// option is read (#115): as an exact, case-sensitive option anywhere after the
// name, with or without a name, and never from the name itself. Both
// encoding/json engines match options this way (verified on Go 1.27 with and
// without GOEXPERIMENT=nojsonv2): `json:"a, string"` and `json:"e,STRING"`
// marshal unquoted, and `json:"string"` names the field "string".
func TestParseStructTagsJSONStringOption(t *testing.T) {
	a := New("")
	cases := map[string]bool{
		`json:"i,string"`:           true,
		`json:"u,omitempty,string"`: true,
		`json:"u,string,omitempty"`: true,
		`json:",string"`:            true,
		`json:"i" query:"i"`:        false,
		`json:"i,omitempty"`:        false,
		`json:"string"`:             false,
		`json:"i,stringify"`:        false,
		`json:"a, string"`:          false,
		`json:"e,STRING"`:           false,
		`validate:"string"`:         false,
		``:                          false,
	}
	for tag, want := range cases {
		assert.Equal(t, want, a.parseStructTags(tag).jsonString, tag)
	}
	assert.Equal(t, "", a.parseStructTags(`json:",string"`).jsonName, "an empty name keeps the Go field name")
	assert.Equal(t, "u", a.parseStructTags(`json:"u,omitempty,string"`).jsonName)
}

// TestParseFieldTagsCopiesJSONStringOption pins that the option reaches the
// FieldInfo, where the generator reads it; a field without it stays false.
func TestParseFieldTagsCopiesJSONStringOption(t *testing.T) {
	a := New("")
	var quoted, plain models.FieldInfo
	a.parseFieldTags(&quoted, &ast.BasicLit{Value: "`json:\"i,string\" validate:\"min=1\"`"})
	a.parseFieldTags(&plain, &ast.BasicLit{Value: "`json:\"i\" validate:\"min=1\"`"})
	assert.True(t, quoted.JSONString)
	assert.Equal(t, "i", quoted.JSONName)
	assert.Equal(t, "1", quoted.Constraints["min"], "the analyzer still records the validate rules")
	assert.False(t, plain.JSONString)
}

// TestPointerAliasBit pins FieldInfo.PointerAlias (issue 154): a ",string"
// field's own type is an alias chain, followed through aliases only, that
// ends at a *T type literal. Both encoding/json engines quote such a field,
// so the generator does. A defined type before the literal (PA2 = PI, or PI
// itself), a second pointer level (*PA), build-tagged declarations that
// disagree on being such an alias, a type from outside the module, a cyclic
// chain (invalid Go, but it parses) and a field without ",string" are false.
func TestPointerAliasBit(t *testing.T) {
	variant := func(constraint, decl string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\n" + decl + "\n"
	}
	decls := "type Cents int64\ntype PA = *int64\ntype PI *int64\ntype PA2 = PI\ntype PAA = PA\ntype PC = *Cents\ntype PQ = b.PA\n" +
		"type CA = CB\ntype CB = CA\n"
	rows := "\tAlias PA `json:\"alias,string\"`\n\tAliasAl PAA `json:\"aliasAl,string\"`\n\tAliasC PC `json:\"aliasC,string\"`\n" +
		"\tQual b.PA `json:\"qual,string\"`\n\tQualAl PQ `json:\"qualAl,string\"`\n\tQualDef b.PI `json:\"qualDef,string\"`\n" +
		"\tDefined PI `json:\"defined,string\"`\n\tAliasDef PA2 `json:\"aliasDef,string\"`\n\tPtrAlias *PA `json:\"ptrAlias,string\"`\n" +
		"\tTagged PT `json:\"tagged,string\"`\n\tTaggedU PU `json:\"taggedU,string\"`\n\tPlain PA `json:\"plain\"`\n\tPtr *int64 `json:\"ptr,string\"`\n\tNum int64 `json:\"num,string\"`\n" +
		"\tDur time.Duration `json:\"dur,string\"`\n\tCycle CA `json:\"cycle,string\"`\n"
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                            resolveGoMod,
		filepath.Join("b", "b.go"):          "package b\n\ntype PI *int64\ntype PA = *int64\n",
		filepath.Join("mod", "module.go"):   rowsModule("\t\"time\"\n\n\t\"github.com/example/app/b\"\n", decls, rows),
		filepath.Join("mod", "t_linux.go"):  variant("linux", "type PT = *int64"),
		filepath.Join("mod", "t_darwin.go"): variant("darwin", "type PT *int64"),
		filepath.Join("mod", "u_darwin.go"): variant("darwin", "type PU = *int64"),
		filepath.Join("mod", "u_linux.go"):  variant("linux", "type PU *int64"),
	})
	ti := a.typeRegistry["Rows"]
	require.NotNil(t, ti)
	got := map[string]bool{}
	for i := range ti.Fields {
		got[ti.Fields[i].Name] = ti.Fields[i].PointerAlias
	}
	want := map[string]bool{
		"Alias": true, "AliasAl": true, "AliasC": true, "Qual": true, "QualAl": true,
		"QualDef": false, "Defined": false, "AliasDef": false, "PtrAlias": false,
		"Tagged": false, "Plain": false, "Ptr": false, "Num": false,
		"TaggedU": false, "Dur": false, "Cycle": false,
	}
	assert.Equal(t, want, got)
}
