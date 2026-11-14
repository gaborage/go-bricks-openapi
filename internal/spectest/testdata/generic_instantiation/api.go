// Package genericinstantiation pins issue 114's component names (ADR 0003):
// each distinct struct Instantiation is one component, named by its generic
// type and each type argument joined by -, and shared by every spelling of
// one Go type. Rows holds one field per naming, sharing and identity row; a
// generic request whose struct holds only parameters documents them, with
// substituted types, and no orphaned component. Defined holds the defined
// types over a generic Marshaler type, which drop its methods, and the
// Instantiations whose arguments hold a func or chan, which are refused.
package genericinstantiation

import (
	"time"
	t "time"

	"github.com/example/genericinstantiation/billing"
	ma "github.com/example/genericinstantiation/modelsa"
	mb "github.com/example/genericinstantiation/modelsb"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Module is the genericinstantiation module.
type Module struct{}

func (m *Module) Name() string                    { return "genericinstantiation" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Address is a struct type argument with a component of its own.
type Address struct {
	City string `json:"city"`
}

// User is a second struct type argument.
type User struct {
	Name string `json:"name"`
}

// Box is a generic struct with one type parameter.
type Box[T any] struct {
	V T `json:"v"`
}

// Page is a generic struct with a slice of its type parameter.
type Page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// Pair is a generic struct with two type parameters.
type Pair[K comparable, V any] struct {
	Key K `json:"key"`
	Val V `json:"val"`
}

// Node is a recursive generic struct: its own Instantiation's component.
type Node[T any] struct {
	Val  T        `json:"val"`
	Next *Node[T] `json:"next"`
}

// List is a generic slice, inlined wherever it is written.
type List[T any] []T

// AliasPage is an alias of Page[Address]: it documents as Page-Address.
type AliasPage = Page[Address]

// AddrPage is a defined type over Page[Address]: its own component.
type AddrPage Page[Address]

// Query is a generic struct of parameters only.
type Query[T any] struct {
	ID   T              `param:"id"`
	Tags List[string]   `query:"tags"`
	Opt  List[T]        `header:"X-Opt"`
	_    struct{}       `json:"-"`
	note map[string]int // unexported: not documented
}

// Rows holds one field per row of ADR 0003.
type Rows struct {
	P1  Page[Address]           `json:"p1"`
	P2  Page[Address]           `json:"p2"`
	PU  Page[User]              `json:"pu"`
	PR  Pair[string, int]       `json:"pr"`
	PB  Page[Box[Address]]      `json:"pb"`
	BP  Box[*Address]           `json:"bp"`
	BS  Box[[]Address]          `json:"bs"`
	BM  Box[map[string]int]     `json:"bm"`
	BT  Box[time.Time]          `json:"bt"`
	BTA Box[t.Time]             `json:"bta"`
	BTS Box[billing.Timestamp]  `json:"bts"`
	BU  Box[uuid.UUID]          `json:"bu"`
	BD  Box[decimal.Decimal]    `json:"bd"`
	BC  Box[billing.Cents]      `json:"bc"`
	BMo Box[billing.Money]      `json:"bmo"`
	BA  Box[any]                `json:"ba"`
	BI  Box[interface{}]        `json:"bi"`
	B3  Box[[3]int]             `json:"b3"`
	B5  Box[[5]int]             `json:"b5"`
	BX  Box[struct{ X int }]    `json:"bx"`
	IDA Box[ma.ID]              `json:"ida"`
	IDB Box[mb.ID]              `json:"idb"`
	BPG billing.Page[Address]   `json:"bpg"`
	AP  AliasPage               `json:"ap"`
	DP  AddrPage                `json:"dp"`
	N   Node[int]               `json:"n"`
	L   List[Address]           `json:"l"`
	LS  List[string]            `json:"ls"`
	ML  map[string]List[string] `json:"ml"`
}

// MJ is a generic text marshaler over a slice.
type MJ[T any] []T

// MarshalText makes MJ a text marshaler.
func (MJ[T]) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText makes MJ a string both ways.
func (*MJ[T]) UnmarshalText([]byte) error { return nil }

// JBox is a generic struct Marshaler type.
type JBox[T any] struct {
	V T `json:"v"`
}

// MarshalJSON makes JBox a Marshaler type.
func (JBox[T]) MarshalJSON() ([]byte, error) { return nil, nil }

// DM is a defined type over MJ[int]: an array, MJ's methods dropped.
type DM MJ[int]

// GM is a generic defined type over MJ: an array too.
type GM[T any] MJ[T]

// GD is a generic defined type over JBox: JBox[T]'s fields, no method.
type GD[T any] JBox[T]

// EmbedDM embeds DM: one array property.
type EmbedDM struct {
	DM
	K int `json:"k"`
}

// Defined holds the defined types over generic Marshaler types and the
// refused func or chan arguments.
type Defined struct {
	MJ MJ[int]       `json:"mj"`
	DM DM            `json:"dm"`
	GM GM[int]       `json:"gm"`
	GD GD[int]       `json:"gd"`
	ED EmbedDM       `json:"ed"`
	BC Box[chan int] `json:"bc"`
	BF Box[func()]   `json:"bf"`
}

// RegisterRoutes registers the Rows payload, a generic request body and a
// params-only generic request.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/rows", m.rows, server.WithTags("generic"))
	server.POST(hr, r, "/boxes", m.box, server.WithTags("generic"))
	server.GET(hr, r, "/queries/:id", m.query, server.WithTags("generic"))
	server.GET(hr, r, "/defined", m.defined, server.WithTags("generic"))
}

func (m *Module) defined(ctx server.HandlerContext) (server.Result[Defined], server.IAPIError) {
	return server.Result[Defined]{}, nil
}

func (m *Module) rows(ctx server.HandlerContext) (server.Result[Rows], server.IAPIError) {
	return server.Result[Rows]{}, nil
}

func (m *Module) box(req Box[Address], ctx server.HandlerContext) (server.Result[Pair[string, User]], server.IAPIError) {
	return server.Result[Pair[string, User]]{}, nil
}

func (m *Module) query(req Query[int], ctx server.HandlerContext) (server.Result[List[Page[User]]], server.IAPIError) {
	return server.Result[List[Page[User]]]{}, nil
}
