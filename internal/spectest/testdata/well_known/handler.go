package events

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
)

// Handler holds the events HTTP handlers.
type Handler struct{}

// Address is a nested struct used as a map value (map[string]Address).
type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

// Event exercises the well-known type formats, maps, and uint64's mapping.
// Count's int64 format understates uint64: values of 2^63 and above exceed
// it (a documented Known limitation), while minimum: 0 keeps it non-negative.
type Event struct {
	ID        uuid.UUID            `json:"id"`        // -> {string, uuid}
	CreatedAt time.Time            `json:"createdAt"` // -> {string, date-time}
	TTL       time.Duration        `json:"ttl"`       // -> {integer, int64} (encoding/json: ns count)
	Payload   []byte               `json:"payload"`   // -> {string, byte}
	Count     uint64               `json:"count"`     // -> {integer, int64, minimum 0}
	Labels    map[string]string    `json:"labels"`    // -> object, additionalProperties {string}
	Addrs     map[string]Address   `json:"addrs"`     // -> object, additionalProperties $ref Address
	History   map[string][]Address `json:"history"`   // -> object, additionalProperties {array, items $ref}
	Raw       json.RawMessage      `json:"raw"`       // -> {} (any JSON value)
}

// GetEventReq identifies an event by path parameter.
type GetEventReq struct {
	ID string `param:"id" validate:"required"`
}

func (h *Handler) getEvent(req GetEventReq, ctx server.HandlerContext) (server.Result[Event], server.IAPIError) {
	return server.NewResult(http.StatusOK, Event{}), nil
}

// FiscalMonth wraps time.Month and resolves through it.
type FiscalMonth time.Month

// Schedule exercises json.Number, time.Month and time.Weekday in every field
// position. encoding/json writes a json.Number as the number literal it holds
// and Month/Weekday as their int, unclamped, so neither carries a range bound
// unless a validate tag adds one.
type Schedule struct {
	Month    time.Month                `json:"month" validate:"min=1,max=12" example:"3"` // -> {integer, int64, minimum 1, maximum 12, example 3}
	Day      time.Weekday              `json:"day"`                                       // -> {integer, int64}, no bounds
	EndMonth *time.Month               `json:"endMonth"`                                  // -> {integer, int64, nullable}
	Workdays []time.Weekday            `json:"workdays" validate:"dive,min=0,max=6"`      // -> items {integer, int64, minimum 0, maximum 6}
	ByRegion map[string]time.Month     `json:"byRegion"`                                  // -> additionalProperties {integer, int64}
	Rota     [][]time.Weekday          `json:"rota"`                                      // -> array of array of {integer, int64}
	Shifts   map[string][]time.Weekday `json:"shifts"`                                    // -> additionalProperties array of {integer, int64}
	Fiscal   FiscalMonth               `json:"fiscal"`                                    // -> {integer, int64}
	Total    json.Number               `json:"total" example:"42"`                        // -> {number, example 42}
	Cap      *json.Number              `json:"cap"`                                       // -> {number, nullable}
	Rates    []json.Number             `json:"rates"`                                     // -> items {number}
	Grid     [][]json.Number           `json:"grid"`                                      // -> array of array of {number}
	Totals   map[string]json.Number    `json:"totals"`                                    // -> additionalProperties {number}
	Series   map[string][]json.Number  `json:"series"`                                    // -> additionalProperties array of {number}
}

// GetScheduleReq selects a schedule by path and query parameters of the
// well-known scalar types.
type GetScheduleReq struct {
	Month time.Month   `param:"month"`
	Day   time.Weekday `query:"day"`
	Min   json.Number  `query:"min"`
}

func (h *Handler) getSchedule(req GetScheduleReq, ctx server.HandlerContext) (server.Result[Schedule], server.IAPIError) {
	return server.NewResult(http.StatusOK, Schedule{}), nil
}

// Non-slice well-known and builtin payloads document inline, exactly as a
// struct field of the same type: no component is ever emitted for them, so a
// $ref would dangle.

func (h *Handler) rawPayload(ctx server.HandlerContext) (server.Result[json.RawMessage], server.IAPIError) { // data: {}
	return server.Result[json.RawMessage]{}, nil
}

func (h *Handler) rawPointerPayload(ctx server.HandlerContext) (server.Result[*json.RawMessage], server.IAPIError) { // data: {} (pointer shed)
	return server.Result[*json.RawMessage]{}, nil
}

func (h *Handler) createdAt(ctx server.HandlerContext) (server.Result[time.Time], server.IAPIError) { // data: {string, date-time}
	return server.Result[time.Time]{}, nil
}

func (h *Handler) eventID(ctx server.HandlerContext) (server.Result[uuid.UUID], server.IAPIError) { // data: {string, uuid}
	return server.Result[uuid.UUID]{}, nil
}

func (h *Handler) ttl(ctx server.HandlerContext) (server.Result[time.Duration], server.IAPIError) { // data: {integer, int64}
	return server.Result[time.Duration]{}, nil
}

func (h *Handler) month(ctx server.HandlerContext) (server.Result[time.Month], server.IAPIError) { // data: {integer, int64}
	return server.Result[time.Month]{}, nil
}

func (h *Handler) weekday(ctx server.HandlerContext) (server.Result[*time.Weekday], server.IAPIError) { // data: {integer, int64} (pointer shed)
	return server.Result[*time.Weekday]{}, nil
}

func (h *Handler) rates(ctx server.HandlerContext) (server.Result[[]json.Number], server.IAPIError) { // data: array of {number}
	return server.Result[[]json.Number]{}, nil
}

func (h *Handler) exactTotal(ctx server.HandlerContext) (server.ResultWithMeta[json.Number], server.IAPIError) { // data: {number}
	return server.ResultWithMeta[json.Number]{}, nil
}

func (h *Handler) name(ctx server.HandlerContext) (server.Result[string], server.IAPIError) { // data: {string}
	return server.Result[string]{}, nil
}

func (h *Handler) total(ctx server.HandlerContext) (server.Result[int64], server.IAPIError) { // data: {integer, int64}
	return server.Result[int64]{}, nil
}

func (h *Handler) count(ctx server.HandlerContext) (server.Result[uint64], server.IAPIError) { // data: {integer, int64, minimum 0}
	return server.Result[uint64]{}, nil
}

func (h *Handler) enabled(ctx server.HandlerContext) (server.Result[bool], server.IAPIError) { // data: {boolean}
	return server.Result[bool]{}, nil
}

func (h *Handler) anything(ctx server.HandlerContext) (server.Result[any], server.IAPIError) { // data: {}
	return server.Result[any]{}, nil
}

func (h *Handler) updatedAt(ctx server.HandlerContext) (server.ResultWithMeta[*time.Time], server.IAPIError) { // data: {string, date-time}
	return server.ResultWithMeta[*time.Time]{}, nil
}

func (h *Handler) rawBody(ctx server.HandlerContext) (server.Result[json.RawMessage], server.IAPIError) { // raw body: {}
	return server.Result[json.RawMessage]{}, nil
}
