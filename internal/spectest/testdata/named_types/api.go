// Package api demonstrates named non-struct type resolution: an alias to a
// struct (must emit a full $ref'ed component) and a named slice (documented as
// an array of $ref User, like a []User field, rather than a dangling $ref to a
// component of its own; #110).
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

// UserList is a named slice of User. It names no component of its own: the
// route below documents it as an array of $ref User, exactly as a []User
// field does (#110), and User is emitted.
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
