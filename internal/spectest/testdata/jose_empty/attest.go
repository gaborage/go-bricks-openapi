// Package attest exercises JOSE routes whose plaintext types carry the jose
// sentinel field and nothing else — no serializable properties.
//
// joseDescription names the plaintext component in prose ("see
// #/components/schemas/<Name>") while the wire schema is a string token, not a
// $ref. A property-less plaintext type would otherwise get no component at all,
// leaving that cross-reference pointing at a schema that does not exist. The
// component must be emitted (as an empty object) so the reference resolves.
package attest

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the attest module.
type Module struct{}

func (m *Module) Name() string                    { return "attest" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// AttestRequest is JOSE-protected on inbound and carries no plaintext fields —
// the sentinel tag is its only field, so it has no serializable properties.
type AttestRequest struct {
	_ struct{} `jose:"decrypt=our-signing,verify=partner-verify"`
}

// AttestAck is JOSE-protected on outbound and is an empty acknowledgement — the
// sentinel tag is its only field.
type AttestAck struct {
	_ struct{} `jose:"sign=our-signing,encrypt=partner-encrypt"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/v1/attestations", m.attest, server.WithTags("attest"))
}

func (m *Module) attest(req AttestRequest, ctx server.HandlerContext) (server.Result[AttestAck], server.IAPIError) {
	return server.NewResult(http.StatusOK, AttestAck{}), nil
}
