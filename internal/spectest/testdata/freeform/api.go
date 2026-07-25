// Package api exposes a free-form metadata endpoint.
package api

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the API module.
type Module struct{}

func (m *Module) Name() string                    { return "api" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// UpdateMetadataReq is the request body for updating free-form metadata. Each
// field legitimately holds any JSON value (scalar, array, object, or null),
// so none of them may be constrained to type: object.
type UpdateMetadataReq struct {
	Metadata any            `json:"metadata"`
	Extra    interface{}    `json:"extra"`
	Tags     map[string]any `json:"tags"`
}

// MetadataResp is the metadata resource, echoing the same free-form shapes
// back in the response.
type MetadataResp struct {
	ID       int64          `json:"id"`
	Metadata any            `json:"metadata"`
	Extra    interface{}    `json:"extra"`
	Tags     map[string]any `json:"tags"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/metadata", m.updateMetadata, server.WithTags("metadata"), server.WithSummary("Update metadata"))
}

func (m *Module) updateMetadata(req UpdateMetadataReq, ctx server.HandlerContext) (server.Result[MetadataResp], server.IAPIError) {
	return server.Created(MetadataResp{}), nil
}
