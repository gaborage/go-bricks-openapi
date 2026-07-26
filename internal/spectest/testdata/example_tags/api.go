// Package exampletags is a regression fixture for Plan 027 (coercing
// `example:` tag values to their declared schema type). CreateItemReq exercises
// every coercion outcome in one request type — kept (Count, Big, Ratio, Flag,
// Name) and dropped (Bad, Huge, Kids) — and ListItemsReq's Limit exercises a
// parameter-level example, which must mirror the schema's coerced value.
package exampletags

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the exampletags module.
type Module struct{}

func (m *Module) Name() string                    { return "exampletags" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// ListItemsReq carries a query parameter whose example is a plain in-range
// integer — kept, and emitted on both the parameter and its schema.
type ListItemsReq struct {
	Limit int `query:"limit" example:"25"`
}

// CreateItemReq covers, field by field: a plain integer example (Count), an
// int64 example large enough to overflow int32 (Big), a float example
// (Ratio), a boolean example (Flag), a string example (Name, which stays a
// string), a non-numeric example on an integer field (Bad, dropped), an
// int-typed (int32-format) field whose example overflows that format (Huge,
// dropped), a scalar example on a slice field (Kids, dropped), and an example
// exactly at an exclusive minimum (Amount, dropped — `gt=10` emits minimum: 10
// plus exclusiveMinimum: true, and an example of exactly 10 is invalid there).
type CreateItemReq struct {
	Count  int      `json:"count" example:"3"`
	Big    int64    `json:"big" example:"3000000000"`
	Ratio  float64  `json:"ratio" example:"2.5"`
	Flag   bool     `json:"flag" example:"true"`
	Name   string   `json:"name" example:"bob"`
	Bad    int      `json:"bad" example:"abc"`
	Huge   int      `json:"huge" example:"3000000000"`
	Kids   []string `json:"kids" example:"nope"`
	Amount float64  `json:"amount" validate:"gt=10" example:"10"`
}

// Item is returned by both routes.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/v1/items", m.listItems, server.WithTags("items"))
	server.POST(hr, r, "/v1/items", m.createItem, server.WithTags("items"))
}

func (m *Module) listItems(req ListItemsReq, ctx server.HandlerContext) (server.Result[[]Item], server.IAPIError) {
	return server.NewResult(http.StatusOK, []Item{}), nil
}

func (m *Module) createItem(req CreateItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.NewResult(http.StatusOK, Item{}), nil
}
