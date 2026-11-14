// Package wellknownbigtypes pins math/big's Int, Float and Rat documented
// from their encoding/json wire form. Every marshal method of the three is
// on the pointer, so encoding/json uses it only where the value is
// addressable. An explicit *big.Int is always addressable and is a JSON
// number; *big.Float and *big.Rat are JSON strings ("1.5", "1/3"). A value
// spelling is {} wherever it is not addressable (a map value, a payload
// returned by value), so every value position is the untyped schema with the
// Marshaler warning. A defined type over one drops the methods and keeps the
// untyped object with its defined-type warning. A pointer embed promotes the
// methods into the embedding struct's value method set; a value embed keeps
// them pointer-only.
package wellknownbigtypes

import (
	"math/big"
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "wellknownbigtypes" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// BI is an alias of big.Int: it keeps big.Int's methods.
type BI = big.Int

// Big, BF and BR are defined types over the three: they drop the methods,
// and encoding/json writes the struct, which has no exported field, as {}.
type Big big.Int

// BF is a defined type over big.Float.
type BF big.Float

// BR is a defined type over big.Rat.
type BR big.Rat

// Numbers holds the field rows.
type Numbers struct {
	Int     *big.Int              `json:"int" example:"42"`         // -> {integer, nullable, example 42}
	Float   *big.Float            `json:"float"`                    // -> {string, nullable}
	Rat     *big.Rat              `json:"rat"`                      // -> {string, nullable}
	Ints    []*big.Int            `json:"ints"`                     // -> items {integer}
	Floats  map[string]*big.Float `json:"floats"`                   // -> additionalProperties {string}
	Alias   *BI                   `json:"alias"`                    // -> {integer, nullable}
	ValInt  big.Int               `json:"valInt"`                   // -> {}, warned
	ValFlt  big.Float             `json:"valFloat"`                 // -> {}, warned
	ValRat  big.Rat               `json:"valRat"`                   // -> {}, warned
	DefInt  *Big                  `json:"defInt"`                   // -> {object, nullable}, warned
	DefFlt  *BF                   `json:"defFloat"`                 // -> {object, nullable}, warned
	DefRat  *BR                   `json:"defRat"`                   // -> {object, nullable}, warned
	Quoted  *big.Int              `json:"quoted,string"`            // -> {integer, nullable}: ,string never quotes a struct
	QuotedF *big.Float            `json:"quotedFloat,string"`       // -> {string, nullable}
	PtrArr  *[2]big.Int           `json:"ptrArr"`                   // -> items {}, warned
	PtrMap  *map[string]big.Rat   `json:"ptrMap"`                   // -> additionalProperties {}, warned
	Bounded *big.Int              `json:"bounded" validate:"min=1"` // -> {integer, nullable}: bounds dropped, as on every well-known type
}

// EmbPtrInt embeds *big.Int: MarshalJSON is in its value method set.
type EmbPtrInt struct {
	*big.Int
}

// EmbInt embeds big.Int: MarshalJSON is pointer-only, but still its own.
type EmbInt struct {
	big.Int
}

// EmbPtrFloat embeds *big.Float: MarshalText and UnmarshalText are in its
// value method set, so it is a string both ways.
type EmbPtrFloat struct {
	*big.Float
}

// EmbFloat embeds big.Float: MarshalText stays pointer-only.
type EmbFloat struct {
	big.Float
}

// EmbPtrRat embeds *big.Rat.
type EmbPtrRat struct {
	*big.Rat
}

// EmbRat embeds big.Rat.
type EmbRat struct {
	big.Rat
}

// Embeds holds the embed rows.
type Embeds struct {
	PtrInt   EmbPtrInt   `json:"ptrInt"`   // -> {}, warned, no component
	Int      EmbInt      `json:"int"`      // -> {}, warned, no component
	PtrFloat EmbPtrFloat `json:"ptrFloat"` // -> {string}, no component
	Float    EmbFloat    `json:"float"`    // -> {}, warned, no component
	PtrRat   EmbPtrRat   `json:"ptrRat"`   // -> {string}, no component
	Rat      EmbRat      `json:"rat"`      // -> {}, warned, no component
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/numbers", m.numbers)
	server.GET(hr, r, "/embeds", m.embeds)

	server.GET(hr, r, "/payload/int", m.payloadInt)
	server.GET(hr, r, "/payload/float", m.payloadFloat)
	server.GET(hr, r, "/payload/rat", m.payloadRat)
	server.GET(hr, r, "/payload/ints", m.payloadInts)
	server.GET(hr, r, "/payload/meta-int", m.payloadMetaInt)
	server.GET(hr, r, "/payload/val-int", m.payloadValInt)
}

func (m *Module) numbers(ctx server.HandlerContext) (server.Result[Numbers], server.IAPIError) {
	return server.NewResult(http.StatusOK, Numbers{}), nil
}

func (m *Module) embeds(ctx server.HandlerContext) (server.Result[Embeds], server.IAPIError) {
	return server.NewResult(http.StatusOK, Embeds{}), nil
}

func (m *Module) payloadInt(ctx server.HandlerContext) (server.Result[*big.Int], server.IAPIError) {
	return server.Result[*big.Int]{}, nil
}

func (m *Module) payloadFloat(ctx server.HandlerContext) (server.Result[*big.Float], server.IAPIError) {
	return server.Result[*big.Float]{}, nil
}

func (m *Module) payloadRat(ctx server.HandlerContext) (server.Result[*big.Rat], server.IAPIError) {
	return server.Result[*big.Rat]{}, nil
}

func (m *Module) payloadInts(ctx server.HandlerContext) (server.Result[[]*big.Int], server.IAPIError) {
	return server.Result[[]*big.Int]{}, nil
}

func (m *Module) payloadMetaInt(ctx server.HandlerContext) (server.ResultWithMeta[*big.Int], server.IAPIError) {
	return server.ResultWithMeta[*big.Int]{}, nil
}

func (m *Module) payloadValInt(ctx server.HandlerContext) (server.Result[big.Int], server.IAPIError) {
	return server.Result[big.Int]{}, nil
}
