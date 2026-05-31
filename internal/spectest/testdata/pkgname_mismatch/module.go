// Package api demonstrates that an unaliased in-module import whose declared
// package name differs from its path base (transport/httpapi -> package http) is
// resolved by its DECLARED name. The handler references http.Money and the
// generator must emit a Money component schema with a $ref from the response.
package api

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module wires the routes to a Handler.
type Module struct {
	h *Handler
}

func (m *Module) Name() string                    { return "wallet" }
func (m *Module) Init(deps *app.ModuleDeps) error { m.h = &Handler{}; return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/balances/:id", m.h.getBalance, server.WithTags("wallet"))
}
