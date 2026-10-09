// Package namedpayloadfallback pins response payloads that end in a fallback
// (#110): each documents the schema a struct field of its type documents and
// raises exactly one warning, so --strict fails.
package namedpayloadfallback

import (
	t "time"

	"github.com/example/named_payload_fallback/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "fallback" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// RegisterRoutes registers one route per fallback row. Missing is declared
// nowhere.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/tier", m.tier)
	server.GET(hr, r, "/ptier-ptr", m.ptierPtr)
	server.GET(hr, r, "/ptier", m.ptier)
	server.GET(hr, r, "/tiers", m.tiers)
	server.GET(hr, r, "/flagvs", m.flagvs)
	server.GET(hr, r, "/b-tier", m.bTier)
	server.GET(hr, r, "/tier-map", m.tierMap)
	server.GET(hr, r, "/tree", m.tree)
	server.GET(hr, r, "/addr", m.addr)
	server.GET(hr, r, "/addr-map", m.addrMap)
	server.GET(hr, r, "/uintptr", m.uintptr)
	server.GET(hr, r, "/uintptrs", m.uintptrs)
	server.GET(hr, r, "/decimal", m.decimal)
	server.GET(hr, r, "/aliased-time", m.aliasedTime)
	server.GET(hr, r, "/missing", m.missing)
	server.GET(hr, r, "/decimal-map", m.decimalMap)
	server.GET(hr, r, "/stampd", m.stampd)
	server.GET(hr, r, "/stampds", m.stampds)
	server.GET(hr, r, "/cx", m.cx)
	server.GET(hr, r, "/complex", m.complex)
}

func (m *Module) tier(ctx server.HandlerContext) (server.Result[Tier], server.IAPIError) {
	var v server.Result[Tier]
	return v, nil
}

func (m *Module) ptierPtr(ctx server.HandlerContext) (server.Result[*Tier], server.IAPIError) {
	var v server.Result[*Tier]
	return v, nil
}

func (m *Module) ptier(ctx server.HandlerContext) (server.Result[PTier], server.IAPIError) {
	var v server.Result[PTier]
	return v, nil
}

func (m *Module) tiers(ctx server.HandlerContext) (server.Result[[]Tier], server.IAPIError) {
	var v server.Result[[]Tier]
	return v, nil
}

func (m *Module) flagvs(ctx server.HandlerContext) (server.Result[[]FlagV], server.IAPIError) {
	var v server.Result[[]FlagV]
	return v, nil
}

func (m *Module) bTier(ctx server.HandlerContext) (server.Result[b.Tier], server.IAPIError) {
	var v server.Result[b.Tier]
	return v, nil
}

func (m *Module) tierMap(ctx server.HandlerContext) (server.Result[map[string]Tier], server.IAPIError) {
	var v server.Result[map[string]Tier]
	return v, nil
}

func (m *Module) tree(ctx server.HandlerContext) (server.Result[Tree], server.IAPIError) {
	var v server.Result[Tree]
	return v, nil
}

func (m *Module) addr(ctx server.HandlerContext) (server.Result[Addr], server.IAPIError) {
	var v server.Result[Addr]
	return v, nil
}

func (m *Module) addrMap(ctx server.HandlerContext) (server.Result[map[string]uintptr], server.IAPIError) {
	var v server.Result[map[string]uintptr]
	return v, nil
}

func (m *Module) uintptr(ctx server.HandlerContext) (server.Result[uintptr], server.IAPIError) {
	var v server.Result[uintptr]
	return v, nil
}

func (m *Module) uintptrs(ctx server.HandlerContext) (server.Result[[]uintptr], server.IAPIError) {
	var v server.Result[[]uintptr]
	return v, nil
}

func (m *Module) decimal(ctx server.HandlerContext) (server.Result[decimal.Decimal], server.IAPIError) {
	var v server.Result[decimal.Decimal]
	return v, nil
}

func (m *Module) aliasedTime(ctx server.HandlerContext) (server.Result[t.Time], server.IAPIError) {
	var v server.Result[t.Time]
	return v, nil
}

func (m *Module) missing(ctx server.HandlerContext) (server.Result[*Missing], server.IAPIError) {
	var v server.Result[*Missing]
	return v, nil
}

func (m *Module) decimalMap(ctx server.HandlerContext) (server.Result[map[string]decimal.Decimal], server.IAPIError) {
	var v server.Result[map[string]decimal.Decimal]
	return v, nil
}

func (m *Module) stampd(ctx server.HandlerContext) (server.Result[StampD], server.IAPIError) {
	var v server.Result[StampD]
	return v, nil
}

func (m *Module) stampds(ctx server.HandlerContext) (server.Result[[]StampD], server.IAPIError) {
	var v server.Result[[]StampD]
	return v, nil
}

func (m *Module) cx(ctx server.HandlerContext) (server.Result[Cx], server.IAPIError) {
	var v server.Result[Cx]
	return v, nil
}

func (m *Module) complex(ctx server.HandlerContext) (server.Result[complex128], server.IAPIError) {
	var v server.Result[complex128]
	return v, nil
}
