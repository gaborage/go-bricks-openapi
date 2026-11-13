// Package pointerresult pins issue 143: a handler returning a pointer to a
// result wrapper, *server.Result[T] or *server.ResultWithMeta[T], is
// documented exactly as its value form. go-bricks serves both alike: the
// wrappers' provider methods have value receivers, so the pointer satisfies
// the interfaces its response writer type-asserts.
//
// Before issue 143 every route here documented data as an untyped object and
// the default 200, and the request-less /receipt was not recognised as its
// route's handler at all, so it lost its 201 and 404. Now data is $ref'd, the
// success status is read through & (a returned &literal, &res, and a binding
// res := &literal), and /receipt documents 201 and 404 like its value-form
// control /receipt-value. /ptr-item-ptr and /value-item-ptr document the same.
package pointerresult

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the pointerresult module.
type Module struct{}

func (m *Module) Name() string                    { return "pointerresult" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Item is the payload.
type Item struct {
	ID string `json:"id"`
}

// Receipt is the request-less payload.
type Receipt struct {
	Number string `json:"number"`
}

// ItemReq carries the path id.
type ItemReq struct {
	ID string `param:"id"`
}

// RegisterRoutes registers the pointer-wrapper routes and their value-form
// controls.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/ptr/:id", m.ptr)
	server.GET(hr, r, "/ptr-meta/:id", m.ptrMeta)
	server.POST(hr, r, "/ptr-created/:id", m.ptrCreated)
	server.POST(hr, r, "/ptr-literal/:id", m.ptrLiteral)
	server.POST(hr, r, "/ptr-bound/:id", m.ptrBound)
	server.POST(hr, r, "/receipt", m.receipt)
	server.POST(hr, r, "/receipt-value", m.receiptValue)
	server.GET(hr, r, "/ptr-item-ptr/:id", m.ptrItemPtr)
	server.GET(hr, r, "/value-item-ptr/:id", m.valueItemPtr)
}

func (m *Module) ptr(req ItemReq, ctx server.HandlerContext) (*server.Result[Item], server.IAPIError) {
	return &server.Result[Item]{Status: http.StatusOK, Data: Item{ID: req.ID}}, nil
}

func (m *Module) ptrMeta(req ItemReq, ctx server.HandlerContext) (*server.ResultWithMeta[Item], server.IAPIError) {
	return &server.ResultWithMeta[Item]{Status: http.StatusOK, Data: Item{ID: req.ID}, Meta: map[string]any{"total": 1}}, nil
}

func (m *Module) ptrCreated(req ItemReq, ctx server.HandlerContext) (*server.Result[Item], server.IAPIError) {
	if req.ID == "" {
		return nil, server.NewConflictError("dup")
	}
	res := server.Created(Item{ID: req.ID})
	return &res, nil
}

func (m *Module) ptrLiteral(req ItemReq, ctx server.HandlerContext) (*server.Result[Item], server.IAPIError) {
	return &server.Result[Item]{Status: http.StatusCreated, Data: Item{ID: req.ID}}, nil
}

func (m *Module) ptrBound(req ItemReq, ctx server.HandlerContext) (*server.Result[Item], server.IAPIError) {
	res := &server.Result[Item]{Status: http.StatusAccepted, Data: Item{ID: req.ID}}
	return res, nil
}

func (m *Module) receipt(ctx server.HandlerContext) (*server.Result[Receipt], server.IAPIError) {
	if ctx.Echo == nil {
		return nil, server.NewNotFoundError("receipt")
	}
	res := server.Created(Receipt{})
	return &res, nil
}

func (m *Module) receiptValue(ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	if ctx.Echo == nil {
		return server.Result[Receipt]{}, server.NewNotFoundError("receipt")
	}
	return server.Created(Receipt{}), nil
}

func (m *Module) ptrItemPtr(req ItemReq, ctx server.HandlerContext) (*server.Result[*Item], server.IAPIError) {
	return &server.Result[*Item]{Data: &Item{ID: req.ID}}, nil
}

func (m *Module) valueItemPtr(req ItemReq, ctx server.HandlerContext) (server.Result[*Item], server.IAPIError) {
	return server.Result[*Item]{Data: &Item{ID: req.ID}}, nil
}
