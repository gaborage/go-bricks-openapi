// Package api demonstrates named non-struct type resolution: an alias to a
// struct (must emit a full $ref'ed component) and a named slice (which has no
// struct shape of its own, so it must emit an untyped envelope rather than a
// dangling $ref).
package api

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the API module.
type Module struct{}

func (m *Module) Name() string                    { return "api" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// User is the user resource.
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserResp is an alias to User — the route below references the alias
// directly, and it must resolve to User's fields under the alias's own name.
type UserResp = User

// UserList is a named slice of User. It has no struct shape of its own, so it
// cannot resolve to a component; the route below must emit an untyped
// response envelope instead of a dangling $ref to "UserList".
type UserList []User

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/user", m.getUser, server.WithTags("users"), server.WithSummary("Get a user"))
	server.GET(hr, r, "/users", m.listUsers, server.WithTags("users"), server.WithSummary("List users"))
}

func (m *Module) getUser(ctx server.HandlerContext) (server.Result[UserResp], server.IAPIError) {
	return server.NewResult(http.StatusOK, UserResp{}), nil
}

func (m *Module) listUsers(ctx server.HandlerContext) (server.Result[UserList], server.IAPIError) {
	return server.NewResult(http.StatusOK, UserList{}), nil
}
