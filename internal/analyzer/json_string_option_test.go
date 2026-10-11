package analyzer

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"

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
