// Package wellknownimports pins how a qualified name reaches a well-known
// type: through the import path its qualifier denotes, never its spelling. A
// project package named like the owner of a well-known type (net, fs, os,
// time) is documented from source, validate keywords included; the stdlib
// packages and the encoding/json types imported beside them under an alias
// are the well-known types, a field, a defined type over one and a payload
// alike; a UUID library whose path base is not its package name still owns
// uuid.UUID; and a drop-in json package, whose json.Number is encoding/json's
// through a type alias, is read by its spelling, as the well-known type.
package wellknownimports

import (
	j "encoding/json"
	stdtime "time"

	"github.com/example/wellknownimports/fs"
	"github.com/example/wellknownimports/net"
	"github.com/example/wellknownimports/os"
	"github.com/example/wellknownimports/time"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/gofrs/uuid/v5"
	"github.com/segmentio/encoding/json"
)

type Module struct{}

func (m *Module) Name() string                    { return "wellknownimports" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Timeout is a defined type over the stdlib time.Duration: an int64.
type Timeout stdtime.Duration

// Perm is a defined type over the project's fs.FileMode: a string.
type Perm fs.FileMode

// Imports holds one field per row. Each comment is the schema the field
// emits.
type Imports struct {
	Mode    fs.FileMode      `json:"mode" validate:"max=3"` // -> {string, maxLength 3}
	IP      net.IP           `json:"ip" validate:"max=45"`  // -> {string, maxLength 45}
	OSMode  os.FileMode      `json:"osMode"`                // -> {boolean}
	Dur     time.Duration    `json:"dur"`                   // -> {string}
	Perm    Perm             `json:"perm"`                  // -> {string}
	StdDur  stdtime.Duration `json:"stdDur"`                // -> {integer, int64}
	At      stdtime.Time     `json:"at"`                    // -> {string, date-time}
	Timeout Timeout          `json:"timeout"`               // -> {integer, int64}
	Raw     j.RawMessage     `json:"raw"`                   // -> {}
	Num     j.Number         `json:"num"`                   // -> {number}
	ID      uuid.UUID        `json:"id"`                    // -> {string, uuid}
	DropIn  json.Number      `json:"dropIn"`                // -> {number}
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/imports", m.imports)
	server.GET(hr, r, "/at", m.at)
	server.GET(hr, r, "/ip", m.ip)
}

func (m *Module) imports(ctx server.HandlerContext) (server.Result[Imports], server.IAPIError) {
	var v server.Result[Imports]
	return v, nil
}

// at returns the stdlib time.Time under its alias: data is a date-time.
func (m *Module) at(ctx server.HandlerContext) (server.Result[stdtime.Time], server.IAPIError) {
	var v server.Result[stdtime.Time]
	return v, nil
}

// ip returns the project's net.IP: data is a string.
func (m *Module) ip(ctx server.HandlerContext) (server.Result[net.IP], server.IAPIError) {
	var v server.Result[net.IP]
	return v, nil
}
