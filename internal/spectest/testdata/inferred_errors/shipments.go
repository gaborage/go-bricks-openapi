// Package shipments exercises call-site error inference: the statuses the
// go-bricks error constructors a handler body calls produce become error
// responses on that operation. One route unions inferred statuses with an
// //openapi:errors Directive, one resolves NewBaseAPIError's status argument, a
// JOSE route infers a 403 alongside its pre-trust catalog, and one route proves
// the one-level limit — its constructor sits in a helper the handler calls, so
// nothing is inferred.
package shipments

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the shipments module.
type Module struct{}

func (m *Module) Name() string                    { return "shipments" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// CreateShipmentRequest is the plaintext shipment payload.
type CreateShipmentRequest struct {
	Reference string `json:"reference" validate:"required"`
}

// Shipment is the plaintext shipment representation.
type Shipment struct {
	ID        string `json:"id"`
	Reference string `json:"reference"`
}

// SealedShipmentRequest is decrypted-then-verified on inbound.
type SealedShipmentRequest struct {
	_         struct{} `jose:"decrypt=our-signing,verify=partner-verify"`
	Reference string   `json:"reference" validate:"required"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	// Inferred 404 and 409; the Directive adds 422 and redeclares 409.
	//openapi:errors 422, 409
	server.POST(hr, r, "/v1/shipments", m.create, server.WithTags("shipments"))

	// NewBaseAPIError's third argument resolves to 410.
	server.GET(hr, r, "/v1/shipments/:id", m.get, server.WithTags("shipments"))

	// A JOSE route inferring 403 on top of its pre-trust catalog.
	server.POST(hr, r, "/v1/shipments/sealed", m.createSealed, server.WithTags("shipments"))

	// The constructor lives in a helper: nothing is inferred (documented limit).
	server.GET(hr, r, "/v1/shipments", m.list, server.WithTags("shipments"))
}

func (m *Module) create(req CreateShipmentRequest, ctx server.HandlerContext) (server.Result[Shipment], server.IAPIError) {
	if req.Reference == "" {
		return server.Result[Shipment]{}, server.NewNotFoundError("carrier not found")
	}
	if req.Reference == "dup" {
		return server.Result[Shipment]{}, server.NewConflictError("shipment already exists")
	}
	return server.Created(Shipment{}), nil
}

func (m *Module) get(ctx server.HandlerContext) (server.Result[Shipment], server.IAPIError) {
	if ctx.Path() == "" {
		return server.Result[Shipment]{}, server.NewBaseAPIError("SHIPMENT_ARCHIVED", "shipment archived", http.StatusGone)
	}
	return server.NewResult(http.StatusOK, Shipment{}), nil
}

func (m *Module) createSealed(req SealedShipmentRequest, ctx server.HandlerContext) (server.Result[Shipment], server.IAPIError) {
	if req.Reference == "" {
		return server.Result[Shipment]{}, server.NewForbiddenError("partner not entitled")
	}
	return server.Created(Shipment{}), nil
}

func (m *Module) list(ctx server.HandlerContext) (server.Result[Shipment], server.IAPIError) {
	if err := requireQuota(); err != nil {
		return server.Result[Shipment]{}, err
	}
	return server.NewResult(http.StatusOK, Shipment{}), nil
}

// requireQuota is a same-package helper; the analyzer does not follow into it,
// so its 429 never reaches the document.
func requireQuota() server.IAPIError {
	return server.NewTooManyRequestsError("quota exceeded")
}
