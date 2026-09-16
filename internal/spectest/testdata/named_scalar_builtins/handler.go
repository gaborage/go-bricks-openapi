package packets

import (
	"net/http"
	"time"

	"github.com/gaborage/go-bricks/server"
)

// Handler holds the packets HTTP handlers.
type Handler struct{}

// Flag is a named scalar over byte (Go's alias for uint8).
type Flag byte

// Code is a named scalar over rune (Go's alias for int32).
type Code rune

// Addr is a named scalar over uintptr, a machine address the tool never types.
type Addr uintptr

// Packet exercises Go's predeclared scalar aliases and their containers.
type Packet struct {
	// Named scalars carry the analyzer's 3-way kind only (no format), exactly
	// as `type Cents int64` does today.
	Flag Flag `json:"flag"` // -> {integer}
	Code Code `json:"code"` // -> {integer}
	Addr Addr `json:"addr"` // -> {object} (uintptr: never typed, see README)

	// The builtins themselves carry type AND format.
	Raw    byte    `json:"raw"`    // -> {integer, int32, minimum 0} (unsigned, no maximum)
	Char   rune    `json:"char"`   // -> {integer, int32} (signed)
	Cursor uintptr `json:"cursor"` // -> {object}, and one analyzer diagnostic

	Text   []rune   `json:"text"`   // -> array of {integer, int32} — NOT a base64 string
	Blob   []byte   `json:"blob"`   // -> {string, binary} (unchanged well-known shape)
	Chunks [][]byte `json:"chunks"` // -> array of {string, binary}
}

// GetPacketReq identifies a packet by path parameter.
type GetPacketReq struct {
	ID string `param:"id" validate:"required"`
}

func (h *Handler) getPacket(req GetPacketReq, ctx server.HandlerContext) (server.Result[Packet], server.IAPIError) {
	return server.NewResult(http.StatusOK, Packet{}), nil
}

// listSeen returns a []time.Time payload: the response items path resolves
// through the same well-known guard the struct-field path uses, so items are
// {string, date-time} rather than the object fallback.
func (h *Handler) listSeen(ctx server.HandlerContext) (server.Result[[]time.Time], server.IAPIError) {
	return server.NewResult(http.StatusOK, []time.Time{}), nil
}

// listDue returns a []*time.Time payload: a pointer element sheds its pointer
// and resolves to the same well-known scalar, never to a component $ref.
func (h *Handler) listDue(ctx server.HandlerContext) (server.Result[[]*time.Time], server.IAPIError) {
	return server.NewResult(http.StatusOK, []*time.Time{}), nil
}
