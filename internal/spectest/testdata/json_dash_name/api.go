// Package jsondash locks how a json tag whose name is "-" is read (issue 155).
//
// encoding/json skips a field only for a bare json:"-". A tag with a comma
// after the dash, json:"-," or json:"-,omitempty", names the field "-" and
// writes it under that key. DashComma and DashOmit are separate structs on
// purpose: in one struct the two fields would collide on the key "-" and
// encoding/json would drop both. EmbDash nests Base under the key "-", so Base
// is emitted. Skipped is the bare json:"-" control, and ItemReq keeps its
// json:"-" field as a path parameter.
package jsondash

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the jsondash module.
type Module struct{}

func (m *Module) Name() string                    { return "jsondash" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Base is nested by EmbDash under the key "-".
type Base struct {
	X int `json:"x"`
}

// DashComma writes Dash under the key "-".
type DashComma struct {
	Dash int `json:"-,"`
	Keep int `json:"keep"`
}

// DashOmit writes Dash2 under the key "-" too.
type DashOmit struct {
	Dash2 int `json:"-,omitempty"`
	Keep  int `json:"keep"`
}

// Skipped drops Gone: a bare json:"-".
type Skipped struct {
	Gone int `json:"-"`
	Keep int `json:"keep"`
}

// EmbDash nests Base as one property "-" rather than promoting X.
type EmbDash struct {
	Base `json:"-,"`
	Keep int `json:"keep"`
}

// Bundle is the response payload.
type Bundle struct {
	Comma   DashComma `json:"comma"`
	Omit    DashOmit  `json:"omit"`
	Skipped Skipped   `json:"skipped"`
	Emb     EmbDash   `json:"emb"`
}

// ItemReq binds ID from the path; json:"-" only keeps it out of the body.
type ItemReq struct {
	ID int64 `param:"id" json:"-"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/items/:id", m.get, server.WithTags("jsondash"))
}

func (m *Module) get(req ItemReq, ctx server.HandlerContext) (server.Result[Bundle], server.IAPIError) {
	return server.NewResult(http.StatusOK, Bundle{}), nil
}
