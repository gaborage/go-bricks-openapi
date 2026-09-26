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
