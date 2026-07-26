// Package vts integrates with Visa Token Services using JOSE-protected payloads.
package vts

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the VTS module.
type Module struct{}

func (m *Module) Name() string                    { return "vts" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// CreateTokenRequest is decrypted-then-verified on inbound. The sentinel field
// carries the jose tag that marks the route as JOSE-protected; the analyzer
// detects it and the generator emits Content-Type application/jose for the body.
type CreateTokenRequest struct {
	_   struct{} `jose:"decrypt=our-signing,verify=visa-vts-verify"`
	PAN string   `json:"pan" validate:"required"`
}

// CreateTokenResponse is signed-then-encrypted on outbound.
type CreateTokenResponse struct {
	_     struct{} `jose:"sign=our-signing,encrypt=visa-vts-encrypt"`
	Token string   `json:"token"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/v1/tokens", m.createToken, server.WithTags("vts"))
}

// createToken's response is wrapped in server.Result[T]. The analyzer unwraps
// that generic, so route.Response resolves to CreateTokenResponse and the
// generated 200 shows application/jose per the response struct's jose tag —
// not the standard application/json SuccessResponse envelope. The request
// side is detected the same way, so the requestBody is application/jose and
// the 4xx references JOSEErrorEnvelope. This fixture exercises the full
// JOSE-response round-trip end-to-end.
func (m *Module) createToken(req CreateTokenRequest, ctx server.HandlerContext) (server.Result[CreateTokenResponse], server.IAPIError) {
	return server.NewResult(http.StatusOK, CreateTokenResponse{}), nil
}
