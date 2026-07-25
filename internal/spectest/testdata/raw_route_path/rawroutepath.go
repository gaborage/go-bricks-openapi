// Package rawroutepath registers two routes whose only difference is the
// source form of their path literal — one raw (backtick), one interpreted
// (double-quoted). Both forms are legal Go and identical at runtime, so the
// analyzer must resolve both to a path key rather than silently dropping the
// raw-literal route (extractPathFromArg previously trimmed only `"`, a no-op
// on a raw literal, which made the route unresolvable and caused the caller
// to drop it from the emitted spec without any warning).
package rawroutepath

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the rawroutepath module.
type Module struct{}

func (m *Module) Name() string                    { return "rawroutepath" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// PingReq is a request type with no serializable properties.
type PingReq struct{}

// PingResp is the shared response type for both routes.
type PingResp struct {
	OK bool `json:"ok"`
}

// RegisterRoutes registers two routes that differ only in path literal form.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, `/v1/raw`, m.h, server.WithTags("api"))
	server.POST(hr, r, "/v1/normal", m.h, server.WithTags("api"))
}

func (m *Module) h(req PingReq, ctx server.HandlerContext) (server.Result[PingResp], server.IAPIError) {
	return server.NewResult(http.StatusOK, PingResp{}), nil
}
