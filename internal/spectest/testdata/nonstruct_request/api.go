// Package nonstructreq pins #102: a handler whose request type is not a
// struct documents no requestBody and orphans no component.
//
// go-bricks binds every request by its struct fields and panics at request
// time on any other type (F26, gaborage/go-bricks#1811), so the analyzer warns
// about each of these routes and leaves the request untyped. Goldens do not
// capture warnings — the analyzer and command tests carry that proof. This
// golden locks the output half: no `requestBody` key on any operation, and no
// component for Status or any other request type; only the Receipt response
// and the standard shared components are emitted.
package nonstructreq

import (
	"encoding/json"
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the nonstructreq module.
type Module struct{}

func (m *Module) Name() string                    { return "nonstructreq" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Status is a local named scalar request type.
type Status string

// Receipt is the response payload.
type Receipt struct {
	ID string `json:"id"`
}

// RegisterRoutes registers one route per non-struct request family.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/builtin", m.builtin, server.WithTags("nonstructreq"))
	server.POST(hr, r, "/well-known", m.wellKnown, server.WithTags("nonstructreq"))
	server.POST(hr, r, "/named-scalar", m.namedScalar, server.WithTags("nonstructreq"))
	server.POST(hr, r, "/slice", m.slice, server.WithTags("nonstructreq"))
	server.POST(hr, r, "/map", m.mapReq, server.WithTags("nonstructreq"))
}

func (m *Module) builtin(req *string, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) wellKnown(req json.RawMessage, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) namedScalar(req Status, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) slice(req []Receipt, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) mapReq(req map[string]string, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}
