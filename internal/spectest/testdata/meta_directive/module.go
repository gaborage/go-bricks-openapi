// Package catalog exercises the //openapi:meta route Directive (issue 69):
// a ResultWithMeta route documents its handler-supplied meta keys with a
// named struct beside the framework's timestamp and traceId.
package catalog

import (
	"github.com/example/metadirective/pagination"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "catalog" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Item is one catalog entry.
type Item struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PageMeta is the offset-paging meta /items supplies.
type PageMeta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// Window is a time range, nested in WindowMeta.
type Window struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// WindowMeta is the meta /items/recent supplies: a nested struct field.
type WindowMeta struct {
	Window Window `json:"window"`
	Count  int    `json:"count"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	//openapi:meta PageMeta
	server.GET(hr, r, "/items", m.listItems)
	//openapi:meta pagination.Meta
	server.GET(hr, r, "/items/cursor", m.cursorItems)
	//openapi:meta WindowMeta
	server.GET(hr, r, "/items/recent", m.recentItems)
	server.GET(hr, r, "/items/all", m.allItems)
}

func (m *Module) listItems(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) {
	return server.ResultWithMeta[[]Item]{}, nil
}

func (m *Module) cursorItems(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) {
	return server.ResultWithMeta[[]Item]{}, nil
}

func (m *Module) recentItems(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) {
	return server.ResultWithMeta[[]Item]{}, nil
}

// allItems has no directive: its meta stays the framework's two keys.
func (m *Module) allItems(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) {
	return server.ResultWithMeta[[]Item]{}, nil
}
