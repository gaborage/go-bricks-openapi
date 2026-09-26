// Package dateformat is a regression fixture for issue #99: kin-openapi
// validates an `example:` against `format: date` and `format: date-time`, so an
// example that is not RFC 3339 would invalidate the whole document. Only an
// example that kin's pattern and Go's time parser both accept is kept.
// CreateEventReq covers, field by field, a valid RFC 3339 example on a
// time.Time (StartsAt, kept), free text on a time.Time (EndsAt, dropped), an
// impossible day on a *time.Time (CancelledAt, dropped), a
// validate:"datetime=2006-01-02" string that yields format: date with a valid
// full-date (Day, kept) and a slash-separated date (BadDay, dropped), and a
// validate:"date" string with a year-month example (Month, dropped).
// ListEventsReq's Since drops a non-RFC 3339 example from both the query
// parameter and its schema, which kin-openapi validates independently.
package dateformat

import (
	"net/http"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the dateformat module.
type Module struct{}

func (m *Module) Name() string                    { return "dateformat" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// ListEventsReq carries a date-time query parameter whose example is not RFC 3339.
type ListEventsReq struct {
	Since time.Time `query:"since" example:"yesterday"`
}

// CreateEventReq exercises every date / date-time example outcome.
type CreateEventReq struct {
	StartsAt    time.Time  `json:"startsAt" example:"2024-01-02T03:04:05Z"`
	EndsAt      time.Time  `json:"endsAt" example:"now"`
	CancelledAt *time.Time `json:"cancelledAt" example:"2024-02-31T00:00:00Z"`
	Day         string     `json:"day" validate:"datetime=2006-01-02" example:"2024-01-02"`
	BadDay      string     `json:"badDay" validate:"datetime=2006-01-02" example:"01/02/2024"`
	Month       string     `json:"month" validate:"date" example:"2024-01"`
}

// Event is returned by both routes.
type Event struct {
	ID       string    `json:"id"`
	StartsAt time.Time `json:"startsAt"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/v1/events", m.listEvents, server.WithTags("events"))
	server.POST(hr, r, "/v1/events", m.createEvent, server.WithTags("events"))
}

func (m *Module) listEvents(req ListEventsReq, ctx server.HandlerContext) (server.Result[[]Event], server.IAPIError) {
	return server.NewResult(http.StatusOK, []Event{}), nil
}

func (m *Module) createEvent(req CreateEventReq, ctx server.HandlerContext) (server.Result[Event], server.IAPIError) {
	return server.NewResult(http.StatusOK, Event{}), nil
}
