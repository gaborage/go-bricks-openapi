// Package namedresolution demonstrates how local named non-struct types are
// documented: each emits exactly what its hand-expanded type emits, in every
// field position.
package namedresolution

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module wires the resolved route.
type Module struct{}

func (m *Module) Name() string                    { return "namedresolution" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/resolved", m.get, server.WithTags("resolved"))
}

func (m *Module) get(req ResolvedQuery, ctx server.HandlerContext) (server.Result[Resolved], server.IAPIError) {
	return server.NewResult(http.StatusOK, Resolved{}), nil
}
