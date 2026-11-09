// Package api returns slice and array payloads of a project struct named like
// a well-known type: each is an array of $ref to the struct's component, never
// the well-known type's array. A genuine time.Time slice stays inline.
package api

import (
	"time"

	"github.com/example/wellknownnamedslice/uuid"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the api module.
type Module struct{}

func (m *Module) Name() string                    { return "api" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/list", m.list, server.WithTags("api"))
	server.GET(hr, r, "/ptrlist", m.ptrList, server.WithTags("api"))
	server.GET(hr, r, "/arr", m.arr, server.WithTags("api"))
	server.GET(hr, r, "/times", m.times, server.WithTags("api"))
}

func (m *Module) list(ctx server.HandlerContext) (server.Result[[]uuid.UUID], server.IAPIError) {
	return server.OK([]uuid.UUID{}), nil
}

func (m *Module) ptrList(ctx server.HandlerContext) (server.Result[[]*uuid.UUID], server.IAPIError) {
	return server.OK([]*uuid.UUID{}), nil
}

func (m *Module) arr(ctx server.HandlerContext) (server.Result[[2]uuid.UUID], server.IAPIError) {
	return server.OK([2]uuid.UUID{}), nil
}

func (m *Module) times(ctx server.HandlerContext) (server.Result[[]time.Time], server.IAPIError) {
	return server.OK([]time.Time{}), nil
}
