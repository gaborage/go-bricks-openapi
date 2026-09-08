// Package catalog exposes list endpoints whose result payload is a slice, so the
// success envelope's data property must be a typed array rather than an object.
package catalog

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the catalog module.
type Module struct{}

func (m *Module) Name() string                    { return "catalog" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Status is a local named scalar: it resolves to no component, so a
// []Status payload documents its items as an untyped object and the route is
// reported as having no resolved type (the analyzer warns).
type Status string

// Item is the catalog resource.
type Item struct {
	ID   int64  `json:"id"`
	Name string `json:"name" doc:"Display name"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/items", m.listItems, server.WithTags("catalog"))
	server.GET(hr, r, "/items/featured", m.featuredItems, server.WithTags("catalog"))
	server.GET(hr, r, "/items/archived", m.archivedItems, server.WithTags("catalog"))
	server.GET(hr, r, "/tags", m.listTags, server.WithTags("catalog"))
	server.GET(hr, r, "/items/export", m.exportItems, server.WithTags("catalog"))
	server.GET(hr, r, "/statuses", m.listStatuses, server.WithTags("catalog"))
}

// listItems returns a slice of a named struct: data is an array of $ref.
func (m *Module) listItems(ctx server.HandlerContext) (server.Result[[]Item], server.IAPIError) {
	return server.OK([]Item{}), nil
}

// featuredItems proves ResultWithMeta behaves identically to Result.
func (m *Module) featuredItems(ctx server.HandlerContext) (server.ResultWithMeta[[]Item], server.IAPIError) {
	return server.OKWithMeta([]Item{}, nil), nil
}

// archivedItems returns a slice of pointers, documented like a slice of values.
func (m *Module) archivedItems(ctx server.HandlerContext) (server.Result[[]*Item], server.IAPIError) {
	return server.OK([]*Item{}), nil
}

// exportItems returns a byte slice: a well-known shape documented as a base64
// string, exactly as a []byte struct field is — not as an array.
func (m *Module) exportItems(ctx server.HandlerContext) (server.Result[[]byte], server.IAPIError) {
	return server.OK([]byte{}), nil
}

// listTags returns a slice of a primitive: items carries the primitive type.
func (m *Module) listTags(ctx server.HandlerContext) (server.Result[[]string], server.IAPIError) {
	return server.OK([]string{}), nil
}

// listStatuses returns a slice of a named scalar, which resolves to no
// component: items stay an untyped object, exactly as the non-slice
// server.Result[Status] payload does.
func (m *Module) listStatuses(ctx server.HandlerContext) (server.Result[[]Status], server.IAPIError) {
	return server.OK([]Status{}), nil
}
