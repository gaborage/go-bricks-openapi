package analyzer

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// qualifiedB is package b of the qualified-resolution tests: one declaration
// per row of #100's table. Status's MarshalText lives in b/status_text.go.
const qualifiedB = `package b

import (
	"time"

	"github.com/example/app/c"
)

type Cents int64
type Big uint64
type Flag bool
type Code string
type Codes []Code
type Wrapped Cents
type Grade c.Level
type Base struct {
	Grade Grade   ` + "`json:\"grade\"`" + `
	Level c.Level ` + "`json:\"level\"`" + `
}
type Tags []string
type User struct {
	Name string ` + "`json:\"name\"`" + `
}
type UserList []User
type Items []c.Item
type Stamp = time.Time
type Blob []byte
type Tree map[string]Tree
type Addr uintptr
type Status int
type PStatus int

func (p *PStatus) UnmarshalJSON(b []byte) error { return nil }

type FlagU byte

func (f *FlagU) UnmarshalJSON(b []byte) error { return nil }

type FlagV byte

func (f *FlagV) MarshalJSON() ([]byte, error) { return nil, nil }

type T2 time.Time
type MStamp time.Time

func (MStamp) MarshalJSON() ([]byte, error) { return nil, nil }
`

// qualifiedImports are the import lines of every qualified rows module.
const qualifiedImports = "\t\"github.com/example/app/b\"\n\t\"github.com/example/app/kinds\"\n\tmoney \"github.com/example/app/b\"\n"

// analyzeQualified analyzes a project with packages b, c and kinds beside a
// rows module (mod/module.go) declaring decls and the Rows fields rows, plus
// any extra files; it returns the analyzer and Rows' rendered resolutions.
func analyzeQualified(t *testing.T, decls, rows string, extra map[string]string) (a *ProjectAnalyzer, got map[string]string) {
	t.Helper()
	files := map[string]string{
		"go.mod":                             resolveGoMod,
		filepath.Join("b", "b.go"):           qualifiedB,
		filepath.Join("b", "status_text.go"): "package b\n\nfunc (s Status) MarshalText() ([]byte, error) { return nil, nil }\n",
		filepath.Join("c", "c.go"):           "package c\n\ntype Level int32\n\ntype Item struct {\n\tID int `json:\"id\"`\n}\n",
		filepath.Join("kinds", "kinds.go"):   "package kinds\n\ntype Labels map[string]string\n",
		filepath.Join("mod", "module.go"):    rowsModule(qualifiedImports, "var _ kinds.Labels\nvar _ money.Cents\n"+decls, rows),
	}
	for k, v := range extra {
		files[k] = v
	}
	a = analyzeDirectiveProject(t, files)
	return a, renders(t, a.typeRegistry["Rows"])
}

// TestQualifiedNamedRows pins #100's resolvable rows: each in-module
// qualified named type resolves in its declaring package, in every position,
// and none of them warns.
func TestQualifiedNamedRows(t *testing.T) {
	rows := []resolveRow{
		{"cents", "b.Cents", goTypeInt64},
		{"moneyCents", "money.Cents", goTypeInt64},
		{"big", "b.Big", goTypeUint64},
		{"flag", "b.Flag", goTypeBool},
		{"code", "b.Code", goTypeString},
		{"wrapped", "b.Wrapped", goTypeInt64},
		{"grade", "b.Grade", goTypeInt32},
		{"discount", "Discount", goTypeInt64},
		{"cents2", "Cents2", goTypeInt64},
		{"pCents", "*b.Cents", "*int64"},
		{"centsList", "[]b.Cents", "[]int64"},
		{"centsGrid", "[][]b.Cents", "[][]int64"},
		{"centsMap", "map[string]b.Cents", "map[string]int64"},
		{"centsMapL", "map[string][]b.Cents", "map[string][]int64"},
		{"centsMapP", "map[string]*b.Cents", "map[string]*int64"},
		{"pCentsMap", "*map[string]b.Cents", "*map[string]int64"},
		{"centsMapArr", "[]map[string]b.Cents", "[]map[string]int64"},
		{"tags", "b.Tags", "[]string"},
		{"pTags", "*b.Tags", "*[]string"},
		{"tagsList", "[]b.Tags", "[][]string"},
		{"tagsMap", "map[string]b.Tags", "map[string][]string"},
		{"moneyTags", "money.Tags", "[]string"},
		{"labels", "kinds.Labels", "map[string]string"},
		{"codes", "b.Codes", "[]string"},
		{"localTags", "Tags", "[][]string"},
		{"userList", "b.UserList", "[]$User"},
		{"items", "b.Items", "[]$Item"},
		{"stamp", "b.Stamp", "time.Time"},
		{"blob", "b.Blob", "[]byte"},
		{"statusD", "StatusD", "int"},
		{"us", "[]b.FlagU", "[]byte"},
	}
	decls := "type Discount b.Cents\ntype Cents2 = b.Cents\ntype StatusD b.Status\ntype Tags []b.Tags\n"
	a, got := analyzeQualified(t, decls, rowsBody(rows), nil)
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestQualifiedShadowing pins that every step after the lookup runs in the
// declaring package: b's Code, Codes and User win over the route package's
// own Code int32 and User struct, b's Tags is not the local Tags, and
// b.Items registers c's Item although the route package never imports c.
func TestQualifiedShadowing(t *testing.T) {
	decls := "type Code int32\ntype User struct{ ID int `json:\"id\"` }\ntype Tags []b.Tags\n"
	rows := "\tCode b.Code `json:\"code\"`\n\tLocalCode Code `json:\"localCode\"`\n\tCodes b.Codes `json:\"codes\"`\n" +
		"\tUserList b.UserList `json:\"userList\"`\n\tLocalUser User `json:\"localUser\"`\n\tItems b.Items `json:\"items\"`\n" +
		"\tTags Tags `json:\"tags\"`\n"
	a, got := analyzeQualified(t, decls, rows, nil)
	assert.Equal(t, goTypeString, got["code"])
	assert.Equal(t, goTypeInt32, got["localCode"])
	assert.Equal(t, "[]string", got["codes"])
	assert.Equal(t, "[][]string", got["tags"], "a local Tags over b.Tags is not recursive")

	bUser := strings.TrimPrefix(got["userList"], "[]$")
	require.Contains(t, a.typeRegistry, bUser)
	assert.Equal(t, "b", a.typeRegistry[bUser].Package, "b.UserList references b's User")
	localUser := strings.TrimPrefix(got["localUser"], "$")
	assert.NotEqual(t, bUser, localUser)
	require.Contains(t, a.typeRegistry, localUser)
	assert.Equal(t, "mod", a.typeRegistry[localUser].Package)
	assert.Equal(t, "[]$Item", got["items"])
	require.Contains(t, a.typeRegistry, "Item")
	assert.Equal(t, "c", a.typeRegistry["Item"].Package)
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestQualifiedWarnedRowsInEveryPosition pins #100's warned rows in all seven
// positions: the render is the position around the direct render, and
// exactly one warning of the row's kind names the field.
func TestQualifiedWarnedRowsInEveryPosition(t *testing.T) {
	reps := []struct{ goType, direct, warning string }{
		{"b.Status", "marshal:Status(int)", "b.Status has its own MarshalText method"},
		{"StatusA", "marshal:Status(int)", "b.Status has its own MarshalText method"},
		{"b.PStatus", "marshal:PStatus(int)", "b.PStatus has its own UnmarshalJSON method"},
		{"b.Tree", "map[string]cycle:Tree", "b.Tree contains itself"},
		{"b.Addr", "uintptr", "holds a uintptr"},
		{"b.Missing", "b.Missing", "b.Missing resolves to no schema"},
		{"decimal.Decimal", "decimal.Decimal", "decimal.Decimal resolves to no schema"},
		{"b.T2", "T2", "b.T2 is a defined type over time.Time"},
		{"b.MStamp", "marshal:MStamp(MStamp)", "b.MStamp has its own MarshalJSON method"},
		{"LStamp", "marshal:LStamp(LStamp)", "LStamp has its own MarshalJSON method"},
	}
	positions := []string{"%s", "*%s", "[]%s", "map[string]%s", "[][]%s", "map[string][]%s", "[]map[string]%s"}
	var body strings.Builder
	for i, r := range reps {
		for j, p := range positions {
			fmt.Fprintf(&body, "\tF%dP%d %s\n", i, j, fmt.Sprintf(p, r.goType))
		}
	}
	files := map[string]string{
		filepath.Join("mod", "module.go"): rowsModule(qualifiedImports+"\t\"github.com/shopspring/decimal\"\n\t\"time\"\n",
			"var _ kinds.Labels\nvar _ money.Cents\ntype StatusA = b.Status\ntype LStamp time.Time\n\nfunc (LStamp) MarshalJSON() ([]byte, error) { return nil, nil }\n", body.String()),
	}
	a, got := analyzeQualified(t, "", "", files)
	for i, r := range reps {
		for j, p := range positions {
			name := fmt.Sprintf("F%dP%d", i, j)
			assert.Equal(t, fmt.Sprintf(p, r.direct), got[name], name)
			if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
				assert.Contains(t, w[0], r.warning, name)
			}
		}
	}
}

// TestQualifiedByteSliceRule pins the byte-slice rule (#97) across packages:
// it is decided by the methods of the element's declaring package, so a
// decode-only b.FlagU keeps []b.FlagU base64, while a local alias of b's
// encode-side FlagV makes its slice an array of {} with a warning.
func TestQualifiedByteSliceRule(t *testing.T) {
	decls := "type FU = b.FlagU\ntype FV = b.FlagV\n"
	rows := "\tUs []b.FlagU `json:\"us\"`\n\tFUs []FU `json:\"fus\"`\n\tVs []b.FlagV `json:\"vs\"`\n\tFVs []FV `json:\"fvs\"`\n" +
		"\tU b.FlagU `json:\"u\"`\n\tUArr [2]b.FlagU `json:\"uArr\"`\n"
	a, got := analyzeQualified(t, decls, rows, nil)
	assert.Equal(t, "[]byte", got["us"])
	assert.Equal(t, "[]byte", got["fus"])
	assert.Equal(t, "[]marshal:FlagV(byte)", got["vs"])
	assert.Equal(t, "[]marshal:FlagV(byte)", got["fvs"])
	assert.Equal(t, "marshal:FlagU(byte)", got["u"], "outside a slice a decode-only type is still a Marshaler type")
	assert.Equal(t, "[N]marshal:FlagU(byte)", got["uArr"], "the rule applies to slices only")
	for _, name := range []string{"Us", "FUs"} {
		assert.Empty(t, fieldWarnings(a, name), name)
	}
	for _, name := range []string{"Vs", "FVs", "U", "UArr"} {
		assert.Len(t, fieldWarnings(a, name), 1, name)
	}
}

// TestQualifiedLookupDegenerate pins inModuleTypeSite's misses: an unknown
// qualifier, a stdlib or third-party package, an in-module path with no
// directory, and a name only a package main file declares all leave the
// leaf unresolvable, warned; a qualified alias or defined type over a struct
// in b becomes a $ref registered in b.
func TestQualifiedLookupDegenerate(t *testing.T) {
	files := map[string]string{
		filepath.Join("g", "kind.go"):   "package g\n\ntype Other int\n",
		filepath.Join("g", "zz_gen.go"): "//go:build ignore\n\npackage main\n\ntype OnlyMain int64\n\nfunc main() {}\n",
		filepath.Join("b", "alias.go"):  "package b\n\ntype UA = User\ntype UD User\n",
		filepath.Join("mod", "module.go"): rowsModule(qualifiedImports+"\t\"github.com/example/app/nope\"\n\t\"github.com/example/app/g\"\n\t\"github.com/shopspring/decimal\"\n",
			"var _ kinds.Labels\nvar _ money.Cents\n",
			"\tZ zz.X `json:\"z\"`\n\tN nope.X `json:\"n\"`\n\tM g.OnlyMain `json:\"m\"`\n\tD decimal.Decimal `json:\"d\"`\n"+
				"\tUA b.UA `json:\"ua\"`\n\tUD b.UD `json:\"ud\"`\n"),
	}
	a, got := analyzeQualified(t, "", "", files)
	for name, rendered := range map[string]string{"Z": "zz.X", "N": "nope.X", "M": "g.OnlyMain", "D": "decimal.Decimal"} {
		assert.Equal(t, rendered, got[strings.ToLower(name)], name)
		if w := fieldWarnings(a, name); assert.Len(t, w, 1, name) {
			assert.Contains(t, w[0], rendered+" resolves to no schema", name)
		}
	}
	assert.Equal(t, "$UA", got["ua"])
	assert.Equal(t, "$UD", got["ud"])
	require.Contains(t, a.typeRegistry, "UA")
	assert.Equal(t, "b", a.typeRegistry["UA"].Package)
	assert.Empty(t, fieldWarnings(a, "UA"))
	assert.Empty(t, fieldWarnings(a, "UD"))

	// With no module path (no go.mod was read), nothing is in-module.
	bare := New(t.TempDir())
	file, _ := parseInDir(t, bare, bare.projectRoot, "x.go", "package x\n\nimport \"github.com/example/app/b\"\n")
	_, _, ok := bare.inModuleTypeSite("b.Cents", file)
	assert.False(t, ok)
}

// widthCase is one layout of package w's files for the build-tagged variant
// tests, with the rendered leaf every w.Width position resolves to.
type widthCase struct {
	name  string
	files map[string]string
	leaf  string
}

// widthCases are the layouts of the build-tagged variant tests: Width in two
// build-tagged files of w, and a file of another clause that w never builds
// sorting FIRST beside it: a //go:build ignore generator in package main, a
// //go:build ignore or //go:build tools file, a file named _ or . first, or a
// package documentation file.
func widthCases() []widthCase {
	variant := func(constraint, decl string) string {
		return "//go:build " + constraint + "\n\npackage w\n\n" + decl + "\n"
	}
	return []widthCase{
		{"widths disagree", map[string]string{"width_amd64.go": variant("amd64", "type Width int64"), "width_386.go": variant("386", "type Width int32")}, "kind:" + kindInteger},
		{"widths agree", map[string]string{"width_amd64.go": variant("amd64", "type Width int64"), "width_arm64.go": variant("arm64", "type Width int64")}, goTypeInt64},
		{"a package main generator sorts first", map[string]string{
			"aaa_gen.go": "//go:build ignore\n\npackage main\n\ntype Width string\n\nfunc main() {}\n",
			"width.go":   "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a build-ignored tool of another clause sorts first", map[string]string{
			"aaa_tool.go": "//go:build ignore\n\npackage tool\n\ntype Width string\n",
			"width.go":    "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a tools-tagged file of another clause sorts first", map[string]string{
			"aaa_tools.go": "//go:build tools\n\npackage tools\n\ntype Width string\n",
			"width.go":     "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"an underscore-named file of another clause sorts first", map[string]string{
			"_tool.go": "package tool\n\ntype Width string\n",
			"width.go": "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a dot-named file of another clause sorts first", map[string]string{
			".tool.go": "package tool\n\ntype Width string\n",
			"width.go": "package w\n\ntype Width int64\n",
		}, goTypeInt64},
		{"a package documentation file sorts first", map[string]string{
			"doc.go":   "package documentation\n\ntype Width string\n",
			"width.go": "package w\n\ntype Width int64\n",
		}, goTypeInt64},
	}
}

// runWidthCases analyzes each widthCase with w's files in dir, imported by
// importLine and referenced as qual.Width directly, as slice items and as a
// map value.
func runWidthCases(t *testing.T, dir, importLine, qual string) {
	t.Helper()
	for _, c := range widthCases() {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				"go.mod": resolveGoMod,
				filepath.Join("mod", "module.go"): rowsModule(importLine, "",
					"\tW "+qual+".Width `json:\"w\"`\n\tWS []"+qual+".Width `json:\"ws\"`\n\tWM map[string]"+qual+".Width `json:\"wm\"`\n"),
			}
			for name, src := range c.files {
				files[filepath.Join(dir, name)] = src
			}
			a := analyzeDirectiveProject(t, files)
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, c.leaf, got["w"])
			assert.Equal(t, "[]"+c.leaf, got["ws"])
			assert.Equal(t, "map[string]"+c.leaf, got["wm"])
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}

// TestQualifiedBuildTaggedVariants pins that every declaration of a
// qualified name in its package is merged, as for a local name (#92), and
// that a generator or tool sorting first never contributes one, under an
// aliased import.
func TestQualifiedBuildTaggedVariants(t *testing.T) {
	runWidthCases(t, "w", "\tww \"github.com/example/app/w\"\n", "ww")
}

// TestQualifiedBuildTaggedVariantsUnaliased is the unaliased twin (#122): an
// unaliased import of a directory (width) whose base is not its package
// clause (w) is named after its importable files' clause, so a generator or
// tool sorting first neither renames the import nor lends a declaration.
func TestQualifiedBuildTaggedVariantsUnaliased(t *testing.T) {
	runWidthCases(t, "width", "\t\"github.com/example/app/width\"\n", "w")
}

// TestUnaliasedImportName pins the name of an unaliased import: its
// directory's importable clause, else the import path's last element.
func TestUnaliasedImportName(t *testing.T) {
	gen := "//go:build ignore\n\npackage main\n\nfunc main() {}\n"
	tagged := "//go:build tools\n\npackage tools\n"
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("toolsfirst", "aaa_tools.go"): tagged,
		filepath.Join("toolsfirst", "y.go"):         "package y\n",
		filepath.Join("tagonly", "aaa_tools.go"):    tagged,
		filepath.Join("tagonly", "z.go"):            "//go:build linux\n\npackage z\n",
		filepath.Join("skipped", ".a.go"):           "package dot\n",
		filepath.Join("skipped", "_b.go"):           "package under\n",
		filepath.Join("skipped", "doc.go"):          "package documentation\n",
		filepath.Join("skipped", "types.go"):        "package real\n",
		filepath.Join("width", "aaa_gen.go"):        gen,
		filepath.Join("width", "width.go"):          "package w\n",
		filepath.Join("tools", "aaa_tool.go"):       "//go:build ignore\n\npackage tool\n",
		filepath.Join("tools", "x.go"):              "package x\n",
		filepath.Join("genonly", "gen.go"):          gen,
		filepath.Join("ignored", "s.go"):            "//go:build ignore\n\npackage s\n",
	})
	for path, want := range map[string]string{
		"github.com/example/app/width":      "w",
		"github.com/example/app/tools":      "x",
		"github.com/example/app/genonly":    "genonly", // no importable file
		"github.com/example/app/ignored":    "ignored", // its only file is build-ignored
		"github.com/example/app/nope":       "nope",    // no such directory
		"github.com/example/app/toolsfirst": "y",       // an unconstrained file names it
		"github.com/example/app/tagonly":    "tools",   // every file constrained: residual
		"github.com/example/app/skipped":    "real",    // _, . and documentation files never build
		"encoding/json":                     "json",    // not in the module
	} {
		assert.Equal(t, want, a.unaliasedImportName(path), path)
	}
}

// TestQualifiedStructImportablePackage pins that a struct reference resolves
// among its package's importable files only (#122): b's first file is a
// package main generator declaring its own Addr, which neither renames b's
// unaliased import nor replaces b's Addr under an aliased one, and a
// directory holding only such a generator resolves no struct.
func TestQualifiedStructImportablePackage(t *testing.T) {
	gen := "//go:build ignore\n\npackage main\n\ntype Addr struct {\n\tWrong int `json:\"wrong\"`\n}\n\nfunc main() {}\n"
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                         resolveGoMod,
		filepath.Join("b", "aaa_gen.go"): gen,
		filepath.Join("b", "b.go"): "package b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
			"type Base struct {\n\tCreated string `json:\"created\"`\n}\n",
		filepath.Join("g", "gen.go"): gen,
		filepath.Join("mod", "module.go"): rowsModule(
			"\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n\tgg \"github.com/example/app/g\"\n", "",
			"\tb.Base\n\tAddr b.Addr `json:\"addr\"`\n\tAliased bb.Addr `json:\"aliased\"`\n\tOnlyGen gg.Addr `json:\"onlyGen\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "$Addr", got["addr"])
	assert.Equal(t, "$Addr", got["aliased"])
	assert.Equal(t, goTypeString, got["created"])
	assert.Equal(t, "gg.Addr", got["onlyGen"])
	require.Contains(t, a.typeRegistry, "Addr")
	assert.Equal(t, map[string]string{"street": goTypeString}, renders(t, a.typeRegistry["Addr"]))
	assert.Equal(t, "b", a.typeRegistry["Addr"].Package)
	if w := fieldWarnings(a, "OnlyGen"); assert.Len(t, w, 1) {
		assert.Contains(t, w[0], "gg.Addr resolves to no schema")
	}
	assert.Empty(t, fieldWarnings(a, "Addr"))
	assert.Empty(t, fieldWarnings(a, "Aliased"))
}

// TestQualifiedToolsFileSortsFirst pins that a file of another clause that b
// never builds, sorting first in b, neither names b nor stands in for b's
// structs under either import, although it declares its own Addr: a
// //go:build tools file (types.go has no build constraint, so its clause is
// b's), a file named _ or . first, and a package documentation file (go/build
// skips the last three whatever the tags).
func TestQualifiedToolsFileSortsFirst(t *testing.T) {
	standIn := "\n\ntype Addr struct {\n\tWrong int `json:\"wrong\"`\n}\n"
	for _, first := range []struct{ name, src string }{
		{"aaa_tools.go", "//go:build tools\n\npackage tools" + standIn},
		{"_tools.go", "package tools" + standIn},
		{".tools.go", "package tools" + standIn},
		{"doc.go", "package documentation" + standIn},
	} {
		t.Run(first.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                       resolveGoMod,
				filepath.Join("b", first.name): first.src,
				filepath.Join("b", "types.go"): "package b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
					"type Base struct {\n\tCreated string `json:\"created\"`\n}\n",
				filepath.Join("mod", "module.go"): rowsModule(
					"\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n", "",
					"\tbb.Base\n\tAliased bb.Addr `json:\"aliased\"`\n\tUnaliased b.Addr `json:\"unaliased\"`\n"),
			})
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, "$Addr", got["aliased"])
			assert.Equal(t, "$Addr", got["unaliased"])
			assert.Equal(t, goTypeString, got["created"])
			require.Contains(t, a.typeRegistry, "Addr")
			assert.Equal(t, map[string]string{"street": goTypeString}, renders(t, a.typeRegistry["Addr"]))
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}

// TestQualifiedAllConstrainedToolsFirst pins that when every file of b
// carries a build constraint, a //go:build tools file of package tools that
// sorts first still cannot hide b's structs under an aliased import: they are
// searched in every file the go command can build, as before. Named
// non-struct types still take that file's clause, as before: bb.Cents falls
// back with a warning, or, when the tools file declares its own Cents, takes
// that declaration silently. An unaliased import of b is still named tools.
func TestQualifiedAllConstrainedToolsFirst(t *testing.T) {
	for _, row := range []struct {
		name, tools, cents string
	}{
		{"fallback", "", "bb.Cents"},
		{"stand-in", "\ntype Cents string\n", goTypeString},
	} {
		t.Run(row.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                           resolveGoMod,
				filepath.Join("b", "aaa_tools.go"): "//go:build tools\n\npackage tools\n" + row.tools,
				filepath.Join("b", "b.go"): "//go:build !plan9\n\npackage b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
					"type Base struct {\n\tCreated string `json:\"created\"`\n}\n\ntype Cents int64\n",
				filepath.Join("mod", "module.go"): rowsModule("\tbb \"github.com/example/app/b\"\n", "",
					"\tbb.Base\n\tAliased bb.Addr `json:\"aliased\"`\n\tCents bb.Cents `json:\"cents\"`\n"),
			})
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, "$Addr", got["aliased"])
			assert.Equal(t, goTypeString, got["created"])
			require.Contains(t, a.typeRegistry, "Addr")
			assert.Equal(t, "b", a.typeRegistry["Addr"].Package)
			assert.Equal(t, row.cents, got["cents"])
			assert.Empty(t, fieldWarnings(a, "Aliased"))
			w := fieldWarnings(a, "Cents")
			if row.tools != "" {
				assert.Empty(t, w)
			} else if assert.Len(t, w, 1) {
				assert.Contains(t, w[0], "bb.Cents resolves to no schema")
				assert.Contains(t, w[0], "an in-module package whose buildable files all carry a build constraint and whose first one by name has another package clause")
			}
			assert.Equal(t, "tools", a.unaliasedImportName("github.com/example/app/b"))
		})
	}
}

// TestQualifiedAllConstrainedStandIns pins that when b's only real file
// carries a build constraint, a file beside it that b never builds, or one of
// another clause built only on a platform b's file excludes, neither names b
// nor stands in for b's types, although it declares its own Addr: a file named
// _ first, a package documentation file, a //go:build ignore tool, a package
// main generator under a tag other than ignore, and a z_plan9.go of another
// clause with no //go:build line. Its name constrains z_plan9.go, so it never
// names b as an unconstrained file would; it is still searched for structs,
// after types.go.
func TestQualifiedAllConstrainedStandIns(t *testing.T) {
	standIn := "\n\ntype Addr struct {\n\tWrong int `json:\"wrong\"`\n}\n"
	for _, other := range []struct{ name, src string }{
		{"_old.go", "package old" + standIn},
		{"doc.go", "package documentation" + standIn},
		{"aaa_tool.go", "//go:build ignore\n\npackage tool" + standIn},
		{"aaa_gen.go", "//go:build generate\n\npackage main" + standIn + "\nfunc main() {}\n"},
		{"z_plan9.go", "package bplan9" + standIn},
	} {
		t.Run(other.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                       resolveGoMod,
				filepath.Join("b", other.name): other.src,
				filepath.Join("b", "types.go"): "//go:build !plan9\n\npackage b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
					"type Base struct {\n\tCreated string `json:\"created\"`\n}\n\ntype Cents int64\n",
				filepath.Join("mod", "module.go"): rowsModule(
					"\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n", "",
					"\tbb.Base\n\tAliased bb.Addr `json:\"aliased\"`\n\tUnaliased b.Addr `json:\"unaliased\"`\n\tCents bb.Cents `json:\"cents\"`\n"),
			})
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, "$Addr", got["aliased"])
			assert.Equal(t, "$Addr", got["unaliased"])
			assert.Equal(t, goTypeString, got["created"])
			assert.Equal(t, goTypeInt64, got["cents"])
			require.Contains(t, a.typeRegistry, "Addr")
			assert.Equal(t, map[string]string{"street": goTypeString}, renders(t, a.typeRegistry["Addr"]))
			assert.Equal(t, "b", a.typeRegistry["Addr"].Package)
			assert.Equal(t, "b", a.unaliasedImportName("github.com/example/app/b"))
			assert.Empty(t, a.Warnings(t.Context()))
		})
	}
}

// TestQualifiedAllConstrainedOwnClauseFirst pins the known regression of
// this fix (R16), so a later fix flips it deliberately. Every file of b the
// go command can build carries a build constraint, and a file of b's own
// clause that it never builds (_old.go, or a //go:build ignore a_gen.go)
// sorts before a //go:build tools file of package tools. packageClause skips
// the first and takes tools, not sure: an unaliased import of b is named
// tools, so b.Addr and b.Cents fall back with a warning and the embedded
// b.Base promotes nothing, and bb.Cents takes the same clause: it falls back
// too, or, when the tools file declares its own Cents (the stand-in row),
// takes that declaration silently, where main resolved b's. Struct
// references through the aliased import still resolve.
func TestQualifiedAllConstrainedOwnClauseFirst(t *testing.T) {
	constrained := "//go:build !plan9\n\npackage b\n\ntype Addr struct {\n\tStreet string `json:\"street\"`\n}\n\n" +
		"type Base struct {\n\tCreated string `json:\"created\"`\n}\n\ntype Cents int64\n"
	toolsFile := "//go:build tools\n\npackage tools\n"
	oldFile := "package b\n\nfunc deprecated() {}\n"
	for _, row := range []struct {
		name    string
		files   map[string]string
		standIn bool // the tools file declares its own Cents
	}{
		{"_old.go", map[string]string{"_old.go": oldFile, "aaa_tools.go": toolsFile, "b.go": constrained}, false},
		{"a_gen.go", map[string]string{"a_gen.go": "//go:build ignore\n\npackage b\n", "tools.go": toolsFile, "types.go": constrained}, false},
		{"_old.go stand-in", map[string]string{"_old.go": oldFile, "aaa_tools.go": toolsFile + "\ntype Cents string\n", "b.go": constrained}, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			files := map[string]string{
				"go.mod": resolveGoMod,
				filepath.Join("mod", "module.go"): rowsModule(
					"\t\"github.com/example/app/b\"\n\tbb \"github.com/example/app/b\"\n", "",
					"\tb.Base\n\tUnaliased b.Addr `json:\"unaliased\"`\n\tUCents b.Cents `json:\"ucents\"`\n"+
						"\tAliased bb.Addr `json:\"aliased\"`\n\tCents bb.Cents `json:\"cents\"`\n"),
			}
			for name, src := range row.files {
				files[filepath.Join("b", name)] = src
			}
			a := analyzeDirectiveProject(t, files)
			got := renders(t, a.typeRegistry["Rows"])
			assert.Equal(t, "$Addr", got["aliased"])
			require.Contains(t, a.typeRegistry, "Addr")
			assert.Equal(t, map[string]string{"street": goTypeString}, renders(t, a.typeRegistry["Addr"]))
			assert.Equal(t, "b", a.typeRegistry["Addr"].Package)
			assert.Empty(t, fieldWarnings(a, "Aliased"))
			assert.NotContains(t, got, "created")
			assert.Equal(t, "tools", a.unaliasedImportName("github.com/example/app/b"))
			assert.Equal(t, "b.Addr", got["unaliased"])
			assert.Equal(t, "b.Cents", got["ucents"])
			warned := []string{"Unaliased", "UCents", "Cents"}
			if row.standIn {
				assert.Equal(t, goTypeString, got["cents"])
				assert.Empty(t, fieldWarnings(a, "Cents"))
				warned = warned[:2]
			} else {
				assert.Equal(t, "bb.Cents", got["cents"])
			}
			for _, field := range warned {
				if w := fieldWarnings(a, field); assert.Len(t, w, 1, field) {
					assert.Contains(t, w[0], "resolves to no schema", field)
				}
			}
		})
	}
}

// TestOSArchSuffixed pins go/build's file-name constraint as osArchSuffixed
// reads it: the element after the last underscore, cut at the first dot.
func TestOSArchSuffixed(t *testing.T) {
	for name, want := range map[string]bool{
		"z_windows.go":         true,
		"x_amd64.go":           true,
		"x_linux_arm64.go":     true,
		"linux_amd64.go":       true,  // the part before the first _ is a prefix, amd64 a suffix
		"x_linux.pb.go":        true,  // cut at the first dot
		"x_windows_test.pb.go": true,  // a trailing _test element is dropped first
		"linux.go":             false, // no underscore: no suffix
		"x_unix.go":            false, // unix is a build tag only
		"x_tools.go":           false,
		"types.go":             false,
	} {
		assert.Equal(t, want, osArchSuffixed(name), name)
	}
}

// TestDelegateBehindGenerator pins that a route delegate in another package
// is found through an unaliased import when a package main generator sorts
// first in its directory (#122): its routes are discovered and typed.
func TestDelegateBehindGenerator(t *testing.T) {
	_, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                         resolveGoMod,
		filepath.Join("h", "aaa_gen.go"): "//go:build ignore\n\npackage main\n\nfunc main() {}\n",
		filepath.Join("h", "h.go"): `package h

import "github.com/gaborage/go-bricks/server"

type Handler struct{}
type Thing struct{ ID int64 ` + "`json:\"id\"`" + ` }

func (x *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/things", x.list)
}
func (x *Handler) list(ctx server.HandlerContext) (server.Result[Thing], server.IAPIError) {
	return server.Result[Thing]{}, nil
}
`,
		filepath.Join("mod", "module.go"): `package mod

import (
	"github.com/example/app/h"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{ hd *h.Handler }

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.hd.RegisterRoutes(hr, r)
}
`,
	})
	list := routeByPath(t, routes, "/things")
	require.NotNil(t, list.Response)
	assert.Equal(t, "Thing", list.Response.Name)
}

// TestBuildIgnored pins which build constraints keep a file out of its
// directory's importable package: only those that need the ignore tag.
func TestBuildIgnored(t *testing.T) {
	for _, c := range []struct {
		head string
		want bool
	}{
		{"//go:build ignore\n\n", true},
		{"// +build ignore\n\n", true},
		{"//go:build ignore && linux\n\n", true},
		{"// Code generated by tool. DO NOT EDIT.\n\n//go:build ignore\n\n", true},
		{"//go:build ignore || linux\n\n", false},
		{"//go:build !linux\n\n", false},
		{"//go:build amd64\n\n", false},
		{"//go:build (\n\n", false},
		{"// a doc comment\n", false},
		{"", false},
	} {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", c.head+"package x\n\n//go:build ignore\n", parser.ParseComments)
		require.NoError(t, err)
		assert.Equal(t, c.want, buildIgnored(f), "%q", c.head)
	}
}

// TestInModuleTypeSiteNoImportablePackage pins that a directory holding only
// a package main generator and a build-ignored tool has no importable
// package, so no declaration in it is a site.
func TestInModuleTypeSiteNoImportablePackage(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod":                      resolveGoMod,
		filepath.Join("g", "gen.go"):  "//go:build ignore\n\npackage main\n\ntype Width int64\n\nfunc main() {}\n",
		filepath.Join("g", "tool.go"): "//go:build ignore\n\npackage tool\n\ntype Width int64\n",
	})
	file, _ := parseInDir(t, a, a.projectRoot, "x.go", "package x\n\nimport gg \"github.com/example/app/g\"\n")
	_, _, ok := a.inModuleTypeSite("gg.Width", file)
	assert.False(t, ok)
}

// TestQualifiedSameClausePackages pins that two in-module packages sharing a
// package clause (orders/model, users/model) keep distinct components: a
// named slice over each one's Item, and a direct struct field, reference the
// struct of their own directory.
func TestQualifiedSameClausePackages(t *testing.T) {
	pkg := func(field string) string {
		return "package model\n\ntype Item struct {\n\t" + field + "\n}\n\ntype Items []Item\n"
	}
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("orders", "model", "model.go"): pkg("SKU string `json:\"sku\"`"),
		filepath.Join("users", "model", "model.go"):  pkg("Email string `json:\"email\"`"),
		filepath.Join("mod", "module.go"): rowsModule(
			"\tom \"github.com/example/app/orders/model\"\n\tum \"github.com/example/app/users/model\"\n", "",
			"\tOrders om.Items `json:\"orders\"`\n\tUsers um.Items `json:\"users\"`\n\tUser um.Item `json:\"user\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "[]$Item", got["orders"])
	assert.Equal(t, "[]$ModelItem", got["users"])
	assert.Equal(t, "$ModelItem", got["user"])
	assert.Equal(t, map[string]string{"sku": goTypeString}, renders(t, a.typeRegistry["Item"]))
	assert.Equal(t, map[string]string{"email": goTypeString}, renders(t, a.typeRegistry["ModelItem"]))
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestQualifiedPayloadResolves pins the payload half (#110): a
// Result[b.Cents] payload resolves in b exactly as a b.Cents field does, to
// int64, with no warning.
func TestQualifiedPayloadResolves(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                   resolveGoMod,
		filepath.Join("b", "b.go"): "package b\n\ntype Cents int64\n",
		filepath.Join("mod", "module.go"): strings.Replace(resolveModuleHead, "import (\n", "import (\n\t\"github.com/example/app/b\"\n", 1) + `
func (m *Module) get(ctx server.HandlerContext) (server.Result[b.Cents], server.IAPIError) {
	return server.Result[b.Cents]{}, nil
}
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/cents", m.get)
}
`,
	})
	assert.Empty(t, a.Warnings(t.Context()))
	cents := routeByPath(t, routes, "/cents").Response
	require.NotNil(t, cents)
	require.NotNil(t, cents.Resolution)
	assert.Equal(t, goTypeInt64, renderShape(*cents.Resolution))
	assert.Empty(t, cents.Name)
}

// TestQualifiedInSiblingFile pins that a struct declared in a sibling of the
// handlers' file resolves its qualified named types against its own imports
// (#128's structDeclSite), so the handlers' file need not import b.
func TestQualifiedInSiblingFile(t *testing.T) {
	files := map[string]string{
		filepath.Join("mod", "module.go"): rowsModule("", "", "\tSib Sib `json:\"sib\"`\n"),
		filepath.Join("mod", "sib.go"): "package mod\n\nimport \"github.com/example/app/b\"\n\n" +
			"type Sib struct {\n\tCents b.Cents `json:\"cents\"`\n\tUsers b.UserList `json:\"users\"`\n}\n",
	}
	a, got := analyzeQualified(t, "", "", files)
	assert.Equal(t, "$Sib", got["sib"])
	sib := renders(t, a.typeRegistry["Sib"])
	assert.Equal(t, goTypeInt64, sib["cents"])
	assert.Equal(t, "[]$User", sib["users"])
	require.Contains(t, a.typeRegistry, "User")
	assert.Equal(t, "b", a.typeRegistry["User"].Package)
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestQualifiedNamedOverStructKeyedBySite pins that a named type over a
// struct is keyed by its own declaring site, not the struct's: a local
// `type User r.Record` and r's own `type User RecordV2`, both over structs of
// r, are two components, each with its own struct's fields.
func TestQualifiedNamedOverStructKeyedBySite(t *testing.T) {
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("r", "r.go"): "package r\n\ntype Record struct {\n\tName string `json:\"name\"`\n}\n\n" +
			"type RecordV2 struct {\n\tEmail string `json:\"email\"`\n}\n\ntype User RecordV2\n",
		filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/r\"\n", "type User r.Record\n",
			"\tLocal User `json:\"local\"`\n\tRemote r.User `json:\"remote\"`\n"),
	})
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, "$User", got["local"])
	assert.Equal(t, "$RUser", got["remote"])
	assert.Equal(t, map[string]string{"name": goTypeString}, renders(t, a.typeRegistry["User"]))
	assert.Equal(t, map[string]string{"email": goTypeString}, renders(t, a.typeRegistry["RUser"]))
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestQualifiedAliasReExportSharesComponent pins that an alias re-exporting a
// same-named type of another package (type User = d.User) is that type, so it
// shares the component a direct d.User reference registers, whichever is met
// first: one component, no <Pkg><Name> twin. An alias under another name
// (type UA = d.User) keeps its own component, as before keying by site.
// Registering an alias of a struct by name, as a payload does, adds no
// component.
func TestQualifiedAliasReExportSharesComponent(t *testing.T) {
	const record = "type Record struct {\n\tID int `json:\"id\"`\n}\n\n"
	cases := []struct {
		name, d, decls, rows string
		want                 map[string]string
		alias, absent        string
	}{
		{"re-export of a struct", "type User struct {\n\tID int `json:\"id\"`\n}\n", "type User = d.User\n",
			"\tA User `json:\"a\"`\n\tB d.User `json:\"b\"`\n", map[string]string{"a": "$User", "b": "$User"}, "User", "DUser"},
		{"re-export of a defined type", record + "type User Record\n", "type User = d.User\n",
			"\tA User `json:\"a\"`\n\tB d.User `json:\"b\"`\n", map[string]string{"a": "$User", "b": "$User"}, "", "DUser"},
		{"re-export of a same-named alias", record + "type User = Record\n", "type User = d.User\n",
			"\tA User `json:\"a\"`\n\tB d.User `json:\"b\"`\n", map[string]string{"a": "$User", "b": "$User"}, "", "DUser"},
		{"re-export inside a container", "type Item struct {\n\tID int `json:\"id\"`\n}\n\ntype Items []Item\n",
			"type Item = d.Item\n\ntype LItems []Item\n", "\tL LItems `json:\"l\"`\n\tI d.Items `json:\"i\"`\n",
			map[string]string{"l": "[]$Item", "i": "[]$Item"}, "Item", "DItem"},
		{"an alias under another name", "type User struct {\n\tID int `json:\"id\"`\n}\n", "type UA = d.User\n",
			"\tA UA `json:\"a\"`\n\tB d.User `json:\"b\"`\n", map[string]string{"a": "$UA", "b": "$User"}, "UA", "DUser"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                          resolveGoMod,
				filepath.Join("d", "d.go"):        "package d\n\n" + c.d,
				filepath.Join("mod", "module.go"): rowsModule("\t\"github.com/example/app/d\"\n", c.decls, c.rows),
			})
			assert.Equal(t, c.want, renders(t, a.typeRegistry["Rows"]))
			assert.NotContains(t, a.typeRegistry, c.absent)
			assert.Empty(t, a.Warnings(t.Context()))
			if c.alias == "" {
				return // an alias of another package's non-struct is no payload component
			}
			path := filepath.Join(a.projectRoot, "mod", "module.go")
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
			require.NoError(t, err)
			before := len(a.typeRegistry)
			reg := a.registerType(c.alias, f.Name.Name, f, path)
			require.NotNil(t, reg)
			assert.Equal(t, c.alias, reg.Name)
			assert.Len(t, a.typeRegistry, before)
		})
	}
}

// TestQualifiedBuildTaggedRefClash pins that build-tagged declarations
// reaching two structs of one short name in different packages (dw.Item,
// lx.Item) do not merge into whichever was met first: they disagree, and
// warn. Declarations reaching one struct through two named types of its
// package (dw.Items, dw.List) still merge.
func TestQualifiedBuildTaggedRefClash(t *testing.T) {
	variant := func(constraint, decl string) string {
		return "//go:build " + constraint + "\n\npackage mod\n\nimport (\n\t\"github.com/example/app/dw\"\n\t\"github.com/example/app/lx\"\n)\n\n" +
			"var _ dw.Items\nvar _ lx.Items\n\n" + decl + "\n"
	}
	pkg := func(name, field string) string {
		return "package " + name + "\n\ntype Item struct {\n\t" + field + "\n}\n\ntype Items []Item\n\ntype List []Item\n"
	}
	cases := []struct {
		name, linux, want string
		warned            bool
	}{
		{"two structs of one name", "type W lx.Items", "W", true},
		{"one struct by two paths", "type W dw.List", "[]$Item", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := analyzeDirectiveProject(t, map[string]string{
				"go.mod":                            resolveGoMod,
				filepath.Join("dw", "dw.go"):        pkg("dw", "Darwin int `json:\"darwin\"`"),
				filepath.Join("lx", "lx.go"):        pkg("lx", "Linux string `json:\"linux\"`"),
				filepath.Join("mod", "module.go"):   rowsModule("", "", "\tW W `json:\"w\"`\n"),
				filepath.Join("mod", "w_darwin.go"): variant("darwin", "type W dw.Items"),
				filepath.Join("mod", "w_linux.go"):  variant("linux", c.linux),
			})
			assert.Equal(t, c.want, renders(t, a.typeRegistry["Rows"])["w"])
			w := fieldWarnings(a, "W")
			if !c.warned {
				assert.Empty(t, w)
				return
			}
			if assert.Len(t, w, 1) {
				assert.Contains(t, w[0], "has type W: W's build-tagged declarations disagree on shape")
			}
		})
	}
}
