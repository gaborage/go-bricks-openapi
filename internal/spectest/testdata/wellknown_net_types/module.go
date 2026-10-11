// Package wellknownnettypes pins os.FileMode, fs.FileMode, net.IP,
// netip.Addr, netip.Prefix and netip.AddrPort documented from their
// encoding/json wire form. Both FileMode spellings are a uint32 written as
// its number; the four text types are a JSON string through their value
// MarshalText, with no type-derived format, and a validate format, enum or
// pattern tag still stamps its keyword, as on uuid.UUID. A defined type over
// net.IP drops MarshalText and is base64 []byte; one over netip.Addr or
// netip.Prefix keeps the untyped object and its warning. An embedded text
// type promotes its methods, so the embedding struct is a string; an
// embedded os.FileMode is one uint32 property keyed FileMode, and an
// embedded defined type over a netip struct adds nothing.
package wellknownnettypes

import (
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"os"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "wellknownnettypes" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Perm is a defined type over os.FileMode: a uint32, as encoding/json
// writes it.
type Perm os.FileMode

// FMA is an alias of os.FileMode.
type FMA = os.FileMode

// IPA is an alias of net.IP: it keeps net.IP's methods.
type IPA = net.IP

// MyIP is a defined type over net.IP: it drops MarshalText, so
// encoding/json writes the underlying []byte as base64.
type MyIP net.IP

// MyAddr and MyPrefix are defined types over netip structs with no exported
// field: they drop the text methods and encoding/json writes {}. They stay
// untyped objects with the defined-type warning.
type MyAddr netip.Addr

// MyPrefix is a defined type over netip.Prefix.
type MyPrefix netip.Prefix

// Modes holds the FileMode field rows.
type Modes struct {
	Mode    os.FileMode            `json:"mode" validate:"max=511"` // -> {integer, int64, minimum 0, maximum 511}
	FSMode  fs.FileMode            `json:"fsMode" example:"420"`    // -> {integer, int64, minimum 0, example 420}
	PtrMode *os.FileMode           `json:"ptrMode"`                 // -> {integer, int64, minimum 0, nullable}
	List    []os.FileMode          `json:"list"`                    // -> items {integer, int64, minimum 0}
	ByName  map[string]fs.FileMode `json:"byName"`                  // -> additionalProperties {integer, int64, minimum 0}
	Perm    Perm                   `json:"perm"`                    // -> {integer, int64, minimum 0}
	PtrPerm *Perm                  `json:"ptrPerm"`                 // -> {integer, int64, minimum 0, nullable}
	Alias   FMA                    `json:"alias" validate:"min=1"`  // -> {integer, int64, minimum 1}
}

// Addrs holds the text-type field rows.
type Addrs struct {
	IP        net.IP                `json:"ip" example:"1.2.3.4" validate:"required,ip"` // -> {string, example}, required, no format
	Bounded   net.IP                `json:"bounded" validate:"min=7,max=15"`             // -> {string}: length bounds dropped
	Port      netip.AddrPort        `json:"port" validate:"min=9,max=21"`                // -> {string}: length bounds dropped
	V4        net.IP                `json:"v4" validate:"ipv4"`                          // -> {string, format ipv4}
	PtrV4     *net.IP               `json:"ptrV4" validate:"omitempty,ipv4"`             // -> {string, format ipv4, nullable}
	V6List    []netip.Addr          `json:"v6List" validate:"max=2,dive,ipv6"`           // -> maxItems 2, items {string, format ipv6}
	OneOf     netip.Addr            `json:"oneOf" validate:"oneof=1.2.3.4 5.6.7.8"`      // -> {string, enum}
	Ten       netip.Prefix          `json:"ten" validate:"startswith=10."`               // -> {string, pattern ^10\.}
	Addr      netip.Addr            `json:"addr"`                                        // -> {string}
	Prefix    netip.Prefix          `json:"prefix"`                                      // -> {string}
	AddrPort  netip.AddrPort        `json:"addrPort"`                                    // -> {string}
	PtrIP     *net.IP               `json:"ptrIP"`                                       // -> {string, nullable}
	PtrAddr   *netip.Addr           `json:"ptrAddr"`                                     // -> {string, nullable}
	IPs       []net.IP              `json:"ips" validate:"max=3"`                        // -> maxItems 3, items {string}
	AddrList  []netip.Addr          `json:"addrList" validate:"max=2,dive,ip"`           // -> maxItems 2, items {string}
	Prefixes  []netip.Prefix        `json:"prefixes"`                                    // -> items {string}
	ByHost    map[string]netip.Addr `json:"byHost"`                                      // -> additionalProperties {string}
	Alias     IPA                   `json:"alias"`                                       // -> {string}
	Defined   MyIP                  `json:"defined"`                                     // -> {string, format byte}
	DefAddr   MyAddr                `json:"defAddr"`                                     // -> {object}, warned
	DefPrefix MyPrefix              `json:"defPrefix"`                                   // -> {object}, warned
}

// EmbIP embeds net.IP beside a field: the promoted MarshalText wins, so
// encoding/json writes a string and Note is never written.
type EmbIP struct {
	net.IP
	Note string `json:"note"`
}

// EmbAddr embeds netip.Addr alone.
type EmbAddr struct {
	netip.Addr
}

// EmbPrefix embeds netip.Prefix beside a field.
type EmbPrefix struct {
	netip.Prefix
	N int `json:"n"`
}

// EmbAddrPort embeds netip.AddrPort alone.
type EmbAddrPort struct {
	netip.AddrPort
}

// EmbMode embeds os.FileMode, a non-struct type with no methods: one
// property keyed FileMode, as encoding/json writes it.
type EmbMode struct {
	os.FileMode
	N int `json:"n"`
}

// EmbDefAddr embeds a defined type over netip.Addr: a struct with no
// exported field, so it adds nothing.
type EmbDefAddr struct {
	MyAddr
	N int `json:"n"`
}

// Embeds holds the embed rows.
type Embeds struct {
	IP       EmbIP       `json:"ip"`       // -> {string}, no component
	Addr     EmbAddr     `json:"addr"`     // -> {string}, no component
	Prefix   EmbPrefix   `json:"prefix"`   // -> {string}, no component
	AddrPort EmbAddrPort `json:"addrPort"` // -> {string}, no component
	Mode     EmbMode     `json:"mode"`     // -> $ref EmbMode {FileMode, n}
	DefAddr  EmbDefAddr  `json:"defAddr"`  // -> $ref EmbDefAddr {n}
}

// ModeReq carries FileMode and Perm parameters: go-bricks binds them by
// kind, as the uint32 they are.
type ModeReq struct {
	PMode  os.FileMode `param:"pmode"`
	Mode   os.FileMode `query:"mode" validate:"max=511"`
	Header fs.FileMode `header:"X-Mode"`
	Perm   Perm        `query:"perm"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/modes/:pmode", m.modes)
	server.GET(hr, r, "/addrs", m.addrs)
	server.GET(hr, r, "/embeds", m.embeds)

	server.GET(hr, r, "/payload/mode", m.payloadMode)
	server.GET(hr, r, "/payload/fs-mode", m.payloadFSMode)
	server.GET(hr, r, "/payload/ptr-mode", m.payloadPtrMode)
	server.GET(hr, r, "/payload/perm", m.payloadPerm)
	server.GET(hr, r, "/payload/modes", m.payloadModes)
	server.GET(hr, r, "/payload/ptr-fs-modes", m.payloadPtrFSModes)
	server.GET(hr, r, "/payload/mode-map", m.payloadModeMap)

	server.GET(hr, r, "/payload/ip", m.payloadIP)
	server.GET(hr, r, "/payload/addr", m.payloadAddr)
	server.GET(hr, r, "/payload/prefix", m.payloadPrefix)
	server.GET(hr, r, "/payload/addr-port", m.payloadAddrPort)
	server.GET(hr, r, "/payload/ptr-addr", m.payloadPtrAddr)
	server.GET(hr, r, "/payload/meta-addr-port", m.payloadMetaAddrPort)
	server.GET(hr, r, "/payload/ipa", m.payloadIPA)
	server.GET(hr, r, "/payload/ips", m.payloadIPs)
	server.GET(hr, r, "/payload/prefixes", m.payloadPrefixes)
	server.GET(hr, r, "/payload/my-ip", m.payloadMyIP)
	server.GET(hr, r, "/payload/my-addr", m.payloadMyAddr)
	server.GET(hr, r, "/payload/emb-ip", m.payloadEmbIP)
}

func (m *Module) modes(req ModeReq, ctx server.HandlerContext) (server.Result[Modes], server.IAPIError) {
	return server.NewResult(http.StatusOK, Modes{}), nil
}

func (m *Module) addrs(ctx server.HandlerContext) (server.Result[Addrs], server.IAPIError) {
	return server.NewResult(http.StatusOK, Addrs{}), nil
}

func (m *Module) embeds(ctx server.HandlerContext) (server.Result[Embeds], server.IAPIError) {
	return server.NewResult(http.StatusOK, Embeds{}), nil
}

func (m *Module) payloadMode(ctx server.HandlerContext) (server.Result[os.FileMode], server.IAPIError) {
	return server.Result[os.FileMode]{}, nil
}

func (m *Module) payloadFSMode(ctx server.HandlerContext) (server.Result[fs.FileMode], server.IAPIError) {
	return server.Result[fs.FileMode]{}, nil
}

func (m *Module) payloadPtrMode(ctx server.HandlerContext) (server.Result[*os.FileMode], server.IAPIError) {
	return server.Result[*os.FileMode]{}, nil
}

func (m *Module) payloadPerm(ctx server.HandlerContext) (server.Result[Perm], server.IAPIError) {
	return server.Result[Perm]{}, nil
}

func (m *Module) payloadModes(ctx server.HandlerContext) (server.Result[[]os.FileMode], server.IAPIError) {
	return server.Result[[]os.FileMode]{}, nil
}

func (m *Module) payloadPtrFSModes(ctx server.HandlerContext) (server.Result[[]*fs.FileMode], server.IAPIError) {
	return server.Result[[]*fs.FileMode]{}, nil
}

func (m *Module) payloadModeMap(ctx server.HandlerContext) (server.Result[map[string]fs.FileMode], server.IAPIError) {
	return server.Result[map[string]fs.FileMode]{}, nil
}

func (m *Module) payloadIP(ctx server.HandlerContext) (server.Result[net.IP], server.IAPIError) {
	return server.Result[net.IP]{}, nil
}

func (m *Module) payloadAddr(ctx server.HandlerContext) (server.Result[netip.Addr], server.IAPIError) {
	return server.Result[netip.Addr]{}, nil
}

func (m *Module) payloadPrefix(ctx server.HandlerContext) (server.Result[netip.Prefix], server.IAPIError) {
	return server.Result[netip.Prefix]{}, nil
}

func (m *Module) payloadAddrPort(ctx server.HandlerContext) (server.Result[netip.AddrPort], server.IAPIError) {
	return server.Result[netip.AddrPort]{}, nil
}

func (m *Module) payloadPtrAddr(ctx server.HandlerContext) (server.Result[*netip.Addr], server.IAPIError) {
	return server.Result[*netip.Addr]{}, nil
}

func (m *Module) payloadMetaAddrPort(ctx server.HandlerContext) (server.ResultWithMeta[netip.AddrPort], server.IAPIError) {
	return server.ResultWithMeta[netip.AddrPort]{}, nil
}

func (m *Module) payloadIPA(ctx server.HandlerContext) (server.Result[IPA], server.IAPIError) {
	return server.Result[IPA]{}, nil
}

func (m *Module) payloadIPs(ctx server.HandlerContext) (server.Result[[]net.IP], server.IAPIError) {
	return server.Result[[]net.IP]{}, nil
}

func (m *Module) payloadPrefixes(ctx server.HandlerContext) (server.Result[[]netip.Prefix], server.IAPIError) {
	return server.Result[[]netip.Prefix]{}, nil
}

func (m *Module) payloadMyIP(ctx server.HandlerContext) (server.Result[MyIP], server.IAPIError) {
	return server.Result[MyIP]{}, nil
}

func (m *Module) payloadMyAddr(ctx server.HandlerContext) (server.Result[MyAddr], server.IAPIError) {
	return server.Result[MyAddr]{}, nil
}

func (m *Module) payloadEmbIP(ctx server.HandlerContext) (server.Result[EmbIP], server.IAPIError) {
	return server.Result[EmbIP]{}, nil
}
