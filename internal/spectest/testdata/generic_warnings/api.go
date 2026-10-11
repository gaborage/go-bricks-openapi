// Package genericwarnings pins issue 138: an unresolved generic type
// instantiation (G[A], G[A, B], q.G[A]) in a field, a path/query/header
// parameter, a request or a payload raises one warning, so --strict fails,
// while the emitted spec is unchanged. Substituting the type arguments is
// issue 114's.
//
// Goldens do not capture warnings: the analyzer tests carry that proof. This
// golden locks the output half. It was generated before the analyzer learned
// about instantiations and must stay byte-identical: every generic field and
// parameter is still an untyped object (or an array or map of them), no
// generic request gets a requestBody, every generic payload's data is still
// an untyped object, and no component is emitted for Page, Box, Pair or List.
// Every handler returns server.NewResult(http.StatusOK, ...) or a bare value
// and calls no error constructor, so recognising a generic-only handler
// (issue 138's status change, pinned by generic_status) moves nothing here.
package genericwarnings

import (
	"net/http"

	"github.com/example/genericwarnings/pg"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/samber/mo"
)

// Module is the genericwarnings module.
type Module struct{}

func (m *Module) Name() string                    { return "genericwarnings" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Address is a plain struct type argument.
type Address struct {
	City string `json:"city"`
}

// Receipt is the typed response of the request and param routes.
type Receipt struct {
	ID string `json:"id"`
}

// IDReq is the typed request of every payload route.
type IDReq struct {
	ID string `param:"id"`
}

// List is a generic slice.
type List[T any] []T

// Box is a generic struct.
type Box[T any] struct {
	V T `json:"v"`
}

// Page is a generic struct.
type Page[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// Pair is a generic struct with two type parameters.
type Pair[K comparable, V any] struct {
	Key K `json:"key"`
	Val V `json:"val"`
}

// ID is a generic named string.
type ID[T any] string

// AddrPage is a defined type over a struct instantiation.
type AddrPage Page[Address]

// AliasPage is an alias of a struct instantiation.
type AliasPage = Page[Address]

// LP is a named slice of a struct instantiation.
type LP []Page[Address]

// AL is a defined type over a slice instantiation.
type AL List[Address]

// AddrSlice is a named slice of a struct.
type AddrSlice []Address

// Status is a named string.
type Status string

// JBox has MarshalJSON on a generic receiver.
type JBox[T any] struct {
	V T
}

func (b JBox[T]) MarshalJSON() ([]byte, error) { return nil, nil }

// TBox is a text marshaler on a generic receiver.
type TBox[T any] struct {
	V T
}

func (b TBox[T]) MarshalText() ([]byte, error) { return nil, nil }
func (b *TBox[T]) UnmarshalText([]byte) error  { return nil }

// Tree is a recursive generic map.
type Tree[T any] map[string]Tree[T]

// NTree is a recursive non-generic map.
type NTree map[string]NTree

// Fields holds one body field per generic form. It is both a request body and
// a response payload.
type Fields struct {
	L             List[string]        `json:"l"`
	B             Box[int]            `json:"b"`
	P             Page[Address]       `json:"p"`
	PR            Pair[string, int]   `json:"pr"`
	AP            AddrPage            `json:"ap"`
	AAP           AliasPage           `json:"aap"`
	ALF           AL                  `json:"al"`
	JB            JBox[int]           `json:"jb"`
	TB            TBox[int]           `json:"tb"`
	TR            Tree[int]           `json:"tr"`
	PGP           pg.Page[Address]    `json:"pgp"`
	PGL           pg.List[Address]    `json:"pgl"`
	MO            mo.Option[Address]  `json:"mo"`
	UD            Undeclared[Address] `json:"ud"`
	PB            *Box[Address]       `json:"pb"`
	SB            []Box[Address]      `json:"sb"`
	LPF           LP                  `json:"lp"`
	MB            map[string]Box[int] `json:"mb"`
	Page[Address] `json:"page"`
}

// Params holds one generic field per parameter location.
type Params struct {
	IDs  List[string] `query:"ids"`
	ID   ID[int]      `param:"id"`
	Tags List[string] `header:"X-Tags"`
	BQ   Box[int]     `query:"b"`
}

// Controls holds the shapes issue 138 must not move.
type Controls struct {
	Page[Address]
	Note string   `json:"note"`
	X    T        `json:"x"`
	NT   NTree    `json:"nt"`
	Ch   chan int `json:"ch"`
	Fn   func()   `json:"fn"`
	Anon struct {
		X int `json:"x"`
	} `json:"anon"`
}

// RegisterRoutes registers one route per generic position.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/fields", m.fields, server.WithTags("generic"))
	server.GET(hr, r, "/params/:id", m.params, server.WithTags("generic"))
	server.GET(hr, r, "/controls/:id", m.controls, server.WithTags("generic"))

	server.POST(hr, r, "/req/box", m.reqBox, server.WithTags("generic"))
	server.POST(hr, r, "/req/box-ptr", m.reqBoxPtr, server.WithTags("generic"))
	server.POST(hr, r, "/req/pair", m.reqPair, server.WithTags("generic"))
	server.POST(hr, r, "/req/pg-page", m.reqPgPage, server.WithTags("generic"))
	server.POST(hr, r, "/req/addr-page", m.reqAddrPage, server.WithTags("generic"))
	server.POST(hr, r, "/req/alias-page", m.reqAliasPage, server.WithTags("generic"))
	server.POST(hr, r, "/req/list", m.reqList, server.WithTags("generic"))
	server.POST(hr, r, "/req/list-ptr", m.reqListPtr, server.WithTags("generic"))
	server.POST(hr, r, "/req/pg-list", m.reqPgList, server.WithTags("generic"))
	server.POST(hr, r, "/req/option", m.reqOption, server.WithTags("generic"))
	server.POST(hr, r, "/req/undeclared", m.reqUndeclared, server.WithTags("generic"))
	server.POST(hr, r, "/req/al", m.reqAL, server.WithTags("generic"))
	server.POST(hr, r, "/req/lp", m.reqLP, server.WithTags("generic"))
	server.POST(hr, r, "/req/slice", m.reqSlice, server.WithTags("generic"))
	server.POST(hr, r, "/req/addr-slice", m.reqAddrSlice, server.WithTags("generic"))
	server.POST(hr, r, "/req/status", m.reqStatus, server.WithTags("generic"))

	server.GET(hr, r, "/pay/page/:id", m.payPage, server.WithTags("generic"))
	server.GET(hr, r, "/pay/page-meta/:id", m.payPageMeta, server.WithTags("generic"))
	server.GET(hr, r, "/pay/page-slice/:id", m.payPageSlice, server.WithTags("generic"))
	server.GET(hr, r, "/pay/page-ptr/:id", m.payPagePtr, server.WithTags("generic"))
	server.GET(hr, r, "/pay/page-map/:id", m.payPageMap, server.WithTags("generic"))
	server.GET(hr, r, "/pay/list/:id", m.payList, server.WithTags("generic"))
	server.GET(hr, r, "/pay/box/:id", m.payBox, server.WithTags("generic"))
	server.GET(hr, r, "/pay/pair/:id", m.payPair, server.WithTags("generic"))
	server.GET(hr, r, "/pay/pg-page/:id", m.payPgPage, server.WithTags("generic"))
	server.GET(hr, r, "/pay/option/:id", m.payOption, server.WithTags("generic"))
	server.GET(hr, r, "/pay/bare-page/:id", m.payBarePage, server.WithTags("generic"))
	server.GET(hr, r, "/pay/bare-page-ptr/:id", m.payBarePagePtr, server.WithTags("generic"))
	server.GET(hr, r, "/pay/bare-pair/:id", m.payBarePair, server.WithTags("generic"))
	server.GET(hr, r, "/pay/bare-list/:id", m.payBareList, server.WithTags("generic"))
	server.GET(hr, r, "/pay/addr-page/:id", m.payAddrPage, server.WithTags("generic"))
	server.GET(hr, r, "/pay/alias-page/:id", m.payAliasPage, server.WithTags("generic"))
	server.GET(hr, r, "/pay/lp/:id", m.payLP, server.WithTags("generic"))

	server.POST(hr, r, "/untyped/box-page", m.untypedBoxPage, server.WithTags("generic"))
	server.GET(hr, r, "/untyped/page", m.untypedPage, server.WithTags("generic"))
}

func (m *Module) fields(req Fields, ctx server.HandlerContext) (server.Result[Fields], server.IAPIError) {
	return server.NewResult(http.StatusOK, Fields{}), nil
}

func (m *Module) params(req Params, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) controls(req IDReq, ctx server.HandlerContext) (server.Result[Controls], server.IAPIError) {
	return server.NewResult(http.StatusOK, Controls{}), nil
}

func (m *Module) reqBox(req Box[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqBoxPtr(req *Box[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqPair(req Pair[string, int], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqPgPage(req pg.Page[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqAddrPage(req AddrPage, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqAliasPage(req AliasPage, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqList(req List[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqListPtr(req *List[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqPgList(req pg.List[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqOption(req mo.Option[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqUndeclared(req Undeclared[Address], ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqAL(req AL, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqLP(req LP, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqSlice(req []Address, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqAddrSlice(req AddrSlice, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) reqStatus(req Status, ctx server.HandlerContext) (server.Result[Receipt], server.IAPIError) {
	return server.NewResult(http.StatusOK, Receipt{}), nil
}

func (m *Module) payPage(req IDReq, ctx server.HandlerContext) (server.Result[Page[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, Page[Address]{}), nil
}

func (m *Module) payPageMeta(req IDReq, ctx server.HandlerContext) (server.ResultWithMeta[Page[Address]], server.IAPIError) {
	var v server.ResultWithMeta[Page[Address]]
	return v, nil
}

func (m *Module) payPageSlice(req IDReq, ctx server.HandlerContext) (server.Result[[]Page[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, []Page[Address]{}), nil
}

func (m *Module) payPagePtr(req IDReq, ctx server.HandlerContext) (server.Result[*Page[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, &Page[Address]{}), nil
}

func (m *Module) payPageMap(req IDReq, ctx server.HandlerContext) (server.Result[map[string]Page[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, map[string]Page[Address]{}), nil
}

func (m *Module) payList(req IDReq, ctx server.HandlerContext) (server.Result[List[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, List[Address]{}), nil
}

func (m *Module) payBox(req IDReq, ctx server.HandlerContext) (server.Result[Box[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, Box[Address]{}), nil
}

func (m *Module) payPair(req IDReq, ctx server.HandlerContext) (server.Result[Pair[string, int]], server.IAPIError) {
	return server.NewResult(http.StatusOK, Pair[string, int]{}), nil
}

func (m *Module) payPgPage(req IDReq, ctx server.HandlerContext) (server.Result[pg.Page[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, pg.Page[Address]{}), nil
}

func (m *Module) payOption(req IDReq, ctx server.HandlerContext) (server.Result[mo.Option[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, mo.None[Address]()), nil
}

func (m *Module) payBarePage(req IDReq, ctx server.HandlerContext) (Page[Address], server.IAPIError) {
	return Page[Address]{}, nil
}

func (m *Module) payBarePagePtr(req IDReq, ctx server.HandlerContext) (*Page[Address], server.IAPIError) {
	return &Page[Address]{}, nil
}

func (m *Module) payBarePair(req IDReq, ctx server.HandlerContext) (Pair[string, int], server.IAPIError) {
	return Pair[string, int]{}, nil
}

func (m *Module) payBareList(req IDReq, ctx server.HandlerContext) (List[Address], server.IAPIError) {
	return List[Address]{}, nil
}

func (m *Module) payAddrPage(req IDReq, ctx server.HandlerContext) (server.Result[AddrPage], server.IAPIError) {
	return server.NewResult(http.StatusOK, AddrPage{}), nil
}

func (m *Module) payAliasPage(req IDReq, ctx server.HandlerContext) (server.Result[AliasPage], server.IAPIError) {
	return server.NewResult(http.StatusOK, AliasPage{}), nil
}

func (m *Module) payLP(req IDReq, ctx server.HandlerContext) (server.Result[LP], server.IAPIError) {
	return server.NewResult(http.StatusOK, LP{}), nil
}

func (m *Module) untypedBoxPage(req Box[Address], ctx server.HandlerContext) (server.Result[Page[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, Page[Address]{}), nil
}

func (m *Module) untypedPage(ctx server.HandlerContext) (server.Result[Page[Address]], server.IAPIError) {
	return server.NewResult(http.StatusOK, Page[Address]{}), nil
}
