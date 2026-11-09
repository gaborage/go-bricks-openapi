// Package narrowintbounds is a regression fixture for issue #107: the integer
// widths narrower than their emitted format (int8, int16, uint8/byte, uint16,
// uint32) carry their Go type's range as inclusive minimum/maximum, at every
// position the leaf type mapper reaches: a field, a pointer, slice/array items,
// map values, a named scalar, path/query/header params and Result payloads.
// An explicit validate bound overwrites the matching stamped keyword even when
// it is looser, and a one-sided bound keeps the other side's stamp. An example
// outside the range drops unless a looser explicit bound re-admits it. The
// controls (int32, rune, int, int64, uint, uint64, []byte, floats) do not move.
package narrowintbounds

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the narrowintbounds module.
type Module struct{}

func (m *Module) Name() string                    { return "narrowintbounds" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// N sizes an array by a constant.
const N = 4

// Level is a named int8.
type Level int8

// Port is a named uint16.
type Port uint16

// Widths exercises every narrow width, its positions, bounds and examples.
type Widths struct {
	Tiny      int8              `json:"tiny"`
	Small     int16             `json:"small"`
	Octet     uint8             `json:"octet"`
	Raw       byte              `json:"raw"`
	Short     uint16            `json:"short"`
	Word      uint32            `json:"word"`
	MaybeTiny *int8             `json:"maybeTiny"`
	Shorts    []uint16          `json:"shorts"`
	Pair      [2]int8           `json:"pair"`
	Sized     [N]byte           `json:"sized"`
	Words     map[string]uint32 `json:"words"`
	Level     Level             `json:"level"`
	Port      Port              `json:"port"`

	Plain    int32   `json:"plain"`
	Char     rune    `json:"char"`
	Count    int     `json:"count"`
	Big      int64   `json:"big"`
	Unsigned uint    `json:"unsigned"`
	Huge     uint64  `json:"huge"`
	Blob     []byte  `json:"blob"`
	Ratio    float32 `json:"ratio"`
	Score    float64 `json:"score"`

	Capped  uint8    `json:"capped" validate:"max=100"`
	Loose   uint16   `json:"loose" validate:"max=70000"`
	Below   int8     `json:"below" validate:"lt=50"`
	Above   int16    `json:"above" validate:"gt=-10"`
	Limited []uint16 `json:"limited" validate:"dive,max=10"`

	TooBig      uint8  `json:"tooBig" example:"300"`
	TooSmall    int8   `json:"tooSmall" example:"-200"`
	Fits        uint8  `json:"fits" example:"200"`
	Edge        int16  `json:"edge" example:"-32768"`
	LooseSample uint16 `json:"looseSample" validate:"max=70000" example:"68000"`
}

// GetWidthReq carries narrow widths as path, query and header params.
type GetWidthReq struct {
	ID    uint32 `param:"id"`
	Page  uint16 `query:"page"`
	Shift int8   `query:"shift"`
	Hops  uint8  `header:"X-Hops"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/v1/widths", m.createWidths, server.WithTags("widths"))
	server.GET(hr, r, "/v1/widths/:id", m.getWidth, server.WithTags("widths"))
}

// createWidths returns an int16 payload.
func (m *Module) createWidths(req Widths, ctx server.HandlerContext) (server.Result[int16], server.IAPIError) {
	return server.NewResult(http.StatusCreated, int16(0)), nil
}

// getWidth returns a []uint16 payload.
func (m *Module) getWidth(req GetWidthReq, ctx server.HandlerContext) (server.Result[[]uint16], server.IAPIError) {
	return server.OK([]uint16{}), nil
}
