// Package groupjoin registers routes whose group prefix and route path must
// be joined the way go-bricks' RouteRegistrar.FullPath joins them, not
// concatenated: a missing leading slash is added, a trailing slash on a group
// prefix is trimmed, "/" and "" name the group itself, and a route path that
// repeats its group's whole prefix has it stripped (never a partial word).
package groupjoin

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the groupjoin module.
type Module struct{}

func (m *Module) Name() string                    { return "groupjoin" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// In is a request type with no serializable properties.
type In struct{}

// Out is the shared response type.
type Out struct {
	OK bool `json:"ok"`
}

// RegisterRoutes registers routes covering each go-bricks join rule.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	g := r.Group("/api")
	server.GET(hr, g, "/", m.root)          // /api
	server.GET(hr, g, "grp", m.grp)         // /api/grp
	server.GET(hr, g, "/apix", m.apix)      // /api/apix (no partial-word strip)
	server.GET(hr, g, "//x", m.doubleSlash) // /api//x
	server.GET(hr, g, "/api/x", m.strip2)   // /api/x (whole prefix stripped)
	server.GET(hr, g, "/api/x2", m.strip3)  // /api/x2 (whole prefix stripped)
	g.Add(http.MethodPost, "add", m.raw)    // /api/add

	server.GET(hr, r, "/", m.home) // /
	t := r.Group("v2/")
	server.GET(hr, t, "/y", m.trail) // /v2/y

	b := r.Group("/b")
	server.GET(hr, b, "", m.empty) // /b
	c := r.Group("/c")
	server.GET(hr, c, "/c", m.strip) // /c (whole prefix stripped)

	n := g.Group("/v1")
	server.GET(hr, n, "/api/v1/s", m.nestStrip) // /api/v1/s
	nt := g.Group("v3/")
	server.GET(hr, nt, "/w", m.nestTrail) // /api/v3/w

	server.GET(hr, r, "nolead", m.lone) // /nolead
	s := r.Group("/")
	server.GET(hr, s, "/sg", m.slashGroup) // /sg
}

func (m *Module) root(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) grp(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) apix(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) doubleSlash(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) empty(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) strip(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) nestStrip(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) nestTrail(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) lone(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) slashGroup(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) strip2(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) strip3(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) home(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) trail(req In, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) raw(ctx server.HandlerContext) error { return nil }
