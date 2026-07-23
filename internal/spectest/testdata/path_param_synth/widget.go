// Package widget serves widget lookups whose handler reads its path variable
// via ctx.Param instead of struct-binding it — the request type declares only
// a query parameter. OpenAPI 3.0 still requires a declared "id" path
// parameter for the "/widgets/{id}" template; this fixture exercises the
// generator synthesizing one for a template variable no struct field covers.
package widget

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the widget module.
type Module struct{}

func (m *Module) Name() string                    { return "widget" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// GetWidgetReq carries only optional pagination. Deliberately no `param:"id"`
// field: the "id" path variable in the route template below has no declared
// parameter and must be synthesized by the generator.
type GetWidgetReq struct {
	Page int `query:"page"`
}

// Widget is a catalog widget.
type Widget struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/widgets/:id", m.getWidget, server.WithTags("widget"))
}

func (m *Module) getWidget(req GetWidgetReq, ctx server.HandlerContext) (server.Result[Widget], server.IAPIError) {
	// Reads the path variable directly instead of struct-binding it — the
	// idiom this fixture exercises.
	_ = ctx.Param("id")
	return server.NewResult(http.StatusOK, Widget{}), nil
}
