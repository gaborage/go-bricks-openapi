// Package jsonexcluded locks the invariant that a request type with non-empty
// Fields but ZERO serializable properties must still get a component.
//
// Every field of DashReq is json:"-", so typeInfoToSchema returns a NON-nil
// schema whose Properties map is empty. The fields carry no param tags, so
// extractParameters (which keys on param tags, not json tags) puts them in
// bodyFields — len(bodyFields) > 0 — and buildRequestBody emits a REAL
// requestBody $ref to DashReq. DashReq is a non-JOSE request, so it is NOT in
// referencedSchemaNames. The component therefore exists only because a non-nil
// schema is emitted unconditionally.
//
// If anyone ever "optimizes" the `schema == nil` check in generateSchemasFromTypes
// into a zero-properties check, DashReq is dropped, the requestBody $ref dangles,
// and this fixture fails the harness's in-process OpenAPI 3.0 validation.
package jsonexcluded

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the jsonexcluded module.
type Module struct{}

func (m *Module) Name() string                    { return "jsonexcluded" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// DashReq has fields, but every one is excluded from JSON. They are still body
// fields (extractParameters keys on param tags, not json tags), so a requestBody
// $ref IS emitted — the component must therefore exist.
type DashReq struct {
	Secret string `json:"-"`
	Other  int    `json:"-"`
}

// Dashboard is the response payload.
type Dashboard struct {
	Widgets int `json:"widgets"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/dashboards", m.build, server.WithTags("jsonexcluded"))
}

func (m *Module) build(req DashReq, ctx server.HandlerContext) (server.Result[Dashboard], server.IAPIError) {
	return server.NewResult(http.StatusOK, Dashboard{}), nil
}
