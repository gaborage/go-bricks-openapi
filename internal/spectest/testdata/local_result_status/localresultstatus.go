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
	server.POST(hr, r, "/conditional-status", m.conditionalStatus, server.WithTags("shapes"))
	server.POST(hr, r, "/literal", m.literal, server.WithTags("shapes"))
	server.POST(hr, r, "/literal-meta", m.literalMeta, server.WithTags("shapes"))
	server.POST(hr, r, "/named-bare", m.namedBare, server.WithTags("shapes"))
	server.POST(hr, r, "/created-then-error", m.createdThenError, server.WithTags("shapes"))
	server.POST(hr, r, "/created-then-ok-literal", m.createdThenOKLiteral, server.WithTags("shapes"))
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

// conditionalStatus writes Status inside a branch, so the write may or may not
// happen. The analyzer refuses to document a status it cannot prove and the
// generator falls back to 200 — the same conservative rule as a double binding.
func (m *Module) conditionalStatus(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	res := server.Accepted(toResponse(req))
	if req.Name == "" {
		res.Status = http.StatusCreated
	}
	return res, nil
}

// literal returns a Result composite literal directly with a nil error -> 201.
func (m *Module) literal(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	return server.Result[ItemResponse]{Status: http.StatusCreated, Data: toResponse(req)}, nil
}

// literalMeta returns a ResultWithMeta composite literal directly -> 202.
func (m *Module) literalMeta(req ItemReq, ctx server.HandlerContext) (server.ResultWithMeta[ItemResponse], server.IAPIError) {
	return server.ResultWithMeta[ItemResponse]{Status: http.StatusAccepted, Data: toResponse(req)}, nil
}

// namedBare binds its first named result and returns with no results -> 201.
func (m *Module) namedBare(req ItemReq, ctx server.HandlerContext) (res server.Result[ItemResponse], apiErr server.IAPIError) {
	res = server.Created(toResponse(req))
	return
}

// createdThenError ends on a zero-value literal returned with a non-nil error.
// go-bricks ignores the result when the error is non-nil, so the literal does
// not override the earlier Created -> 201 (plus the inferred 404).
func (m *Module) createdThenError(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	if req.Name != "" {
		return server.Created(toResponse(req)), nil
	}
	return server.Result[ItemResponse]{}, server.NewNotFoundError("item")
}

// createdThenOKLiteral ends on a literal with no Status and a nil error. The
// last recognised return wins, and that literal is served as 200 -> 200.
func (m *Module) createdThenOKLiteral(req ItemReq, ctx server.HandlerContext) (server.Result[ItemResponse], server.IAPIError) {
	if req.Name == "new" {
		return server.Created(toResponse(req)), nil
	}
	return server.Result[ItemResponse]{Data: toResponse(req)}, nil
}
