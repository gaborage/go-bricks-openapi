// Package orders exercises the //openapi:errors comment Directive: the declared
// statuses join the unconditional 400/500 baseline on a JSON route, union with
// the JOSE 401/415 pre-trust catalog (deduplicating silently) on a sealed
// route, and a malformed token is diagnosed while its well-formed siblings
// still apply. A directive-free route pins the untouched baseline.
package orders

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the orders module.
type Module struct{}

func (m *Module) Name() string                    { return "orders" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// CreateOrderRequest is the plaintext order payload.
type CreateOrderRequest struct {
	SKU string `json:"sku" validate:"required"`
}

// Order is the plaintext order representation.
type Order struct {
	ID  string `json:"id"`
	SKU string `json:"sku"`
}

// SealedOrderRequest is decrypted-then-verified on inbound.
type SealedOrderRequest struct {
	_   struct{} `jose:"decrypt=our-signing,verify=partner-verify"`
	SKU string   `json:"sku" validate:"required"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	// A JSON route declaring two extra error branches.
	//openapi:errors 404, 409
	server.POST(hr, r, "/v1/orders", m.create, server.WithTags("orders"))

	// A JOSE route: 403 is new, 401 dedupes against the pre-trust catalog.
	//openapi:errors 403, 401
	server.POST(hr, r, "/v1/orders/sealed", m.createSealed, server.WithTags("orders"))

	// One malformed token is diagnosed; the valid 404 still applies.
	//openapi:errors 404,teapot
	server.GET(hr, r, "/v1/orders/:id", m.get, server.WithTags("orders"))

	// No directive: the 400/500 baseline only.
	server.GET(hr, r, "/v1/orders", m.list, server.WithTags("orders"))
}

func (m *Module) create(req CreateOrderRequest, ctx server.HandlerContext) (server.Result[Order], server.IAPIError) {
	return server.Created(Order{}), nil
}

func (m *Module) createSealed(req SealedOrderRequest, ctx server.HandlerContext) (server.Result[Order], server.IAPIError) {
	return server.Created(Order{}), nil
}

func (m *Module) get(ctx server.HandlerContext) (server.Result[Order], server.IAPIError) {
	return server.NewResult(http.StatusOK, Order{}), nil
}

func (m *Module) list(ctx server.HandlerContext) (server.Result[Order], server.IAPIError) {
	return server.NewResult(http.StatusOK, Order{}), nil
}
