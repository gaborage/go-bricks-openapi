package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// importsFile parses a file of package p importing each spec, a quoted path
// optionally preceded by its alias (`t "time"`) or a bare path.
func importsFile(t *testing.T, specs ...string) *ast.File {
	t.Helper()
	var b strings.Builder
	b.WriteString("package p\n\nimport (\n")
	for _, s := range specs {
		if !strings.Contains(s, `"`) {
			s = fmt.Sprintf("%q", s)
		}
		fmt.Fprintf(&b, "\t%s\n", s)
	}
	b.WriteString(")\n")
	file, err := parser.ParseFile(token.NewFileSet(), "p.go", b.String(), parser.ImportsOnly)
	require.NoError(t, err)
	return file
}

// TestWellKnownName pins how a qualified name reaches the well-known tables,
// through the import that binds its qualifier: an owner (wellKnownPackages),
// under an alias too, names its package name joined to the type name; a
// project package names nothing, whatever its name; any other package, or a
// qualifier no import binds, names the spelling as written. An owner's type
// outside the tables, and a spelling that is no key, reach nothing.
func TestWellKnownName(t *testing.T) {
	a := New(t.TempDir())
	a.modulePath = "github.com/example/app"
	file := importsFile(t, `t "time"`, "net", "io/fs", `g "github.com/gofrs/uuid/v5"`, `j "encoding/json"`,
		"github.com/example/app/os", "github.com/segmentio/encoding/json", `_ "net/netip"`, `tm "github.com/acme/time"`)
	for qualified, want := range map[string]string{
		"t.Time":       models.WellKnownTimeTime,
		"t.Duration":   models.WellKnownTimeDuration,
		"net.IP":       models.WellKnownNetIP,
		"fs.FileMode":  stdFSFileMode,
		"g.UUID":       models.WellKnownUUID,
		"j.RawMessage": models.WellKnownRawMessage,
		"json.Number":  models.WellKnownJSONNumber, // a drop-in json package, read by its spelling
		"netip.Addr":   models.WellKnownNetipAddr,  // a blank import binds no name
		"time.Time":    models.WellKnownTimeTime,   // time is imported as t only
		"uuid.UUID":    models.WellKnownUUID,       // no import binds uuid
	} {
		got, ok := a.wellKnownName(qualified, file)
		assert.True(t, ok, qualified)
		assert.Equal(t, want, got, qualified)
	}
	for _, qualified := range []string{
		"os.FileMode",     // a project package named os
		"t.Location",      // an owner's type outside the tables
		"tm.Duration",     // another package's spelling is no key
		"decimal.Decimal", // no key
		"Missing",         // no qualifier
	} {
		_, ok := a.wellKnownName(qualified, file)
		assert.False(t, ok, qualified)
	}
}

// shadowPackages are project packages named like the owners of the
// well-known types, each declaring non-struct types of well-known names.
var shadowPackages = map[string]string{
	filepath.Join("net", "net.go"):     "package net\n\ntype IP int\n",
	filepath.Join("netip", "netip.go"): "package netip\n\ntype Addr string\n",
	filepath.Join("fs", "fs.go"):       "package fs\n\ntype FileMode string\n",
	filepath.Join("os", "os.go"):       "package os\n\ntype FileMode bool\n",
	filepath.Join("time", "time.go"):   "package time\n\ntype Duration string\n\ntype Month string\n",
	filepath.Join("json", "json.go"):   "package json\n\ntype Number bool\n\ntype RawMessage string\n",
	filepath.Join("uuid", "uuid.go"):   "package uuid\n\ntype UUID [16]byte\n",
}

// shadowImports import every shadow package by its own name, and the stdlib
// time and net under aliases.
const shadowImports = "\t\"github.com/example/app/net\"\n\t\"github.com/example/app/netip\"\n\t\"github.com/example/app/fs\"\n" +
	"\t\"github.com/example/app/os\"\n\t\"github.com/example/app/time\"\n\t\"github.com/example/app/json\"\n" +
	"\t\"github.com/example/app/uuid\"\n\tstdtime \"time\"\n\tstdnet \"net\"\n"

// shadowProject is a project of the shadow packages beside a rows module
// (mod/module.go) declaring decls and the Rows fields rows.
func shadowProject(decls, rows string) map[string]string {
	files := map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("mod", "module.go"): rowsModule(shadowImports, decls, rows),
	}
	for path, src := range shadowPackages {
		files[path] = src
	}
	return files
}

// TestShadowPackagesResolveFromSource pins issue 164: a project package
// named like the owner of a well-known type (net, netip, fs, os, time, json,
// uuid) is resolved from source, at every position and through a defined
// type, while the stdlib packages imported beside them under an alias are
// still the well-known types. Nothing warns.
func TestShadowPackagesResolveFromSource(t *testing.T) {
	rows := []resolveRow{
		{"ip", "net.IP", goTypeInt},
		{"addr", "netip.Addr", goTypeString},
		{"mode", "fs.FileMode", goTypeString},
		{"osMode", "os.FileMode", goTypeBool},
		{"dur", "time.Duration", goTypeString},
		{"month", "time.Month", goTypeString},
		{"num", "json.Number", goTypeBool},
		{"raw", "json.RawMessage", goTypeString},
		{"id", "uuid.UUID", "[N]byte"},
		{"ptrIP", "*net.IP", "*" + goTypeInt},
		{"ips", "[]net.IP", "[]" + goTypeInt},
		{"byName", "map[string]fs.FileMode", "map[string]" + goTypeString},
		{"timeout", "Timeout", goTypeString},
		{"perm", "Perm", goTypeString},
		{"stdDur", "stdtime.Duration", goTypeInt64},
		{"stdTime", "stdtime.Time", models.WellKnownTimeTime},
		{"stdIP", "stdnet.IP", models.WellKnownNetIP},
		{"stdTimeout", "StdTimeout", goTypeInt64},
		{"stdMyIP", "StdMyIP", "[]byte"},
	}
	decls := "type Timeout time.Duration\ntype Perm fs.FileMode\ntype StdTimeout stdtime.Duration\ntype StdMyIP stdnet.IP\n"
	a := analyzeDirectiveProject(t, shadowProject(decls, rowsBody(rows)))
	got := renders(t, a.typeRegistry["Rows"])
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestShadowPackageEmbeds pins an embedded shadow type: an exported
// non-struct embed is one property keyed by its type name and typed from
// source, while an aliased stdlib embed keeps its well-known classification
// (stdnet.IP promotes its text methods, stdtime.Duration is its int64).
func TestShadowPackageEmbeds(t *testing.T) {
	decls := `
type EmbIP struct {
	net.IP
}
type EmbDur struct {
	time.Duration
	N int ` + "`json:\"n\"`" + `
}
type EmbStdIP struct {
	stdnet.IP
}
type EmbStdDur struct {
	*stdtime.Duration
}
`
	rows := "\tIP EmbIP `json:\"ip\"`\n\tDur EmbDur `json:\"dur\"`\n\tStdIP EmbStdIP `json:\"stdIP\"`\n\tStdDur EmbStdDur `json:\"stdDur\"`\n"
	a := analyzeDirectiveProject(t, shadowProject(decls, rows))
	assert.Equal(t, map[string]string{"IP": goTypeInt}, renders(t, a.typeRegistry["EmbIP"]))
	assert.Equal(t, map[string]string{"Duration": goTypeString, "n": goTypeInt}, renders(t, a.typeRegistry["EmbDur"]))
	assert.Equal(t, map[string]string{"Duration": "*" + goTypeInt64}, renders(t, a.typeRegistry["EmbStdDur"]))
	assert.Nil(t, a.typeRegistry["EmbStdIP"], "a text type's embedding struct registers no component")
	assert.Equal(t, "text:EmbStdIP", renders(t, a.typeRegistry["Rows"])["stdIP"])
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestShadowPackagePayloads pins response payloads of shadow types: typed
// from source with their name cleared, where the aliased stdlib types keep
// their well-known name and Resolution, spelled as the tables spell them.
func TestShadowPackagePayloads(t *testing.T) {
	files := shadowProject("", "")
	var reg, handlers strings.Builder
	payloads := []struct{ result, name, res string }{
		{"server.Result[net.IP]", "", goTypeInt},
		{"server.Result[[]net.IP]", "", "[]" + goTypeInt},
		{"server.Result[*fs.FileMode]", "", "*" + goTypeString},
		{"server.Result[stdtime.Time]", "Time", models.WellKnownTimeTime},
		{"server.Result[[]stdnet.IP]", "IP", "[]" + models.WellKnownNetIP},
		{"server.Result[stdtime.Duration]", "Duration", goTypeInt64},
	}
	for i, p := range payloads {
		fmt.Fprintf(&reg, "\tserver.GET(hr, r, \"/p%d\", m.h%d)\n", i, i)
		fmt.Fprintf(&handlers, "func (m *Module) h%d(ctx server.HandlerContext) (%s, server.IAPIError) {\n\tvar v %s\n\treturn v, nil\n}\n\n",
			i, p.result, p.result)
	}
	files[filepath.Join("mod", "module.go")] = "package mod\n\nimport (\n\t\"github.com/gaborage/go-bricks/app\"\n\t\"github.com/gaborage/go-bricks/server\"\n" +
		shadowImports + ")\n\ntype Module struct{}\n\n" +
		"func (m *Module) Name() string                    { return \"mod\" }\n" +
		"func (m *Module) Init(deps *app.ModuleDeps) error { return nil }\n" +
		"func (m *Module) Shutdown() error                 { return nil }\n\n" +
		"func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {\n" + reg.String() + "}\n\n" + handlers.String()
	a, routes := analyzeProjectRoutes(t, files)
	for i, p := range payloads {
		ti := routeByPath(t, routes, fmt.Sprintf("/p%d", i)).Response
		require.NotNil(t, ti, p.result)
		assert.Equal(t, p.name, ti.Name, p.result)
		if assert.NotNil(t, ti.Resolution, p.result) {
			assert.Equal(t, p.res, renderShape(*ti.Resolution), p.result)
		}
		assert.True(t, IsTypedPayload(ti), p.result)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestWellKnownOwners pins which packages own a well-known name: every UUID
// library whose UUID is the canonical string, unaliased (its path base need
// not be its name) or aliased. A json package other than encoding/json, such
// as a drop-in whose Number is encoding/json's through a type alias, is read
// by its spelling: its json.Number fields and payloads stay the well-known
// type, with no warning.
func TestWellKnownOwners(t *testing.T) {
	src := `package mod

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	gofrs "github.com/gofrs/uuid"
	"github.com/gofrs/uuid/v5"
	sat "github.com/satori/go.uuid"
	gu "github.com/google/uuid"
	"github.com/segmentio/encoding/json"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type Rows struct {
	A gofrs.UUID ` + "`json:\"a\"`" + `
	B uuid.UUID ` + "`json:\"b\"`" + `
	C sat.UUID ` + "`json:\"c\"`" + `
	D gu.UUID ` + "`json:\"d\"`" + `
	N json.Number ` + "`json:\"n\"`" + `
	Ns []json.Number ` + "`json:\"ns\"`" + `
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
	server.GET(hr, r, "/num", m.num)
	server.GET(hr, r, "/nums", m.nums)
	server.GET(hr, r, "/id", m.id)
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) { return server.Result[Rows]{}, nil }
func (m *Module) num(ctx server.HandlerContext) (server.Result[json.Number], server.IAPIError) { return server.Result[json.Number]{}, nil }
func (m *Module) nums(ctx server.HandlerContext) (server.Result[[]json.Number], server.IAPIError) { return server.Result[[]json.Number]{}, nil }
func (m *Module) id(ctx server.HandlerContext) (server.Result[sat.UUID], server.IAPIError) { return server.Result[sat.UUID]{}, nil }
`
	a, routes := analyzeSingleModule(t, src)
	got := renders(t, a.typeRegistry["Rows"])
	assert.Equal(t, map[string]string{
		"a": models.WellKnownUUID, "b": models.WellKnownUUID, "c": models.WellKnownUUID, "d": models.WellKnownUUID,
		"n": models.WellKnownJSONNumber, "ns": "[]" + models.WellKnownJSONNumber,
	}, got)

	for path, want := range map[string]struct{ name, res string }{
		"/id":   {"UUID", models.WellKnownUUID},
		"/num":  {"Number", models.WellKnownJSONNumber},
		"/nums": {"Number", "[]" + models.WellKnownJSONNumber},
	} {
		ti := routeByPath(t, routes, path).Response
		require.NotNil(t, ti, path)
		assert.Equal(t, want.name, ti.Name, path)
		if assert.NotNil(t, ti.Resolution, path) {
			assert.Equal(t, want.res, renderShape(*ti.Resolution), path)
		}
		assert.True(t, IsTypedPayload(ti), path)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestWellKnownTableKeysHaveOwners pins that every key of the well-known
// tables is spelled with the package name of a path in wellKnownPackages:
// the tables are read only through it, so a key of any other package would
// never be reached.
func TestWellKnownTableKeysHaveOwners(t *testing.T) {
	owners := map[string]bool{}
	for _, pkg := range wellKnownPackages {
		owners[pkg] = true
	}
	keys := slices.Collect(maps.Keys(models.WellKnownTypeNames))
	keys = slices.AppendSeq(keys, maps.Keys(knownUnderlyingBuiltins))
	keys = slices.AppendSeq(keys, maps.Keys(wellKnownMethodSets))
	for _, key := range keys {
		pkg, _, _ := strings.Cut(key, ".")
		assert.True(t, owners[pkg], "%s: no import path in wellKnownPackages declares package %s", key, pkg)
		assert.True(t, spelledLikeWellKnown(key), key)
	}
}
