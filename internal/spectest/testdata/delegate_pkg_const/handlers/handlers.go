// Package handlers registers the widget routes on paths named by its own
// constants.
package handlers

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

const widgetPath = "/widgets/:id"

// Handler registers and serves the widget routes.
type Handler struct{}

// Widget is the widget resource.
type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name" validate:"required"`
}

type getWidgetRequest struct {
	ID string `param:"id" validate:"required"`
}

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, widgetPath, h.Get)
	server.POST(hr, r, widgetsPath, h.create)
	admin := r.Group(adminPrefix)
	server.GET(hr, admin, widgetPath, h.Get)
}

func (h *Handler) Get(req getWidgetRequest, ctx server.HandlerContext) (server.Result[Widget], server.IAPIError) {
	return server.NewResult(http.StatusOK, Widget{ID: req.ID}), nil
}

func (h *Handler) create(req Widget, ctx server.HandlerContext) (server.Result[Widget], server.IAPIError) {
	return server.Created(req), nil
}
