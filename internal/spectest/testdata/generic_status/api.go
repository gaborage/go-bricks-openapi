// Package genericstatus pins issue 138's status change: a handler with a
// generic request parameter, whatever its payload, or a request-less handler
// whose payload holds a generic instantiation, is recognised as the route's
// handler, so its result constructor and the error constructors its body
// calls are documented like any other handler's.
//
// Before issue 138 none of the handlers here was recognised: the Box[Address]
// request vanished, and the Page[Address] and anonymous struct payloads
// resolved to nothing, so every route fell back to the default 200 with no
// inferred error response. Now each documents 201 (server.Created) and 404
// (server.NewNotFoundError). The request still gets no requestBody and a
// Page[Address] payload's data stays an untyped object: substituting the type
// arguments is issue 114's. The anonymous struct payload is documented inline
// (issue 118). Goldens do not capture warnings; the analyzer tests carry that
// proof.
package genericstatus

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the genericstatus module.
type Module struct{}

func (m *Module) Name() string                    { return "genericstatus" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Address is a plain struct type argument.
type Address struct {
	City string `json:"city"`
}

// Box is a generic struct.
type Box[T any] struct {
	V T `json:"v"`
}

// Page is a generic struct.
type Page[T any] struct {
	Items []T `json:"items"`
}

// RegisterRoutes registers a generic request route, a request-less generic
// payload route, and a generic request route with an anonymous struct
// payload.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/boxes", m.createBox, server.WithTags("generic"))
	server.POST(hr, r, "/pages", m.createPage, server.WithTags("generic"))
	server.POST(hr, r, "/receipts", m.createReceipt, server.WithTags("generic"))
}

func (m *Module) createBox(req Box[Address], ctx server.HandlerContext) (server.Result[Page[Address]], server.IAPIError) {
	if req.V.City == "" {
		return server.Result[Page[Address]]{}, server.NewNotFoundError("city not found")
	}
	return server.Created(Page[Address]{}), nil
}

func (m *Module) createPage(ctx server.HandlerContext) (server.Result[Page[Address]], server.IAPIError) {
	if ctx.Path() == "" {
		return server.Result[Page[Address]]{}, server.NewNotFoundError("page not found")
	}
	return server.Created(Page[Address]{}), nil
}

func (m *Module) createReceipt(req Box[Address], ctx server.HandlerContext) (server.Result[struct {
	ID string `json:"id"`
}], server.IAPIError) {
	if req.V.City == "" {
		return server.Result[struct {
			ID string `json:"id"`
		}]{}, server.NewNotFoundError("city not found")
	}
	return server.Created(struct {
		ID string `json:"id"`
	}{}), nil
}
