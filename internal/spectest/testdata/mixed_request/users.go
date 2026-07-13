// Package users exercises a request struct that mixes path/header params with
// JSON body fields — the body schema must contain ONLY the body fields.
package users

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the users module.
type Module struct{}

func (m *Module) Name() string                    { return "users" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// UpdateUserReq carries a path param, a header param, and a JSON body.
type UpdateUserReq struct {
	ID    string `param:"id" validate:"required"`
	Token string `header:"X-Api-Token"`
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

// User is the response payload.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.PUT(hr, r, "/users/:id", m.update, server.WithTags("users"))
}

func (m *Module) update(req UpdateUserReq, ctx server.HandlerContext) (server.Result[User], server.IAPIError) {
	return server.NewResult(200, User{}), nil
}
