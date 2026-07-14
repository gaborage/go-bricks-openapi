// Package emptyresp exercises a handler whose response type has no serializable
// properties — the referenced component must still be emitted so the $ref
// resolves and the document is valid OpenAPI 3.0.
package emptyresp

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the emptyresp module.
type Module struct{}

func (m *Module) Name() string                    { return "emptyresp" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// DeleteItemReq identifies the item to delete by path parameter.
type DeleteItemReq struct {
	ID int64 `param:"id" validate:"required"`
}

// DeleteAck is an empty acknowledgement payload (no serializable fields).
type DeleteAck struct{}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.DELETE(hr, r, "/items/:id", m.remove, server.WithTags("emptyresp"))
}

func (m *Module) remove(req DeleteItemReq, ctx server.HandlerContext) (server.Result[DeleteAck], server.IAPIError) {
	return server.NewResult(200, DeleteAck{}), nil
}
