// Package datetimelayouts is a regression fixture for issue #103: the
// validator applies validate:"datetime=<layout>" as time.Parse(layout, v), so
// only a layout that accepts what the OpenAPI format describes may emit it:
// exactly time.DateOnly gets format: date (see the date_format_example
// fixture), exactly time.RFC3339 or time.RFC3339Nano gets format: date-time,
// and every other layout gets no format. With no format, an example the
// layout accepts is kept verbatim instead of failing a date / date-time check.
// CreateSlotReq covers non-RFC 3339 date layouts (YearMonth, SlashDate),
// clock layouts (TimeOnly, Kitchen, SpaceDateTime), strict subsets of
// RFC 3339 (LiteralZ, FixedMillis), a bare datetime (Bare), dive items
// (Times) and an RFC3339Nano control that keeps date-time (Nano).
// ListSlotsReq's At covers a query parameter and its schema.
package datetimelayouts

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the datetimelayouts module.
type Module struct{}

func (m *Module) Name() string                    { return "datetimelayouts" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// ListSlotsReq carries a time-only query parameter.
type ListSlotsReq struct {
	At string `query:"at" validate:"datetime=15:04" example:"10:30"`
}

// CreateSlotReq exercises every datetime layout class.
type CreateSlotReq struct {
	YearMonth     string   `json:"yearMonth" validate:"datetime=2006-01" example:"2024-01"`
	SlashDate     string   `json:"slashDate" validate:"datetime=01/02/2006" example:"01/02/2024"`
	TimeOnly      string   `json:"timeOnly" validate:"datetime=15:04" example:"10:30"`
	Kitchen       string   `json:"kitchen" validate:"datetime=3:04PM" example:"3:04PM"`
	SpaceDateTime string   `json:"spaceDateTime" validate:"datetime=2006-01-02 15:04:05" example:"2024-01-02 10:30:00"`
	LiteralZ      string   `json:"literalZ" validate:"datetime=2006-01-02T15:04:05Z" example:"2024-01-02T10:30:00Z"`
	FixedMillis   string   `json:"fixedMillis" validate:"datetime=2006-01-02T15:04:05.000Z07:00" example:"2024-01-02T10:30:00.000+02:00"`
	Bare          string   `json:"bare" validate:"datetime"`
	Times         []string `json:"times" validate:"dive,datetime=15:04"`
	Nano          string   `json:"nano" validate:"datetime=2006-01-02T15:04:05.999999999Z07:00" example:"2024-01-02T10:30:00.123456789Z"`
}

// Slot is returned by both routes.
type Slot struct {
	ID string `json:"id"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/v1/slots", m.listSlots, server.WithTags("slots"))
	server.POST(hr, r, "/v1/slots", m.createSlot, server.WithTags("slots"))
}

func (m *Module) listSlots(req ListSlotsReq, ctx server.HandlerContext) (server.Result[[]Slot], server.IAPIError) {
	return server.NewResult(http.StatusOK, []Slot{}), nil
}

func (m *Module) createSlot(req CreateSlotReq, ctx server.HandlerContext) (server.Result[Slot], server.IAPIError) {
	return server.NewResult(http.StatusOK, Slot{}), nil
}
