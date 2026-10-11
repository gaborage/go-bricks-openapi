// Package anonymousstruct exercises anonymous struct fields (issue 118):
// each is documented inline, as an object with the properties, required
// list and validate keywords a named struct of the same fields gets, in
// every position, and never as a component.
package anonymousstruct

import (
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
)

// Module is the anonymous-struct demo module.
type Module struct{}

func (m *Module) Name() string                    { return "anonymousstruct" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Home is only promoted into an anonymous struct, so it is no component.
type Home struct {
	City string `json:"city"`
}

// Office and Ack are reachable only through anonymous structs, and still
// register, so their $refs resolve.
type Office struct {
	Floor int `json:"floor"`
}

// Ack is an empty struct reached only through an anonymous struct.
type Ack struct{}

// Rows is a named slice over an anonymous struct.
type Rows []struct {
	ID   int64  `json:"id"`
	Name string `json:"name" validate:"max=20"`
}

// Node contains itself through an anonymous struct that embeds it: the
// anonymous struct met again is cut to {} one level down, with a warning.
type Node struct {
	Kids []struct {
		*Node
		Label string `json:"label"`
	} `json:"kids"`
}

// CreateReq's body holds an anonymous struct whose fields keep their
// validate keywords. Its query parameter of anonymous struct type is not
// extracted: it stays the bare object it was, keeping its enum.
type CreateReq struct {
	ID   int64 `param:"id"`
	Mode struct {
		A int `json:"a"`
	} `query:"mode" validate:"oneof=a b"`
	Meta struct {
		X   int    `json:"x" validate:"min=1"`
		Tag string `json:"tag,omitempty" validate:"required,max=8"`
	} `json:"meta" validate:"required"`
}

// Report holds an anonymous struct in every position.
type Report struct {
	Direct struct {
		A int `json:"a"`
	} `json:"direct"`
	Ptr *struct {
		A int `json:"a"`
	} `json:"ptr" doc:"An optional block"`
	List []struct {
		A int `json:"a"`
	} `json:"list" validate:"min=1"`
	ByKey map[string]struct {
		A int `json:"a"`
	} `json:"byKey"`
	Nested struct {
		In struct {
			B string `json:"b"`
		} `json:"in"`
	} `json:"nested"`
	Promoted struct {
		Home
		Z int `json:"z"`
	} `json:"promoted"`
	Refs struct {
		O Office `json:"o"`
		A Ack    `json:"a"`
	} `json:"refs"`
	Skips struct {
		Kept     int `json:"kept,omitempty"`
		Dropped  int `json:"-"`
		hidden   int
		Untagged bool
	} `json:"skips"`
	Stamp struct {
		time.Time
		Z int
	} `json:"stamp"`
	ID struct {
		uuid.UUID
	} `json:"id"`
	Rows  Rows     `json:"rows"`
	Tree  Node     `json:"tree"`
	Empty struct{} `json:"empty"`
}

// RegisterRoutes registers the request and response routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/reports/:id", m.create, server.WithTags("anonymous"))
	server.GET(hr, r, "/report", m.report, server.WithTags("anonymous"))
}

func (m *Module) create(req CreateReq, ctx server.HandlerContext) (server.Result[Report], server.IAPIError) {
	return server.Created(Report{}), nil
}

func (m *Module) report(ctx server.HandlerContext) (server.Result[Report], server.IAPIError) {
	return server.OK(Report{}), nil
}
