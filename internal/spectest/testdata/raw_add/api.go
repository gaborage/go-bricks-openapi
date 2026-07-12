// Package rawadd exercises raw <registrar>.Add(...) route registration.
package rawadd

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the raw-add API module.
type Module struct{}

func (m *Module) Name() string                    { return "rawadd" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// RegisterRoutes registers raw routes via the registrar's Add method.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	// (1) root registrar: a bare route with no request/response body.
	r.Add(http.MethodGet, "/ping", m.ping)

	// (2) grouped registrar: inherits the "/v1" group prefix.
	api := r.Group("/v1")
	api.Add(http.MethodPost, "/things", m.createThing)

	// (3) negative discriminator: .Add on a non-registrar receiver must be
	// ignored. notReg is a local var absent from the registrar prefix map, so
	// the comma-ok gate drops it silently (no /nope path in the spec).
	notReg := &sink{}
	notReg.Add(http.MethodGet, "/nope", m.ping)
}

// sink is a non-registrar type with an Add method, to prove the registrar gate
// excludes .Add calls on unrelated receivers.
type sink struct{}

func (s *sink) Add(method, path string, handler any) {}

func (m *Module) ping(ctx server.HandlerContext) error       { return nil }
func (m *Module) createThing(ctx server.HandlerContext) error { return nil }
