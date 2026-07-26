// Package api is a regression fixture for Plan 026 (retiring the hand-rolled
// extractTag substring scanner in favor of reflect.StructTag.Lookup). Every
// param/query/header/json/validate tag below is preceded by a vet-clean
// "decoy" tag whose key merely ENDS IN the real key name — e.g. `queryparam:`
// ends in `param:`, `myjson:` ends in `json:`. The retired scanner matched the
// decoy first (substring search), silently emitting the wrong parameter name,
// the wrong header name, or the wrong property name. A doc tag also carries an
// escaped quote, which the old scanner's first-quote-byte value terminator
// truncated.
package api

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the struct_tags module.
type Module struct{}

func (m *Module) Name() string                    { return "struct_tags" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// GetItemReq exercises the param/query/header decoy pairs. Each real tag
// (param, query, header) is preceded by a decoy tag whose key ends in the same
// name (queryparam:, notquery:, xheader:) — the decoy must NOT be matched, and
// the doc tag's decoy (godoc:) must not hijack the description either.
type GetItemReq struct {
	ID    string `queryparam:"decoy-id" param:"id" godoc:"decoy doc" doc:"the item id"`
	Page  string `notquery:"decoy-page" query:"page"`
	Trace string `xheader:"X-Decoy" header:"X-Trace-Id"`
}

// CreateItemReq exercises the json/validate decoy pair plus an escaped quote
// in a doc tag and an example tag on a string field (Note is string-typed so
// the generator's untyped example assignment stays valid OpenAPI).
type CreateItemReq struct {
	Name string `myjson:"decoyName" json:"name" novalidate:"nope" validate:"required"`
	Note string `json:"note" doc:"say \"hi\" now" myexample:"decoy" example:"a note"`
}

// Item is returned by both routes.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/v1/items/:id", m.getItem, server.WithTags("items"))
	server.POST(hr, r, "/v1/items", m.createItem, server.WithTags("items"))
}

func (m *Module) getItem(req GetItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.NewResult(http.StatusOK, Item{}), nil
}

func (m *Module) createItem(req CreateItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.NewResult(http.StatusOK, Item{}), nil
}
