// Package documents stores opaque JSON documents. A json.RawMessage holds any
// JSON value (string, number, array, boolean, null or object), so every
// position below documents it as the untyped schema {}: never `type: object`,
// and never the base64 string or array its underlying []byte would suggest.
package documents

import (
	"encoding/json"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the documents module.
type Module struct{}

func (m *Module) Name() string                    { return "documents" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Document is a stored document and its revision history.
type Document struct {
	ID       int64                      `json:"id"`
	Body     json.RawMessage            `json:"body" example:"{\"a\":1}"`      // -> {}, example kept as a plain string
	Previous *json.RawMessage           `json:"previous"`                      // -> {}, no nullable (same as *any)
	Attrs    map[string]json.RawMessage `json:"attrs"`                         // -> object, additionalProperties {}
	Patches  []json.RawMessage          `json:"patches" validate:"max=5,dive"` // -> array, items {}
}

// CreateDocumentReq is the request body for storing a document. The length
// rule has no keyword on an untyped schema, so it adds nothing.
type CreateDocumentReq struct {
	Body json.RawMessage `json:"body" validate:"required,min=2"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/documents", m.listBodies, server.WithTags("documents"))
	server.POST(hr, r, "/documents", m.createDocument, server.WithTags("documents"))
}

// listBodies returns a slice of raw documents: data is an array of {} items,
// not an array of a dangling RawMessage $ref.
func (m *Module) listBodies(ctx server.HandlerContext) (server.Result[[]json.RawMessage], server.IAPIError) {
	return server.OK([]json.RawMessage{}), nil
}

func (m *Module) createDocument(req CreateDocumentReq, ctx server.HandlerContext) (server.Result[Document], server.IAPIError) {
	return server.Created(Document{}), nil
}
