// Package constroutepaths registers routes whose path arguments are built from
// string constants rather than plain literals: a package-level constant folded
// with "+", a function-local constant, a constant declared in a nested block, a
// local constant that shadows a package-level one of the same name, and a group
// whose prefix is a function-local constant. Each must resolve to the folded
// path. The last registration deliberately uses a `var` — only `const` resolves,
// so that route stays an Unresolved route and is absent from the golden
// document.
package constroutepaths

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

const (
	basePath = "/v1"
	shadowed = "/from-package"
)

// Module is the constroutepaths module.
type Module struct{}

func (m *Module) Name() string                    { return "constroutepaths" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// PingReq is a request type with no serializable properties.
type PingReq struct{}

// PingResp is the shared response type for every route below.
type PingResp struct {
	OK bool `json:"ok"`
}

// RegisterRoutes registers one route per constant-resolution case.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	const localPath = "/local"
	const shadowed = "/from-function"

	// Package-level constant folded with a literal.
	server.GET(hr, r, basePath+"/things", m.h, server.WithTags("api"))
	// Function-local constant.
	server.GET(hr, r, localPath, m.h, server.WithTags("api"))
	// A local constant shadowing the package-level one of the same name.
	server.GET(hr, r, shadowed, m.h, server.WithTags("api"))

	if true {
		const nestedPath = "/nested"
		// Constant declared in a nested block, folded with an outer constant.
		server.GET(hr, r, basePath+nestedPath, m.h, server.WithTags("api"))
	}

	// A group whose prefix argument is a function-local constant: the prefix
	// must survive onto every route registered on the group.
	const groupPrefix = "/grouped"
	api := r.Group(groupPrefix)
	server.GET(hr, api, "/widgets", m.h, server.WithTags("api"))

	// Deliberately unresolved: a `var` is not a constant, so this route is
	// dropped with an unresolved-route warning and never reaches the document.
	var dynamicPath = "/dynamic"
	server.GET(hr, r, dynamicPath, m.h, server.WithTags("api"))
}

func (m *Module) h(req PingReq, ctx server.HandlerContext) (server.Result[PingResp], server.IAPIError) {
	return server.NewResult(http.StatusOK, PingResp{}), nil
}
