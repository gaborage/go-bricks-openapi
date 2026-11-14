// Package other is a second module whose GET /dup is the same route node as
// the root module's; module discovery meets the root module first.
package other

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the second module.
type Module struct{}

func (m *Module) Name() string                    { return "other" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Epsilon is the dropped cross-module /dup registration's payload. It was
// registered before the duplicate was found, so it stays emitted.
type Epsilon struct {
	E string `json:"e"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/dup", m.eps)
}

func (m *Module) eps(ctx server.HandlerContext) (server.Result[Epsilon], server.IAPIError) {
	return server.OK(Epsilon{}), nil
}
