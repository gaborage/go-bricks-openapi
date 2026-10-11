// Package anonymouspayload exercises anonymous struct payloads and requests
// (issue 118). A payload of anonymous struct type, wrapped in Result or
// ResultWithMeta, returned bare or raw, behind a pointer, as slice items or
// map values, or through a named slice, is documented inline exactly as a
// field of its type is, and never as a component: a struct reachable only
// through one still registers. A request-less handler returning one is
// recognised, so its status constructor and error responses are documented.
// An anonymous struct request is bound by go-bricks but not documented yet:
// no requestBody, no query parameter, and its path parameter falls back to a
// string. Goldens do not capture warnings; the analyzer tests carry that
// proof.
package anonymouspayload

import (
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the anonymous-payload demo module.
type Module struct{}

func (m *Module) Name() string                    { return "anonymouspayload" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Office is reachable only through an anonymous struct payload, and still
// registers, so its $ref resolves.
type Office struct {
	Floor int `json:"floor"`
}

// Rows is a named slice over an anonymous struct.
type Rows []struct {
	ID int64 `json:"id"`
}

// RegisterRoutes registers one route per payload position, a request-less
// route with constructor-derived statuses, and an anonymous request route.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/item", m.item, server.WithTags("anonymous"))
	server.GET(hr, r, "/meta", m.meta, server.WithTags("anonymous"))
	server.GET(hr, r, "/bare", m.bare, server.WithTags("anonymous"))
	server.GET(hr, r, "/raw", m.raw, server.WithTags("anonymous"), server.WithRawResponse())
	server.GET(hr, r, "/ptr", m.ptr, server.WithTags("anonymous"))
	server.GET(hr, r, "/list", m.list, server.WithTags("anonymous"))
	server.GET(hr, r, "/by-key", m.byKey, server.WithTags("anonymous"))
	server.GET(hr, r, "/rows", m.rows, server.WithTags("anonymous"))
	server.GET(hr, r, "/stamp", m.stamp, server.WithTags("anonymous"))
	server.POST(hr, r, "/acks", m.ack, server.WithTags("anonymous"))
	server.POST(hr, r, "/notes/:id", m.note, server.WithTags("anonymous"))
}

func (m *Module) item(ctx server.HandlerContext) (server.Result[struct {
	ID     int64  `json:"id"`
	Name   string `json:"name" validate:"required,max=20"`
	Office Office `json:"office"`
}], server.IAPIError) {
	return server.Result[struct {
		ID     int64  `json:"id"`
		Name   string `json:"name" validate:"required,max=20"`
		Office Office `json:"office"`
	}]{}, nil
}

func (m *Module) meta(ctx server.HandlerContext) (server.ResultWithMeta[struct {
	Total int `json:"total"`
}], server.IAPIError) {
	return server.ResultWithMeta[struct {
		Total int `json:"total"`
	}]{}, nil
}

func (m *Module) bare(ctx server.HandlerContext) (struct {
	OK bool `json:"ok"`
}, server.IAPIError) {
	return struct {
		OK bool `json:"ok"`
	}{}, nil
}

func (m *Module) raw(ctx server.HandlerContext) (server.Result[struct {
	A int `json:"a"`
}], server.IAPIError) {
	return server.Result[struct {
		A int `json:"a"`
	}]{}, nil
}

func (m *Module) ptr(ctx server.HandlerContext) (server.Result[*struct {
	A int `json:"a"`
}], server.IAPIError) {
	return server.Result[*struct {
		A int `json:"a"`
	}]{}, nil
}

func (m *Module) list(ctx server.HandlerContext) (server.Result[[]struct {
	A int `json:"a"`
}], server.IAPIError) {
	return server.Result[[]struct {
		A int `json:"a"`
	}]{}, nil
}

func (m *Module) byKey(ctx server.HandlerContext) (server.Result[map[string]struct {
	A int `json:"a"`
}], server.IAPIError) {
	return server.Result[map[string]struct {
		A int `json:"a"`
	}]{}, nil
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}

// stamp's payload embeds time.Time, so encoding/json writes it as a time
// string through the promoted MarshalJSON: an untyped schema, with a warning.
func (m *Module) stamp(ctx server.HandlerContext) (server.Result[struct {
	time.Time
	Z int
}], server.IAPIError) {
	return server.Result[struct {
		time.Time
		Z int
	}]{}, nil
}

// ack takes no request and returns an empty anonymous struct: 201 and 404.
func (m *Module) ack(ctx server.HandlerContext) (server.Result[struct{}], server.IAPIError) {
	if ctx.Path() == "" {
		return server.Result[struct{}]{}, server.NewNotFoundError("ack not found")
	}
	return server.Created(struct{}{}), nil
}

// note's request is an anonymous struct, bound by go-bricks but not
// documented yet.
func (m *Module) note(req struct {
	ID   int64  `param:"id"`
	Q    string `query:"q"`
	Body string `json:"body" validate:"required"`
}, ctx server.HandlerContext) (server.Result[struct {
	ID int64 `json:"id"`
}], server.IAPIError) {
	return server.Result[struct {
		ID int64 `json:"id"`
	}]{}, nil
}
