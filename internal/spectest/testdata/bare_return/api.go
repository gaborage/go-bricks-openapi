// Package barereturn demonstrates bare (non-wrapper) handler returns: go-bricks
// sends a bare T under the same data key as server.Result[T], so each payload is
// documented exactly as its server.Result[T] counterpart would be.
package barereturn

import (
	"encoding/json"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
)

type Module struct{}

func (m *Module) Name() string                    { return "barereturn" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Address is a project struct: as a bare return it is a $ref to its component.
type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/stats/total", m.total, server.WithTags("bare"))
	server.GET(hr, r, "/stats/optional", m.optional, server.WithTags("bare"))
	server.GET(hr, r, "/stats/name", m.name, server.WithTags("bare"))
	server.GET(hr, r, "/stats/enabled", m.enabled, server.WithTags("bare"))
	server.GET(hr, r, "/stats/average", m.average, server.WithTags("bare"))
	server.GET(hr, r, "/stats/anything", m.anything, server.WithTags("bare"))
	server.GET(hr, r, "/stats/legacy", m.legacy, server.WithTags("bare"))
	server.GET(hr, r, "/stats/created-at", m.createdAt, server.WithTags("bare"))
	server.GET(hr, r, "/stats/id", m.id, server.WithTags("bare"))
	server.GET(hr, r, "/stats/raw", m.raw, server.WithTags("bare"))
	server.GET(hr, r, "/stats/tags", m.tags, server.WithTags("bare"))
	server.GET(hr, r, "/addresses", m.addresses, server.WithTags("bare"))
	server.GET(hr, r, "/addresses/primary", m.primary, server.WithTags("bare"))
	server.GET(hr, r, "/addresses/fallback", m.fallback, server.WithTags("bare"))
}

// Builtins are typed inline and name no component.

func (m *Module) total(ctx server.HandlerContext) (int64, server.IAPIError) { // data: {integer, int64}
	return 0, nil
}

func (m *Module) optional(ctx server.HandlerContext) (*int64, server.IAPIError) { // data: {integer, int64} (pointer shed)
	return nil, nil
}

func (m *Module) name(ctx server.HandlerContext) (string, server.IAPIError) { // data: {string}
	return "", nil
}

func (m *Module) enabled(ctx server.HandlerContext) (bool, server.IAPIError) { // data: {boolean}
	return false, nil
}

func (m *Module) average(ctx server.HandlerContext) (float64, server.IAPIError) { // data: {number, double}
	return 0, nil
}

func (m *Module) anything(ctx server.HandlerContext) (any, server.IAPIError) { // data: {}
	return nil, nil
}

func (m *Module) legacy(ctx server.HandlerContext) (interface{}, server.IAPIError) { // data: {}
	return nil, nil
}

// Well-known types are typed inline, never $ref'd.

func (m *Module) createdAt(ctx server.HandlerContext) (time.Time, server.IAPIError) { // data: {string, date-time}
	return time.Time{}, nil
}

func (m *Module) id(ctx server.HandlerContext) (uuid.UUID, server.IAPIError) { // data: {string, uuid}
	return uuid.UUID{}, nil
}

func (m *Module) raw(ctx server.HandlerContext) (json.RawMessage, server.IAPIError) { // data: {}
	return nil, nil
}

// Slices are arrays whose items follow the same rules.

func (m *Module) tags(ctx server.HandlerContext) ([]string, server.IAPIError) { // data: {array, items string}
	return nil, nil
}

func (m *Module) addresses(ctx server.HandlerContext) ([]Address, server.IAPIError) { // data: {array, items $ref Address}
	return nil, nil
}

// Project structs are $ref'd, by value or by pointer, exactly as before.

func (m *Module) primary(ctx server.HandlerContext) (Address, server.IAPIError) { // data: $ref Address
	return Address{}, nil
}

func (m *Module) fallback(ctx server.HandlerContext) (*Address, server.IAPIError) { // data: $ref Address
	return nil, nil
}
