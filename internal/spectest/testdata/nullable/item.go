// Package nullable exercises pointer fields marking `nullable: true` in the
// generated schema (plan 018): a Go pointer without `omitempty` serializes a
// nil to JSON null, which OpenAPI 3.0 only permits when the schema declares
// `nullable: true`.
package nullable

import (
	"net/http"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the nullable-fields demo module.
type Module struct{}

func (m *Module) Name() string                    { return "nullable" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Sub is a small nested struct reached only through a pointer field
// (Item.Detail), so it still registers as its own component.
type Sub struct {
	Label string `json:"label"`
}

// GetItemReq pairs a non-pointer path parameter with a pointer body field, to
// pin RULING 1 end-to-end: extractParameters and typeInfoToSchema share
// fieldInfoToProperty, but only the body field may become nullable.
type GetItemReq struct {
	ID    int64   `param:"id" validate:"required"`
	Notes *string `json:"notes"` // pointer body field -> nullable
}

// Item is the response payload: a pointer scalar, a pointer well-known type,
// and a pointer-to-struct, each of which must carry `nullable: true`.
type Item struct {
	Balance   *int64     `json:"balance"`   // pointer scalar -> nullable
	DeletedAt *time.Time `json:"deletedAt"` // pointer well-known type -> nullable
	Detail    *Sub       `json:"detail"`    // pointer to struct -> allOf + nullable
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/items/:id", m.getItem, server.WithTags("items"))
}

func (m *Module) getItem(req GetItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.NewResult(http.StatusOK, Item{}), nil
}
