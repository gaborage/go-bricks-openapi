// Package fixedarray is a regression fixture for issue #98: encoding/json
// base64-encodes only a byte SLICE and writes a fixed-size array element by
// element, so [N]byte and [N]uint8 are integer arrays (the items a bare byte
// field emits), never `format: byte` strings. CreateKeyReq covers each array
// form (sized by a literal, by zero, by a constant, behind a pointer, nested in
// a slice, an array and a map) and the validate and example tags, which must
// match a [4]uint16 field's exactly. It also holds the controls that must not
// move: arrays of a named byte, a byte pointer, a wider integer, int, a byte
// slice, a struct and a named scalar, plus the []byte forms that stay base64.
// The routes cover the same split for Result/ResultWithMeta payloads and for a
// query parameter.
package fixedarray

import (
	"net/http"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the fixedarray module.
type Module struct{}

func (m *Module) Name() string                    { return "fixedarray" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// N sizes an array by a constant, a length the AST cannot read.
const N = 4

// Flag is a named byte: an array of it was already an integer array.
type Flag byte

// Cents is a named integer scalar.
type Cents int64

// Address is a project struct.
type Address struct {
	Street string `json:"street"`
}

// ListKeysReq carries a fixed-size byte array as a query parameter.
type ListKeysReq struct {
	Prefix [4]byte `query:"prefix"`
}

// CreateKeyReq exercises every fixed-size array field form.
type CreateKeyReq struct {
	Digest      [32]byte              `json:"digest"`
	Key         [4]byte               `json:"key"`
	Octets      [4]uint8              `json:"octets"`
	Empty       [0]byte               `json:"empty"`
	Sized       [N]byte               `json:"sized"`
	MaybeKey    *[4]byte              `json:"maybeKey"`
	Keys        [][4]byte             `json:"keys"`
	KeyPairs    [2][4]byte            `json:"keyPairs"`
	KeyMap      map[string][4]byte    `json:"keyMap"`
	Bounded     [4]byte               `json:"bounded" validate:"min=1,max=4,dive,max=200"`
	BoundedWide [4]uint16             `json:"boundedWide" validate:"min=1,max=4,dive,max=200"`
	Sample      [4]byte               `json:"sample" example:"AQIDBA=="`
	SampleWide  [4]uint16             `json:"sampleWide" example:"AQIDBA=="`
	Flags       [4]Flag               `json:"flags"`
	BytePtrs    [4]*byte              `json:"bytePtrs"`
	Wide        [4]uint16             `json:"wide"`
	Ints        [3]int                `json:"ints"`
	MaybeInts   *[4]int               `json:"maybeInts"`
	Blobs       [2][]byte             `json:"blobs"`
	Addrs       [2]Address            `json:"addrs"`
	AddrPtrs    [2]*Address           `json:"addrPtrs"`
	MaybeAddrs  *[2]Address           `json:"maybeAddrs"`
	AddrMap     map[string][2]Address `json:"addrMap"`
	Amounts     [3]Cents              `json:"amounts" validate:"dive,min=1"`
	Names       [2]string             `json:"names" validate:"dive,min=2"`
	Raw         []byte                `json:"raw"`
	MaybeRaw    *[]byte               `json:"maybeRaw"`
	RawOctets   []uint8               `json:"rawOctets"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/v1/keys", m.listKeys, server.WithTags("keys"))
	server.POST(hr, r, "/v1/keys", m.createKey, server.WithTags("keys"))
	server.GET(hr, r, "/v1/keys/pointers", m.keyPointers, server.WithTags("keys"))
	server.GET(hr, r, "/v1/keys/meta", m.keyWithMeta, server.WithTags("keys"))
	server.GET(hr, r, "/v1/addresses", m.addresses, server.WithTags("keys"))
	server.GET(hr, r, "/v1/addresses/pointers", m.addressPointers, server.WithTags("keys"))
	server.GET(hr, r, "/v1/counts", m.counts, server.WithTags("keys"))
	server.GET(hr, r, "/v1/times", m.times, server.WithTags("keys"))
	server.GET(hr, r, "/v1/blob", m.blob, server.WithTags("keys"))
}

// listKeys returns a [4]byte payload: data is an integer array.
func (m *Module) listKeys(req ListKeysReq, ctx server.HandlerContext) (server.Result[[4]byte], server.IAPIError) {
	return server.NewResult(http.StatusOK, [4]byte{}), nil
}

// createKey returns a [32]uint8 payload: data is an integer array.
func (m *Module) createKey(req CreateKeyReq, ctx server.HandlerContext) (server.Result[[32]uint8], server.IAPIError) {
	return server.NewResult(http.StatusOK, [32]uint8{}), nil
}

// keyPointers returns an array of byte pointers, documented like [4]byte.
func (m *Module) keyPointers(ctx server.HandlerContext) (server.Result[[4]*byte], server.IAPIError) {
	return server.OK([4]*byte{}), nil
}

// keyWithMeta proves ResultWithMeta behaves identically to Result.
func (m *Module) keyWithMeta(ctx server.HandlerContext) (server.ResultWithMeta[[4]byte], server.IAPIError) {
	return server.OKWithMeta([4]byte{}, nil), nil
}

// addresses returns an array of a struct: data is an array of $ref.
func (m *Module) addresses(ctx server.HandlerContext) (server.Result[[2]Address], server.IAPIError) {
	return server.OK([2]Address{}), nil
}

// addressPointers returns an array of struct pointers, documented like values.
func (m *Module) addressPointers(ctx server.HandlerContext) (server.Result[[2]*Address], server.IAPIError) {
	return server.OK([2]*Address{}), nil
}

// counts returns an int array.
func (m *Module) counts(ctx server.HandlerContext) (server.Result[[3]int], server.IAPIError) {
	return server.OK([3]int{}), nil
}

// times returns an array of a well-known type: no Time component is emitted.
func (m *Module) times(ctx server.HandlerContext) (server.Result[[2]time.Time], server.IAPIError) {
	return server.OK([2]time.Time{}), nil
}

// blob returns a byte slice: still a base64 string.
func (m *Module) blob(ctx server.HandlerContext) (server.Result[[]byte], server.IAPIError) {
	return server.OK([]byte{}), nil
}
