package analyzer

import (
	"go/ast"
	"go/parser"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseInDir parses src as a file named name inside dir (which need not hold
// it on disk) and returns the analyzer, the file and its path.
func parseInDir(t *testing.T, a *ProjectAnalyzer, dir, name, src string) (file *ast.File, path string) {
	t.Helper()
	path = filepath.Join(dir, name)
	file, err := parser.ParseFile(a.fileSet, path, src, 0)
	require.NoError(t, err)
	return file, path
}

// marshalerSetOf parses src in a fresh temp dir and returns what T's own
// methods say about its JSON form.
func marshalerSetOf(t *testing.T, src string) marshalerMethods {
	t.Helper()
	dir := t.TempDir()
	a := New(dir)
	file, path := parseInDir(t, a, dir, "t.go", src)
	return a.marshalerSet("T", newResolveCtx(file, path, false))
}

// TestMarshalerSetSevenMethods pins each of the seven encoding/json methods,
// on a value and on a pointer receiver, and which side it is on.
func TestMarshalerSetSevenMethods(t *testing.T) {
	cases := []struct {
		method string
		decl   string // the method after "func (<recv>) "
		encode bool
	}{
		{methodMarshalJSON, "MarshalJSON() ([]byte, error) { return nil, nil }", true},
		{methodMarshalText, "MarshalText() ([]byte, error) { return nil, nil }", true},
		{methodMarshalJSONTo, "MarshalJSONTo(enc *jsontext.Encoder) error { return nil }", true},
		{methodAppendText, "AppendText(b []byte) ([]byte, error) { return b, nil }", true},
		{methodUnmarshalJSON, "UnmarshalJSON(b []byte) error { return nil }", false},
		{methodUnmarshalText, "UnmarshalText(b []uint8) error { return nil }", false},
		{methodUnmarshalJSONFrom, "UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }", false},
	}
	for _, tc := range cases {
		for _, recv := range []string{"t T", "t *T"} {
			t.Run(tc.method+"/"+recv, func(t *testing.T) {
				src := "package p\n\nimport \"encoding/json/jsontext\"\n\nvar _ jsontext.Value\n\ntype T int\n\nfunc (" + recv + ") " + tc.decl + "\n"
				got := marshalerSetOf(t, src)
				assert.True(t, got.found())
				assert.Equal(t, tc.encode, got.encode)
				assert.Equal(t, !tc.encode, got.decode)
				assert.Equal(t, tc.method, got.first)
			})
		}
	}

	t.Run("aliased jsontext import", func(t *testing.T) {
		src := "package p\n\nimport jt \"encoding/json/jsontext\"\n\ntype T int\n\n" +
			"func (T) MarshalJSONTo(enc *jt.Encoder) error { return nil }\n" +
			"func (*T) UnmarshalJSONFrom(dec *jt.Decoder) error { return nil }\n"
		got := marshalerSetOf(t, src)
		assert.True(t, got.encode)
		assert.True(t, got.decode)
		assert.Equal(t, methodMarshalJSONTo, got.first)
	})
}

// TestMarshalerSetWrongSignatures pins that only the exact encoding/json
// signatures count: a same-named method with any other shape is ignored.
func TestMarshalerSetWrongSignatures(t *testing.T) {
	cases := map[string]string{
		"MarshalText returns string":   "type T int\nfunc (T) MarshalText() string { return \"\" }\n",
		"MarshalJSON one result":       "type T int\nfunc (T) MarshalJSON() []byte { return nil }\n",
		"MarshalJSON wrong 2nd result": "type T int\nfunc (T) MarshalJSON() ([]byte, string) { return nil, \"\" }\n",
		"MarshalJSON takes a param":    "type T int\nfunc (T) MarshalJSON(x int) ([]byte, error) { return nil, nil }\n",
		"UnmarshalJSON takes string":   "type T int\nfunc (*T) UnmarshalJSON(s string) error { return nil }\n",
		"UnmarshalJSON no result":      "type T int\nfunc (*T) UnmarshalJSON(b []byte) {}\n",
		"AppendText no param":          "type T int\nfunc (T) AppendText() ([]byte, error) { return nil, nil }\n",
		"MarshalJSONTo json.Encoder":   "import \"encoding/json\"\ntype T int\nfunc (T) MarshalJSONTo(enc *json.Encoder) error { return nil }\n",
		"MarshalJSONTo foreign jsontext": "import \"github.com/go-json-experiment/json/jsontext\"\ntype T int\n" +
			"func (T) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }\n",
		"MarshalJSONTo value encoder": "import \"encoding/json/jsontext\"\ntype T int\nfunc (T) MarshalJSONTo(enc jsontext.Encoder) error { return nil }\n",
		"method on another type":      "type T int\ntype U int\nfunc (U) MarshalJSON() ([]byte, error) { return nil, nil }\n",
		"plain function":              "type T int\nfunc MarshalJSON() ([]byte, error) { return nil, nil }\n",
		"array not slice":             "type T int\nfunc (*T) UnmarshalJSON(b [4]byte) error { return nil }\n",
		"unrelated method":            "type T int\nfunc (T) String() string { return \"\" }\n",
		"MarshalJSONTo local Encoder": "type T int\ntype Encoder struct{}\nfunc (T) MarshalJSONTo(enc *Encoder) error { return nil }\n",
		"MarshalJSONTo jsontext.Decoder": "import \"encoding/json/jsontext\"\ntype T int\n" +
			"func (T) MarshalJSONTo(enc *jsontext.Decoder) error { return nil }\n",
		"slice of other elem": "type T int\nfunc (*T) UnmarshalJSON(b []int) error { return nil }\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got := marshalerSetOf(t, "package p\n\n"+body)
			assert.False(t, got.found(), "%+v", got)
		})
	}
}

// TestMarshalerSetNamedResults pins that named results count, and that a
// single field list entry naming two params is two params.
func TestMarshalerSetNamedResults(t *testing.T) {
	got := marshalerSetOf(t, "package p\n\ntype T int\n\nfunc (T) MarshalJSON() (b []byte, err error) { return nil, nil }\n")
	assert.True(t, got.encode)

	got = marshalerSetOf(t, "package p\n\ntype T int\n\nfunc (T) UnmarshalJSON(a, b []byte) error { return nil }\n")
	assert.False(t, got.found())
}

// TestMarshalerSetSiblingFileAndOtherPackage pins the file scan: a sibling
// same-package file counts, another package in the directory does not, and an
// unreadable directory still scans the current file.
func TestMarshalerSetSiblingFileAndOtherPackage(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "m.go"),
		[]byte("package p\n\nfunc (*T) UnmarshalText(b []byte) error { return nil }\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gen.go"),
		[]byte("//go:build ignore\n\npackage main\n\nfunc (T) MarshalJSON() ([]byte, error) { return nil, nil }\n"), 0o600))
	a := New(dir)
	file, path := parseInDir(t, a, dir, "t.go", "package p\n\ntype T int\n")
	got := a.marshalerSet("T", newResolveCtx(file, path, false))
	assert.True(t, got.decode, "the sibling same-package method counts")
	assert.False(t, got.encode, "a package main sibling does not")
	assert.Equal(t, methodUnmarshalText, got.first)

	gone := filepath.Join(dir, "missing")
	own, ownPath := parseInDir(t, a, gone, "t.go", "package p\n\ntype T int\n\nfunc (T) MarshalText() ([]byte, error) { return nil, nil }\n")
	got = a.marshalerSet("T", newResolveCtx(own, ownPath, false))
	assert.True(t, got.encode, "an unreadable dir still scans the current file")
}

// TestFieldTypes pins the field list flattening the signature matcher uses.
func TestFieldTypes(t *testing.T) {
	assert.Empty(t, fieldTypes(nil))
	fd := func(src string) *ast.FuncDecl {
		f, err := parser.ParseFile(New(t.TempDir()).fileSet, "x.go", "package p\n"+src, 0)
		require.NoError(t, err)
		return f.Decls[0].(*ast.FuncDecl)
	}
	assert.Len(t, fieldTypes(fd("func f(a, b []byte) {}").Type.Params), 2)
	assert.Len(t, fieldTypes(fd("func f() ([]byte, error) { return nil, nil }").Type.Results), 2)
	assert.Empty(t, fieldTypes(fd("func f() {}").Type.Results))
}

// TestMarshalerCandidateUnknownName pins that a method outside the seven is
// never a candidate, whatever its signature.
func TestMarshalerCandidateUnknownName(t *testing.T) {
	a := New(t.TempDir())
	f, err := parser.ParseFile(a.fileSet, "x.go", "package p\nfunc (T) Marshal() ([]byte, error) { return nil, nil }\n", 0)
	require.NoError(t, err)
	fd, _ := a.marshalerCandidate(f.Decls[0], "T")
	assert.Nil(t, fd)
}
