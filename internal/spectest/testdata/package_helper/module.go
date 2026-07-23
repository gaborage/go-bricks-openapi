// Package pkghelper demonstrates routes registered through a package-level
// helper function called directly from RegisterRoutes — a bare identifier
// call (registerUserRoutes(hr, r)), not a method call. This is the idiomatic
// pattern of splitting route registration into free functions.
package pkghelper

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module registers one route directly and delegates the rest to a
// package-level helper function invoked under a group prefix.
type Module struct{}

func (m *Module) Name() string                    { return "pkghelper" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// PingResponse is the health-check payload.
type PingResponse struct {
	Status string `json:"status"`
}

// User is the user resource.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name" validate:"required"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/ping", ping, server.WithTags("health"))
	v1 := r.Group("/v1")
	registerUserRoutes(hr, v1)
}

// registerUserRoutes is a package-level function (not a method) that
// registers the user routes. Called with the /v1 group registrar, so its
// routes must inherit that prefix.
func registerUserRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/users", createUser, server.WithTags("users"))
	server.GET(hr, r, "/users/:id", getUser, server.WithTags("users"))
}

func ping(ctx server.HandlerContext) (server.Result[PingResponse], server.IAPIError) {
	return server.NewResult(http.StatusOK, PingResponse{Status: "ok"}), nil
}

type getUserRequest struct {
	ID string `param:"id" validate:"required"`
}

func getUser(req getUserRequest, ctx server.HandlerContext) (server.Result[User], server.IAPIError) {
	return server.NewResult(http.StatusOK, User{ID: req.ID}), nil
}

func createUser(req User, ctx server.HandlerContext) (server.Result[User], server.IAPIError) {
	return server.Created(req), nil
}
