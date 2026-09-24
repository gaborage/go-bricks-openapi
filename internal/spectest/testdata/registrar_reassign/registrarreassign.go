// Package registrarreassign registers routes on group registrars that are
// reassigned or shadowed. Each registration must carry the prefix of the
// registrar binding in effect at its own position, not the last Group(...)
// assigned anywhere in the body: a same-block `=` reassignment is resolved
// exactly, a group derived after a reassignment composes onto the current
// binding, and an inner `:=` shadows the outer registrar only inside its own
// block. A reassignment inside an `if` stands inside that block only, and its
// `else` still sees the binding from before the statement. The last
// registration follows that reassignment, so its prefix depends on control
// flow: it stays an Unresolved route and is absent from the golden document.
package registrarreassign

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the registrarreassign module.
type Module struct {
	legacy bool
}

func (m *Module) Name() string                    { return "registrarreassign" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// PingReq is a request type with no serializable properties.
type PingReq struct{}

// PingResp is the shared response type for every route below.
type PingResp struct {
	OK bool `json:"ok"`
}

// RegisterRoutes registers one route per registrar-binding case.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	// Straight-line reassignment: each route sees the binding above it.
	api := r.Group("/a")
	server.GET(hr, api, "/first", m.h, server.WithTags("api"))
	api = r.Group("/b")
	server.GET(hr, api, "/second", m.h, server.WithTags("api"))

	// A group opened after the reassignment composes onto the current binding.
	v1 := api.Group("/v1")
	server.GET(hr, v1, "/derived", m.h, server.WithTags("api"))

	// An inner `:=` shadows the outer registrar only inside its own block.
	scoped := r.Group("/outer")
	if m.legacy {
		scoped := r.Group("/inner")
		server.GET(hr, scoped, "/shadowed", m.h, server.WithTags("api"))
	}
	server.GET(hr, scoped, "/unshadowed", m.h, server.WithTags("api"))

	// A reassignment from a nested block stands inside that block only; the
	// else branch still sees the binding from before the if.
	flagged := r.Group("/stable")
	server.GET(hr, flagged, "/before", m.h, server.WithTags("api"))
	if m.legacy {
		flagged = r.Group("/legacy")
		server.GET(hr, flagged, "/inside", m.h, server.WithTags("api"))
	} else {
		server.GET(hr, flagged, "/otherwise", m.h, server.WithTags("api"))
	}

	// Deliberately unresolved: whether the block above ran decides the prefix,
	// so this route is dropped with a warning rather than emitted under a guess.
	server.GET(hr, flagged, "/after", m.h, server.WithTags("api"))
}

func (m *Module) h(req PingReq, ctx server.HandlerContext) (server.Result[PingResp], server.IAPIError) {
	return server.NewResult(http.StatusOK, PingResp{}), nil
}
