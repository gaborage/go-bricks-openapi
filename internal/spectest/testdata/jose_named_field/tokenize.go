// Package tokenize exercises a JOSE-protected route whose jose tag lives on a
// named field rather than the blank `_` sentinel field. jose.ScanType checks no
// field name — any jose-tagged field opts the type in — so the analyzer must
// detect this form too, not only the documented sentinel convention.
package tokenize

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the tokenize module.
type Module struct{}

func (m *Module) Name() string                    { return "tokenize" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// CreateTokenRequest is decrypted-then-verified on inbound. Unlike the sentinel
// convention, the jose tag here sits directly on the named PAN field alongside
// its json tag — the analyzer must still detect JOSE and emit
// Content-Type: application/jose for the request body.
type CreateTokenRequest struct {
	PAN string `json:"pan" jose:"decrypt=our-signing,verify=partner-verify" validate:"required"`
}

// CreateTokenResponse carries no jose tag — a plain plaintext response.
type CreateTokenResponse struct {
	Token string `json:"token"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/v1/tokens", m.createToken, server.WithTags("tokenize"))
}

func (m *Module) createToken(req CreateTokenRequest, ctx server.HandlerContext) (server.Result[CreateTokenResponse], server.IAPIError) {
	return server.NewResult(http.StatusOK, CreateTokenResponse{}), nil
}
