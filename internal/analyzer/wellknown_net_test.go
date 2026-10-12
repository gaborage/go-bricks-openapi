package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// netTextTypes are the four out-of-module types with value MarshalText and
// AppendText and pointer UnmarshalText that this tool knows from a table.
var netTextTypes = []string{
	models.WellKnownNetIP, models.WellKnownNetipAddr, models.WellKnownNetipPrefix, models.WellKnownNetipAddrPort,
}

// netImports are the import lines of the net and FileMode rows modules.
const netImports = "\t\"io/fs\"\n\t\"net\"\n\t\"net/netip\"\n\t\"os\"\n"

// netDecls are the named types the net and FileMode rows use.
const netDecls = `
type Perm os.FileMode
type FMA = os.FileMode
type IPA = net.IP
type MyIP net.IP
type MyAddr netip.Addr
type MyPrefix netip.Prefix
`

// TestFileModeAndNetTypesResolve pins the Resolution of every field position
// of os.FileMode, fs.FileMode and the four text types: both FileMode
// spellings, through a defined type or an alias, are the uint32 encoding/json
// writes; the text types stay well-known leaves, an alias of one included; a
// defined type over net.IP is its underlying []byte, while one over a netip
// struct keeps the defined-type warning. Nothing else warns.
func TestFileModeAndNetTypesResolve(t *testing.T) {
	rows := []resolveRow{
		{"mode", "os.FileMode", goTypeUint32},
		{"fsMode", "fs.FileMode", goTypeUint32},
		{"ptrMode", "*os.FileMode", "*" + goTypeUint32},
		{"list", "[]os.FileMode", "[]" + goTypeUint32},
		{"byName", "map[string]fs.FileMode", "map[string]" + goTypeUint32},
		{"perm", "Perm", goTypeUint32},
		{"ptrPerm", "*Perm", "*" + goTypeUint32},
		{"alias", "FMA", goTypeUint32},
		{"ip", "net.IP", models.WellKnownNetIP},
		{"addr", "netip.Addr", models.WellKnownNetipAddr},
		{"prefix", "netip.Prefix", models.WellKnownNetipPrefix},
		{"addrPort", "netip.AddrPort", models.WellKnownNetipAddrPort},
		{"ptrAddr", "*netip.Addr", "*" + models.WellKnownNetipAddr},
		{"ips", "[]net.IP", "[]" + models.WellKnownNetIP},
		{"byHost", "map[string]netip.Addr", "map[string]" + models.WellKnownNetipAddr},
		{"ipa", "IPA", models.WellKnownNetIP},
		{"myIP", "MyIP", "[]byte"},
		{"myAddr", "MyAddr", "MyAddr"},
		{"myPrefix", "MyPrefix", "MyPrefix"},
	}
	a, got := analyzeRows(t, netImports, netDecls, rowsBody(rows))
	for _, r := range rows {
		assert.Equal(t, r.want, got[r.json], r.json)
	}
	warnings := a.Warnings(t.Context())
	assert.Len(t, warnings, 2, "%v", warnings)
	assert.Len(t, matching(warnings, "MyAddr is a defined type over netip.Addr"), 1, "%v", warnings)
	assert.Len(t, matching(warnings, "MyPrefix is a defined type over netip.Prefix"), 1, "%v", warnings)
}

// TestFileModeAndNetKindBacked pins the defined-chain ends: both FileMode
// spellings are uint32 and net.IP is []byte, while the netip structs have
// no kind to fall back to.
func TestFileModeAndNetKindBacked(t *testing.T) {
	a := New(t.TempDir())
	file := importsFile(t, "io/fs", "net", "net/netip", "os")
	for name, want := range map[string]string{
		stdOSFileMode:         goTypeUint32,
		stdFSFileMode:         goTypeUint32,
		models.WellKnownNetIP: "[]byte",
	} {
		got, ok := a.kindBackedUnderlying(name, file)
		if assert.True(t, ok, name) {
			assert.Equal(t, want, renderShape(got), name)
		}
	}
	for _, name := range []string{models.WellKnownNetipAddr, models.WellKnownNetipPrefix, models.WellKnownNetipAddrPort} {
		_, ok := a.kindBackedUnderlying(name, file)
		assert.False(t, ok, name)
	}
}

// TestNetTextTypesStayOutOfKindTable pins that no text type is typed by its
// kind: knownUnderlyingBuiltins ignores methods, so an entry would document
// net.IP as base64 bytes and a netip struct as its kind.
func TestNetTextTypesStayOutOfKindTable(t *testing.T) {
	for _, name := range netTextTypes {
		assert.NotContains(t, knownUnderlyingBuiltins, name)
		assert.True(t, models.WellKnownTypeNames[name], name)
	}
	for _, name := range []string{stdOSFileMode, stdFSFileMode} {
		assert.False(t, models.WellKnownTypeNames[name], "%s resolves to uint32 before the generator looks it up", name)
	}
}

// TestNetTextTypesMethodSets pins the recorded method set of each text type:
// MarshalText and AppendText in the value method set and UnmarshalText,
// which encoding/json reads both ways as a string.
func TestNetTextTypesMethodSets(t *testing.T) {
	a, c := methodSetProject(t, "")
	for _, name := range netTextTypes {
		m, _ := a.qualifiedMethodSet(name, c, true, nil).marshaler(name)
		assert.Equal(t, bitMarshalText|bitAppendText|bitUnmarshalText, m.has, name)
		assert.True(t, m.textBothWays(), name)
	}
}

// TestNetTextTypeEmbeds pins an embedded text type: it promotes its text
// methods, so the embedding struct is a string with no component and no
// warning, beside other fields (EmbIP, EmbPrefix) or alone, through an alias
// too. A defined type drops the methods: over net.IP it is one []byte
// property keyed by its name; over a netip struct, which has no exported
// field, it adds nothing, like a defined type over time.Time. os.FileMode
// and fs.FileMode, non-struct types with no methods, are one uint32
// property each.
func TestNetTextTypeEmbeds(t *testing.T) {
	decls := netDecls + `
type MyAddrPort netip.AddrPort
type EmbIP struct {
	net.IP
	Note string ` + "`json:\"note\"`" + `
}
type EmbAddr struct {
	netip.Addr
}
type EmbPrefix struct {
	netip.Prefix
	N int ` + "`json:\"n\"`" + `
}
type EmbAddrPort struct {
	*netip.AddrPort
}
type EmbIPA struct {
	IPA
}
type EmbMyIP struct {
	MyIP
	N int ` + "`json:\"n\"`" + `
}
type EmbMyAddr struct {
	MyAddr
	N int ` + "`json:\"n\"`" + `
}
type EmbMyPrefix struct {
	*MyPrefix
	N int ` + "`json:\"n\"`" + `
}
type EmbMyAddrPort struct {
	MyAddrPort
	N int ` + "`json:\"n\"`" + `
}
type EmbMode struct {
	os.FileMode
	N int ` + "`json:\"n\"`" + `
}
type EmbFSMode struct {
	*fs.FileMode
}
type EmbPerm struct {
	Perm
}
`
	a := analyzeDirectiveProject(t, map[string]string{
		"go.mod": resolveGoMod,
		filepath.Join("mod", "module.go"): rowsModule(netImports, decls,
			"\tIP EmbIP `json:\"ip\"`\n\tAddr EmbAddr `json:\"addr\"`\n\tPrefix EmbPrefix `json:\"prefix\"`\n"+
				"\tAddrPort EmbAddrPort `json:\"addrPort\"`\n\tIPA EmbIPA `json:\"ipa\"`\n"+
				"\tMyIP EmbMyIP `json:\"myIP\"`\n\tMyAddr EmbMyAddr `json:\"myAddr\"`\n\tMyPrefix EmbMyPrefix `json:\"myPrefix\"`\n"+
				"\tMyAddrPort EmbMyAddrPort `json:\"myAddrPort\"`\n"+
				"\tMode EmbMode `json:\"mode\"`\n\tFSMode EmbFSMode `json:\"fsMode\"`\n\tPerm EmbPerm `json:\"perm\"`\n"),
	})
	rows := renders(t, a.typeRegistry["Rows"])
	for key, typ := range map[string]string{"ip": "EmbIP", "addr": "EmbAddr", "prefix": "EmbPrefix", "addrPort": "EmbAddrPort", "ipa": "EmbIPA"} {
		assert.Nil(t, a.typeRegistry[typ], "%s registers no component", typ)
		assert.Equal(t, "text:"+typ, rows[key], key)
	}
	for typ, want := range map[string]map[string]string{
		"EmbMyIP":       {"MyIP": "[]byte", "n": goTypeInt},
		"EmbMyAddr":     {"n": goTypeInt},
		"EmbMyPrefix":   {"n": goTypeInt},
		"EmbMyAddrPort": {"n": goTypeInt},
		"EmbMode":       {"FileMode": goTypeUint32, "n": goTypeInt},
		"EmbFSMode":     {"FileMode": "*" + goTypeUint32},
		"EmbPerm":       {"Perm": goTypeUint32},
	} {
		assert.Equal(t, want, renders(t, a.typeRegistry[typ]), typ)
	}
	assert.Empty(t, a.Warnings(t.Context()))
}

// TestWellKnownMarshalerEmbedDefined pins which defined types over a
// well-known Marshaler type add nothing when embedded (the structs with no
// exported field) and which are one property (the non-struct ones).
func TestWellKnownMarshalerEmbedDefined(t *testing.T) {
	for name := range wellKnownMethodSets {
		require.Equal(t, embedMarshaler, wellKnownMarshalerEmbed(name, false).class, name)
	}
	for _, name := range []string{models.WellKnownTimeTime, models.WellKnownNetipAddr, models.WellKnownNetipPrefix, models.WellKnownNetipAddrPort} {
		assert.Equal(t, embedSkip, wellKnownMarshalerEmbed(name, true).class, name)
	}
	for _, name := range []string{models.WellKnownUUID, models.WellKnownRawMessage, models.WellKnownJSONNumber, models.WellKnownNetIP} {
		assert.Equal(t, embedNonStruct, wellKnownMarshalerEmbed(name, true).class, name)
	}
}
