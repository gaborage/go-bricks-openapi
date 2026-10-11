// Package multipointer demonstrates payloads whose written root has two or
// more pointer levels: encoding/json follows every pointer, so each is
// documented exactly as its *T (and T) form, never nullable at the root.
package multipointer

import (
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "multipointer" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Address is reached only through **Address payloads; it still gets its
// component.
type Address struct {
	City string `json:"city"`
}

// Cents is a named scalar, documented as its underlying int64.
type Cents int64

// GetCountReq is a typed request: beside it, a **T payload used to be
// untyped with no warning at all.
type GetCountReq struct {
	ID string `param:"id"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/counts/:id", m.count, server.WithTags("multipointer"))
	server.GET(hr, r, "/addresses/primary", m.primary, server.WithTags("multipointer"))
	server.GET(hr, r, "/tags", m.tags, server.WithTags("multipointer"))
	server.GET(hr, r, "/balance", m.balance, server.WithTags("multipointer"))
	server.GET(hr, r, "/now", m.now, server.WithTags("multipointer"))
	server.GET(hr, r, "/legacy/address", m.legacy, server.WithTags("multipointer"), server.WithRawResponse())
}

func (m *Module) count(req GetCountReq, ctx server.HandlerContext) (server.Result[**int64], server.IAPIError) { // data: {integer, int64}
	return server.Result[**int64]{}, nil
}

func (m *Module) primary(ctx server.HandlerContext) (server.Result[**Address], server.IAPIError) { // data: $ref Address
	return server.Result[**Address]{}, nil
}

func (m *Module) tags(ctx server.HandlerContext) (server.Result[**[]string], server.IAPIError) { // data: array of string
	return server.Result[**[]string]{}, nil
}

func (m *Module) balance(ctx server.HandlerContext) (server.ResultWithMeta[***Cents], server.IAPIError) { // data: {integer, int64}
	return server.ResultWithMeta[***Cents]{}, nil
}

func (m *Module) now(ctx server.HandlerContext) (**time.Time, server.IAPIError) { // data: {string, date-time}
	return nil, nil
}

func (m *Module) legacy(ctx server.HandlerContext) (server.Result[**Address], server.IAPIError) { // body: $ref Address
	return server.Result[**Address]{}, nil
}
