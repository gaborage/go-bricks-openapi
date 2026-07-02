// Package payments demonstrates the handler-field delegation pattern the
// go-bricks-demo-project uses: RegisterRoutes forwards the registry to a
// handler struct in an in-module sibling package, including through a group.
package payments

import (
	"github.com/example/delegation/internal/handlers"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module owns no routes itself — everything is delegated.
type Module struct {
	handler *handlers.PaymentHandler
}

func (m *Module) Name() string                    { return "payments" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.handler.RegisterRoutes(hr, r)
	v1 := r.Group("/v1")
	m.handler.RegisterAdmin(hr, v1)
}
