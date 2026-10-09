// Package namedfallback demonstrates the field types that still fall back to
// an untyped schema, each with an analyzer warning: Marshaler types,
// recursive types, unresolvable types, untyped builtins and uintptr wrappers.
package namedfallback

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module wires the fallback route.
type Module struct{}

func (m *Module) Name() string                    { return "namedfallback" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/fallback", m.get, server.WithTags("fallback"))
}

func (m *Module) get(ctx server.HandlerContext) (server.Result[Fallback], server.IAPIError) {
	return server.NewResult(http.StatusOK, Fallback{}), nil
}
