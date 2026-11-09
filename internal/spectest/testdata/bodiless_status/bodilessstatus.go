// Package bodilessstatus exercises routes whose resolved success status is
// 204. go-bricks writes a 204 with no body whatever the payload, so each such
// route documents a bodiless "No Content" 204 like server.NoContent(), and a
// payload type reachable only from them gets no component. Shared is also
// returned by a 200 route, so it keeps its component and its $ref.
package bodilessstatus

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the bodiless_status module.
type Module struct{}

func (m *Module) Name() string                    { return "bodiless_status" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// IDReq carries the item id path parameter.
type IDReq struct {
	ID string `param:"id"`
}

// Gone is used only as the payload of 204 routes.
type Gone struct {
	ID string `json:"id"`
}

// Shared is the payload of one 204 route and one 200 route.
type Shared struct {
	Name string `json:"name"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.PUT(hr, r, "/items/:id", m.put204)
	server.PATCH(hr, r, "/items/:id", m.patch204)
	server.POST(hr, r, "/items/:id/archive", m.archive)
	server.POST(hr, r, "/ping", m.ping)
	server.DELETE(hr, r, "/items/:id", m.del)
	server.POST(hr, r, "/items/:id/touch", m.touch)
	server.GET(hr, r, "/items/:id", m.get)
}

// put204 returns NewResult with http.StatusNoContent -> bodiless 204.
func (m *Module) put204(req IDReq, ctx server.HandlerContext) (server.Result[Gone], server.IAPIError) {
	return server.NewResult(http.StatusNoContent, Gone{ID: req.ID}), nil
}

// patch204 returns NewResultWithMeta with http.StatusNoContent -> bodiless 204.
func (m *Module) patch204(req IDReq, ctx server.HandlerContext) (server.ResultWithMeta[Gone], server.IAPIError) {
	return server.NewResultWithMeta(http.StatusNoContent, Gone{ID: req.ID}, nil), nil
}

// archive binds a Result literal with Status 204 -> bodiless 204.
func (m *Module) archive(req IDReq, ctx server.HandlerContext) (server.Result[Gone], server.IAPIError) {
	res := server.Result[Gone]{Status: http.StatusNoContent}
	return res, nil
}

// ping takes no request and returns NewResult(204, x) -> bodiless 204, typed.
func (m *Module) ping(ctx server.HandlerContext) (server.Result[Gone], server.IAPIError) {
	return server.NewResult(204, Gone{}), nil
}

// del is the control: server.NoContent() -> bodiless 204.
func (m *Module) del(req IDReq, ctx server.HandlerContext) (server.NoContentResult, server.IAPIError) {
	return server.NoContent(), nil
}

// touch returns Shared with 204 -> bodiless 204.
func (m *Module) touch(req IDReq, ctx server.HandlerContext) (server.Result[Shared], server.IAPIError) {
	return server.NewResult(http.StatusNoContent, Shared{Name: req.ID}), nil
}

// get returns Shared with 200 -> Shared keeps its component.
func (m *Module) get(req IDReq, ctx server.HandlerContext) (server.Result[Shared], server.IAPIError) {
	return server.NewResult(http.StatusOK, Shared{Name: req.ID}), nil
}
