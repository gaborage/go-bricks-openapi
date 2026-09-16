// Package packets demonstrates how Go's predeclared scalar aliases (byte, rune,
// uintptr) and their named wrappers are documented in the generated schema.
package packets

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module wires the packets routes to a Handler.
type Module struct {
	h *Handler
}

func (m *Module) Name() string                    { return "packets" }
func (m *Module) Init(deps *app.ModuleDeps) error { m.h = &Handler{}; return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/packets/:id", m.h.getPacket, server.WithTags("packets"))
	server.GET(hr, r, "/packets/seen", m.h.listSeen, server.WithTags("packets"))
	server.GET(hr, r, "/packets/due", m.h.listDue, server.WithTags("packets"))
}
