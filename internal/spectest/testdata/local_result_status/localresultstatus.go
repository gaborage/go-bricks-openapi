// Package localresultstatus exercises success-status extraction when the
// handler binds its result to a local variable before returning it. A direct
// `return server.Accepted(x), nil` was already recognised; every shape that
// goes through a local (to set a header, or to build the Result literal by
// hand) used to fall back to 200. The bindings below pin each recognised shape
// plus the deliberately conservative ambiguous case.
package localresultstatus

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the local_result_status module.
type Module struct{}

func (m *Module) Name() string                    { return "local_result_status" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// ItemReq is the request body.
type ItemReq struct {
	Name string `json:"name"`
}

// ItemResponse is the response body.
type ItemResponse struct {
	ID string `json:"id"`
}

func toResponse(req ItemReq) ItemResponse { return ItemResponse{ID: req.Name} }

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/direct", m.direct, server.WithTags("shapes"))
	server.POST(hr, r, "/bound-mutated", m.boundMutated, server.WithTags("shapes"))
	server.POST(hr, r, "/bound-plain", m.boundPlain, server.WithTags("shapes"))
	server.POST(hr, r, "/new-result", m.newResult, server.WithTags("shapes"))
	server.DELETE(hr, r, "/no-content", m.noContent, server.WithTags("shapes"))
	server.POST(hr, r, "/literal-status-write", m.literalStatusWrite, server.WithTags("shapes"))
	server.POST(hr, r, "/ambiguous", m.ambiguous, server.WithTags("shapes"))
}

// direct returns the constructor call directly -> 202.
func (m *Module) direct(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	return server.Accepted(toResponse(req)), nil
}

// boundMutated assigns to a local, mutates headers, then returns the local -> 202.
func (m *Module) boundMutated(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	res := server.Accepted(toResponse(req))
	res.Headers = http.Header{"Location": []string{"/x"}}
	return res, nil
}

// boundPlain assigns to a local and returns it unmutated -> 201.
func (m *Module) boundPlain(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	res := server.Created(toResponse(req))
	return res, nil
}

// newResult uses NewResult with an http status constant -> 202.
func (m *Module) newResult(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	return server.NewResult(http.StatusAccepted, toResponse(req)), nil
}

// noContent returns a bodyless NoContentResult -> 204, no body.
func (m *Module) noContent(req ItemReq, ctx server.HandlerContext) (server.NoContentResult, server.IAPIError) {
	return server.NoContent(), nil
}

// literalStatusWrite builds a composite literal and sets Status afterwards -> 201.
func (m *Module) literalStatusWrite(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	res := server.Result[ItemResponse]{Data: toResponse(req)}
	res.Status = http.StatusCreated
	return res, nil
}

// ambiguous binds res twice, so the analyzer refuses to guess and the
// generator falls back to 200. This is the documented conservative behaviour:
// no branch-sensitive flow analysis is attempted.
func (m *Module) ambiguous(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	res := server.Created(toResponse(req))
	if req.Name == "" {
		res = server.Accepted(toResponse(req))
	}
	return res, nil
}
