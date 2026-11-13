// Package duplicateroutes registers several pairs of routes that go-bricks
// treats as one route node. The first registration of each pair is documented
// and the second dropped; a payload only a dropped route registers is not
// emitted.
package duplicateroutes

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the duplicate-routes module.
type Module struct {
	legacy bool
}

func (m *Module) Name() string                    { return "duproutes" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// A is the first /dup registration's payload.
type A struct {
	X int `json:"x"`
}

// Office is reachable only from B, the dropped /dup registration's payload.
type Office struct {
	Floor int `json:"floor"`
}

// B is the dropped /dup registration's payload.
type B struct {
	O Office `json:"o"`
}

// GetU is the first /u/:id registration's request.
type GetU struct {
	ID string `param:"id"`
}

// GetU2 is the dropped /u/:uid registration's request.
type GetU2 struct {
	UID string `param:"uid"`
}

// V1 is the if-branch /feat payload.
type V1 struct {
	Old string `json:"old"`
}

// V2 is the else-branch /feat payload.
type V2 struct {
	New string `json:"new"`
}

// Shared is /keep's payload and also the dropped /share registration's.
type Shared struct {
	S string `json:"s"`
}

// CreateOne is the first /make registration's request body.
type CreateOne struct {
	Name string `json:"name"`
}

// CreateTwo is the dropped /make registration's request body.
type CreateTwo struct {
	Title string `json:"title"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/dup", m.first)
	server.GET(hr, r, "/dup", m.second)
	server.GET(hr, r, "/u/:id", m.u1)
	server.GET(hr, r, "/u/:uid", m.u2)
	if m.legacy {
		server.GET(hr, r, "/feat", m.v1)
	} else {
		server.GET(hr, r, "/feat", m.v2)
	}
	server.GET(hr, r, "/files/*", m.keep)
	server.GET(hr, r, "/files/*rest", m.keep)
	server.GET(hr, r, "/keep", m.keep)
	server.GET(hr, r, "/share", m.first)
	server.GET(hr, r, "/share", m.keep)
	server.POST(hr, r, "/make", m.make1)
	server.POST(hr, r, "/make", m.make2)
}

func (m *Module) first(ctx server.HandlerContext) (server.Result[A], server.IAPIError) {
	return server.OK(A{}), nil
}

func (m *Module) second(ctx server.HandlerContext) (server.Result[B], server.IAPIError) {
	return server.OK(B{}), nil
}

func (m *Module) u1(req GetU, ctx server.HandlerContext) (server.Result[A], server.IAPIError) {
	return server.OK(A{}), nil
}

func (m *Module) u2(req GetU2, ctx server.HandlerContext) (server.Result[A], server.IAPIError) {
	return server.OK(A{}), nil
}

func (m *Module) v1(ctx server.HandlerContext) (server.Result[V1], server.IAPIError) {
	return server.OK(V1{}), nil
}

func (m *Module) v2(ctx server.HandlerContext) (server.Result[V2], server.IAPIError) {
	return server.OK(V2{}), nil
}

func (m *Module) keep(ctx server.HandlerContext) (server.Result[Shared], server.IAPIError) {
	return server.OK(Shared{}), nil
}

func (m *Module) make1(req CreateOne, ctx server.HandlerContext) (server.Result[A], server.IAPIError) {
	return server.Created(A{}), nil
}

func (m *Module) make2(req CreateTwo, ctx server.HandlerContext) (server.Result[A], server.IAPIError) {
	return server.Created(A{}), nil
}
