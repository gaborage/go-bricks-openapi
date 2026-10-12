// Package widgets forwards route registration to a handler in an in-module
// sibling package whose route paths and group prefix are constants of that
// package, declared in the registering file and in a sibling file. Its own
// widgetPath, a name the handler package also declares, must not leak into
// the handler's routes.
package widgets

import (
	"net/http"

	"github.com/example/delegatepkgconst/handlers"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

const widgetPath = "/legacy/widgets/:id"

// Module owns one route and delegates the rest.
type Module struct {
	handler *handlers.Handler
}

func (m *Module) Name() string                    { return "widgets" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, widgetPath, m.getLegacy)
	m.handler.RegisterRoutes(hr, r)
}

type legacyRequest struct {
	ID string `param:"id"`
}

func (m *Module) getLegacy(req legacyRequest, ctx server.HandlerContext) (server.Result[handlers.Widget], server.IAPIError) {
	return server.NewResult(http.StatusOK, handlers.Widget{ID: req.ID}), nil
}
