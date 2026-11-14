// Package buildtagstructs demonstrates named types whose build-tagged
// declarations (w_linux.go, w_other.go) each reach a struct, but not the same
// one. One spec cannot describe both builds, so every use of such a type is
// documented as an untyped object, with a warning, and no component is
// registered for it. Same reaches dw.Item in both builds and keeps its
// component.
package buildtagstructs

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "buildtagstructs" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// OverW is one declaration over the two-copy W, so it falls back too.
type OverW W

// Resp holds a field of every such type, and of Same.
type Resp struct {
	W     W     `json:"w"`
	WA    WA    `json:"wa"`
	WI    WI    `json:"wi"`
	Two   Two   `json:"two"`
	Loc   Loc   `json:"loc"`
	OverW OverW `json:"overW"`
	WP    []W   `json:"wp"`
	Same  Same  `json:"same"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/resp", m.resp, server.WithTags("buildtagstructs"))
	server.GET(hr, r, "/w", m.getW, server.WithTags("buildtagstructs"))
	server.POST(hr, r, "/w", m.postW, server.WithTags("buildtagstructs"))
}

func (m *Module) resp(ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) {
	return server.Result[Resp]{}, nil
}

func (m *Module) getW(ctx server.HandlerContext) (server.Result[W], server.IAPIError) { // data: untyped
	return server.Result[W]{}, nil
}

func (m *Module) postW(req W, ctx server.HandlerContext) (server.Result[Same], server.IAPIError) { // no requestBody
	return server.Result[Same]{}, nil
}
