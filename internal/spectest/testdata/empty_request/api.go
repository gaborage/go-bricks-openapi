// Package emptyreq pins the invariant that a non-JOSE request $ref can never
// dangle, so referencedSchemaNames does not need to scan request types.
//
// A property-less request type produces no body fields, so the operation gets no
// requestBody at all (the guard in buildOperation is `len(bodyFields) > 0 ||
// route.Request.JOSE`) and therefore no request $ref to resolve. The type is
// also unreferenced, so no orphan component is emitted for it. The golden below
// locks both halves: no `requestBody` key, and no EmptyReq component. If the
// requestBody guard ever loosens, this fixture's golden changes and a dangling
// request $ref cannot slip in silently.
package emptyreq

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the emptyreq module.
type Module struct{}

func (m *Module) Name() string                    { return "emptyreq" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// EmptyReq is a request type with no serializable properties.
type EmptyReq struct{}

// Receipt is the response payload.
type Receipt struct {
	ID string `json:"id"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/pings", m.ping, server.WithTags("emptyreq"))
}

func (m *Module) ping(req EmptyReq, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}
