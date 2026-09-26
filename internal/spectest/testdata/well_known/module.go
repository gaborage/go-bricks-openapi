// Package events demonstrates well-known type formats, map additionalProperties,
// and uint64 minimum fidelity in the generated schema.
package events

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module wires the events route to a Handler.
type Module struct {
	h *Handler
}

func (m *Module) Name() string                    { return "events" }
func (m *Module) Init(deps *app.ModuleDeps) error { m.h = &Handler{}; return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/events/:id", m.h.getEvent, server.WithTags("events"))
	server.GET(hr, r, "/events/raw", m.h.rawPayload, server.WithTags("events"))
	server.GET(hr, r, "/events/raw-pointer", m.h.rawPointerPayload, server.WithTags("events"))
	server.GET(hr, r, "/events/created-at", m.h.createdAt, server.WithTags("events"))
	server.GET(hr, r, "/events/id", m.h.eventID, server.WithTags("events"))
	server.GET(hr, r, "/events/ttl", m.h.ttl, server.WithTags("events"))
	server.GET(hr, r, "/events/name", m.h.name, server.WithTags("events"))
	server.GET(hr, r, "/events/total", m.h.total, server.WithTags("events"))
	server.GET(hr, r, "/events/count", m.h.count, server.WithTags("events"))
	server.GET(hr, r, "/events/enabled", m.h.enabled, server.WithTags("events"))
	server.GET(hr, r, "/events/anything", m.h.anything, server.WithTags("events"))
	server.GET(hr, r, "/events/updated-at", m.h.updatedAt, server.WithTags("events"))
	server.GET(hr, r, "/events/raw-body", m.h.rawBody, server.WithTags("events"), server.WithRawResponse())
}
