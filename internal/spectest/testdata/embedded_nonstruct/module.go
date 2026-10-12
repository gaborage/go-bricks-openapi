// Package embeddednonstruct pins untagged embeds of non-struct types: an
// exported one is one property keyed by its type name, as encoding/json
// writes it; alias and defined types over a struct promote its fields; an
// unexported embed, a predeclared type and a well-known Marshaler one add
// nothing, but a defined type over json.RawMessage is a property; an unreadable
// type and a generic instantiation add nothing and warn. A json name nests an
// unexported struct embed, but not an unexported non-struct one.
package embeddednonstruct

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/example/embeddednonstruct/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "embeddednonstruct" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows)
	server.GET(hr, r, "/search/:id", m.search)
	server.GET(hr, r, "/stamped/:id", m.stamped)
}

// Cents is a named scalar.
type Cents int64

// Attrs is a named map.
type Attrs map[string]string

// Namer is a named interface: encoding/json writes the value it holds.
type Namer interface{ Name() string }

// CA is an alias of Cents: the property is keyed CA.
type CA = Cents

// RawD is a defined type over json.RawMessage: it drops its methods, so it
// is a byte string, as a field of it is.
type RawD json.RawMessage

// cents is unexported: encoding/json skips it.
type cents int64

// Base is a local struct.
type Base struct {
	Label string `json:"label"`
}

// BA is an alias of Base and BD a defined type over b.Base: both promote.
type BA = Base

// BD is a defined type over b.Base.
type BD b.Base

// NS is an alias of a type from outside the module: it cannot be read.
type NS = sql.NullString

// Page is a generic struct: an embedded instantiation is not expanded.
type Page[T any] struct {
	Items []T `json:"items"`
}

// Scalars embeds named non-struct types, local and qualified, by value and
// by pointer.
type Scalars struct {
	Cents
	*b.Tags
	Attrs
	Namer
	CA
	time.Duration
	RawD
	Note string `json:"note"`
}

// Promoted embeds an alias and a defined type over structs.
type Promoted struct {
	BA
	*BD
}

// PromotedB embeds b.Base2, an alias declared in b, with a name-less tag.
type PromotedB struct {
	b.Base2 `json:",omitempty"`
	Extra   int `json:"extra"`
}

// Dropped embeds what adds no property: an unexported type, a predeclared
// one, an unreadable type and a generic instantiation. (A well-known Marshaler embed would make
// Dropped itself a Marshaler type; StampedReq has one.)
type Dropped struct {
	cents
	error
	NS
	Page[Base]
	Kept string `json:"kept"`
}

// hidden is an unexported struct.
type hidden struct {
	Detail string `json:"detail"`
}

// Named embeds under json names: encoding/json writes the unexported struct
// under its name, but skips the unexported non-struct before it reads the
// tag.
type Named struct {
	cents  `json:"c"`
	hidden `json:"h"`
	Keep   int `json:"keep"`
}

// Shadowed's tagged field beats its Cents embed at one depth.
type Shadowed struct {
	Cents
	Amount string `json:"Cents"`
}

// HasCents carries a Cents property one level down.
type HasCents struct {
	Cents
}

// HasAmount's tagged field takes the JSON name the Cents property carries.
type HasAmount struct {
	Amount string `json:"Cents"`
}

// Collided promotes both at one depth: the tagged field wins, as it would
// declared first.
type Collided struct {
	HasCents
	HasAmount
}

// Rows holds one field per row struct.
type Rows struct {
	Scalars  Scalars   `json:"scalars"`
	Promoted Promoted  `json:"promoted"`
	PromB    PromotedB `json:"promB"`
	Dropped  Dropped   `json:"dropped"`
	Named    Named     `json:"named"`
	Shadowed Shadowed  `json:"shadowed"`
	Collided Collided  `json:"collided"`
}

// MinFilter carries a min query parameter one level down.
type MinFilter struct {
	Min int `query:"min" json:"min"`
}

// IDRef carries an id path parameter one level down.
type IDRef struct {
	ID string `param:"id"`
}

// SearchReq takes a named scalar embed as a query parameter beside its path
// parameter; its own min parameter beats MinFilter's, so min is listed once.
type SearchReq struct {
	Cents `query:"min" validate:"min=1"`
	MinFilter
	ID string `param:"id"`
}

// StampedReq embeds time.Time: a struct Marshaler request read only for its
// parameters, with no property for the embed. Its own id parameter beats
// IDRef's.
type StampedReq struct {
	time.Time
	IDRef
	ID string `param:"id"`
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.NewResult(http.StatusOK, Rows{}), nil
}

func (m *Module) search(req SearchReq, ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.NewResult(http.StatusOK, Rows{}), nil
}

func (m *Module) stamped(req StampedReq, ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.NewResult(http.StatusOK, Rows{}), nil
}
