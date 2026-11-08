// Package midsegment pins echo's colon rules in route paths: an unescaped ':'
// starts a path parameter anywhere in a segment, its name running to the next
// '/', and an escaped "\:" is a literal colon whose backslash echo drops.
//
//	/files/file_:name/raw -> /files/file_{name}/raw
//	/m/a:x                -> /m/a{x}
//	/e/a\:x               -> /e/a:x
//	/s/\:lit              -> /s/:lit
//	/x/\:a:b              -> /x/:a{b}
package midsegment

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the midsegment module.
type Module struct{}

func (m *Module) Name() string                    { return "midsegment" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// FileReq binds the mid-segment parameter with a param: field.
type FileReq struct {
	Name string `param:"name"`
}

// EmptyReq binds nothing; any template variable is synthesized.
type EmptyReq struct{}

// Out is the response payload.
type Out struct {
	ID string `json:"id"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/files/file_:name/raw", m.file, server.WithTags("midsegment"))
	server.GET(hr, r, "/m/a:x", m.mid, server.WithTags("midsegment"))
	server.GET(hr, r, `/e/a\:x`, m.esc, server.WithTags("midsegment"))
	server.GET(hr, r, `/s/\:lit`, m.lit, server.WithTags("midsegment"))
	server.GET(hr, r, `/x/\:a:b`, m.mixed, server.WithTags("midsegment"))
}

func (m *Module) file(req FileReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) mid(req EmptyReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) esc(req EmptyReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) lit(req EmptyReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}

func (m *Module) mixed(req EmptyReq, ctx server.HandlerContext) (server.Result[Out], server.IAPIError) {
	return server.NewResult(http.StatusOK, Out{}), nil
}
