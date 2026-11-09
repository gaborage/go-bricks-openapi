// Package statedschemaparams pins that a Stated schema (#117) leaves
// parameters alone: they are bound by kind, so a directive-carrying struct
// Marshaler type keeps its component and a non-struct one its kind schema.
package statedschemaparams

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "statedschemaparams" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Money is written as a decimal string by its own MarshalJSON.
//
//openapi:schema string decimal
type Money struct {
	Units int64 `json:"units"`
}

// MarshalJSON writes "12.50".
func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

// Code is written as a string by its own MarshalJSON.
//
//openapi:schema string
type Code int

// MarshalJSON writes the code's name.
func (c Code) MarshalJSON() ([]byte, error) { return nil, nil }

// Query binds both types as query parameters.
type Query struct {
	M Money `query:"m"`
	C Code  `query:"c"`
}

// RegisterRoutes registers the one route.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/q", m.query)
}

func (m *Module) query(req Query, ctx server.HandlerContext) (server.Result[Code], server.IAPIError) {
	var v server.Result[Code]
	return v, nil
}
