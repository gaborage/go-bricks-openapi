// Package anonymousrequest exercises anonymous struct requests (issue 171).
// Each is documented exactly as a named struct of the same fields: its path,
// query and header fields are typed parameters, its other fields an inline
// requestBody object (a params-only one has none), and a jose tag on any
// field makes the body an application/jose token whose plaintext has no
// component. A struct reachable only through one still registers.
package anonymousrequest

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the anonymous-request demo module.
type Module struct{}

func (m *Module) Name() string                    { return "anonymousrequest" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Out is every plain route's response.
type Out struct {
	OK bool `json:"ok"`
}

// SealedResp is a JOSE response.
type SealedResp struct {
	_  struct{} `jose:"sign=ours,encrypt=theirs"`
	OK bool     `json:"ok"`
}

// Ack has no serializable field and is reachable only through an anonymous
// request's body field.
type Ack struct{}

// RegisterRoutes registers one route per anonymous request form.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/anon/:id", m.anon, server.WithTags("anonymous"))
	server.POST(hr, r, "/anonsealed", m.anonSealed, server.WithTags("anonymous"))
	server.GET(hr, r, "/anonparams/:id", m.anonParams, server.WithTags("anonymous"))
	server.POST(hr, r, "/anonack", m.anonAck, server.WithTags("anonymous"))
}

func (m *Module) anon(req struct {
	ID   int64  `param:"id"`
	Q    int    `query:"q"`
	H    string `header:"X-Trace"`
	Body string `json:"body" validate:"required"`
}, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.Result[Out]{}, nil
}

func (m *Module) anonSealed(req struct {
	_   struct{} `jose:"decrypt=ours,verify=theirs"`
	PAN string   `json:"pan"`
}, ctx server.HandlerContext) (SealedResp, server.IAPIError) {
	return SealedResp{}, nil
}

func (m *Module) anonParams(req struct {
	ID int64 `param:"id"`
	Q  int   `query:"q"`
}, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.Result[Out]{}, nil
}

func (m *Module) anonAck(req *struct {
	Ack Ack `json:"ack"`
}, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.Result[Out]{}, nil
}
