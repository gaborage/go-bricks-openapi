// Package namedpayloads pins response payloads that do not register as a
// project struct (#110): each documents exactly the schema a struct field of
// its type documents, the root never nullable, and none of them warns.
package namedpayloads

import (
	"github.com/example/named_payloads/b"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "payloads" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// RegisterRoutes registers one route per payload row.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/cents", m.cents)
	server.GET(hr, r, "/pcents", m.pcents)
	server.GET(hr, r, "/chain", m.chain)
	server.GET(hr, r, "/alias", m.alias)
	server.GET(hr, r, "/dur", m.dur)
	server.GET(hr, r, "/cents-meta", m.centsMeta)
	server.GET(hr, r, "/cents-raw", m.centsRaw, server.WithRawResponse())
	server.GET(hr, r, "/flag", m.flag)
	server.GET(hr, r, "/status", m.status)
	server.GET(hr, r, "/ratio", m.ratio)
	server.GET(hr, r, "/enabled", m.enabled)
	server.GET(hr, r, "/width", m.width)
	server.GET(hr, r, "/cents-list", m.centsList)
	server.GET(hr, r, "/pcents-list", m.pcentsList)
	server.GET(hr, r, "/enabled-list", m.enabledList)
	server.GET(hr, r, "/blob", m.blob)
	server.GET(hr, r, "/flag-list", m.flagList)
	server.GET(hr, r, "/raw", m.raw)
	server.GET(hr, r, "/tags", m.tags)
	server.GET(hr, r, "/ptags", m.ptags)
	server.GET(hr, r, "/tags-list", m.tagsList)
	server.GET(hr, r, "/users", m.users)
	server.GET(hr, r, "/pusers", m.pusers)
	server.GET(hr, r, "/users-meta", m.usersMeta)
	server.GET(hr, r, "/users-raw", m.usersRaw, server.WithRawResponse())
	server.GET(hr, r, "/stamp", m.stamp)
	server.GET(hr, r, "/rawa", m.rawa)
	server.GET(hr, r, "/anyd", m.anyd)
	server.GET(hr, r, "/pc", m.pc)
	server.GET(hr, r, "/pair", m.pair)
	server.GET(hr, r, "/b-cents", m.bCents)
	server.GET(hr, r, "/b-tags", m.bTags)
	server.GET(hr, r, "/map-int", m.mapInt)
	server.GET(hr, r, "/map-cents", m.mapCents)
	server.GET(hr, r, "/map-address", m.mapAddress)
	server.GET(hr, r, "/grid", m.grid)
	server.GET(hr, r, "/cents-grid", m.centsGrid)
	server.GET(hr, r, "/pstrings", m.pstrings)
	server.GET(hr, r, "/maps", m.maps)
	server.GET(hr, r, "/map-cents-list", m.mapCentsList)
	server.GET(hr, r, "/map-int-meta", m.mapIntMeta)
	server.GET(hr, r, "/map-int-raw", m.mapIntRaw, server.WithRawResponse())
	server.GET(hr, r, "/pbytes", m.pbytes)
	server.GET(hr, r, "/pflags", m.pflags)
	server.GET(hr, r, "/flag-array", m.flagArray)
	server.GET(hr, r, "/pbyte-slice", m.pbyteSlice)
	server.GET(hr, r, "/pbyte-array", m.pbyteArray)
	server.GET(hr, r, "/bare-cents", m.bareCents)
	server.GET(hr, r, "/bare-users", m.bareUsers)
	server.GET(hr, r, "/piface", m.piface)
	server.POST(hr, r, "/map-created", m.create)
	server.GET(hr, r, "/bare-map", m.bareMap)
}

func (m *Module) cents(ctx server.HandlerContext) (server.Result[Cents], server.IAPIError) {
	var v server.Result[Cents]
	return v, nil
}

func (m *Module) pcents(ctx server.HandlerContext) (server.Result[*Cents], server.IAPIError) {
	var v server.Result[*Cents]
	return v, nil
}

func (m *Module) chain(ctx server.HandlerContext) (server.Result[Chain], server.IAPIError) {
	var v server.Result[Chain]
	return v, nil
}

func (m *Module) alias(ctx server.HandlerContext) (server.Result[Alias], server.IAPIError) {
	var v server.Result[Alias]
	return v, nil
}

func (m *Module) dur(ctx server.HandlerContext) (server.Result[Dur], server.IAPIError) {
	var v server.Result[Dur]
	return v, nil
}

func (m *Module) centsMeta(ctx server.HandlerContext) (server.ResultWithMeta[Cents], server.IAPIError) {
	var v server.ResultWithMeta[Cents]
	return v, nil
}

func (m *Module) centsRaw(ctx server.HandlerContext) (server.Result[Cents], server.IAPIError) {
	var v server.Result[Cents]
	return v, nil
}

func (m *Module) flag(ctx server.HandlerContext) (server.Result[Flag], server.IAPIError) {
	var v server.Result[Flag]
	return v, nil
}

func (m *Module) status(ctx server.HandlerContext) (server.Result[Status], server.IAPIError) {
	var v server.Result[Status]
	return v, nil
}

func (m *Module) ratio(ctx server.HandlerContext) (server.Result[Ratio], server.IAPIError) {
	var v server.Result[Ratio]
	return v, nil
}

func (m *Module) enabled(ctx server.HandlerContext) (server.Result[Enabled], server.IAPIError) {
	var v server.Result[Enabled]
	return v, nil
}

func (m *Module) width(ctx server.HandlerContext) (server.Result[Width], server.IAPIError) {
	var v server.Result[Width]
	return v, nil
}

func (m *Module) centsList(ctx server.HandlerContext) (server.Result[[]Cents], server.IAPIError) {
	var v server.Result[[]Cents]
	return v, nil
}

func (m *Module) pcentsList(ctx server.HandlerContext) (server.Result[[]*Cents], server.IAPIError) {
	var v server.Result[[]*Cents]
	return v, nil
}

func (m *Module) enabledList(ctx server.HandlerContext) (server.Result[[]Enabled], server.IAPIError) {
	var v server.Result[[]Enabled]
	return v, nil
}

func (m *Module) blob(ctx server.HandlerContext) (server.Result[Blob], server.IAPIError) {
	var v server.Result[Blob]
	return v, nil
}

func (m *Module) flagList(ctx server.HandlerContext) (server.Result[[]Flag], server.IAPIError) {
	var v server.Result[[]Flag]
	return v, nil
}

func (m *Module) raw(ctx server.HandlerContext) (server.Result[Raw], server.IAPIError) {
	var v server.Result[Raw]
	return v, nil
}

func (m *Module) tags(ctx server.HandlerContext) (server.Result[Tags], server.IAPIError) {
	var v server.Result[Tags]
	return v, nil
}

func (m *Module) ptags(ctx server.HandlerContext) (server.Result[*Tags], server.IAPIError) {
	var v server.Result[*Tags]
	return v, nil
}

func (m *Module) tagsList(ctx server.HandlerContext) (server.Result[[]Tags], server.IAPIError) {
	var v server.Result[[]Tags]
	return v, nil
}

func (m *Module) users(ctx server.HandlerContext) (server.Result[UserList], server.IAPIError) {
	var v server.Result[UserList]
	return v, nil
}

func (m *Module) pusers(ctx server.HandlerContext) (server.Result[*UserList], server.IAPIError) {
	var v server.Result[*UserList]
	return v, nil
}

func (m *Module) usersMeta(ctx server.HandlerContext) (server.ResultWithMeta[UserList], server.IAPIError) {
	var v server.ResultWithMeta[UserList]
	return v, nil
}

func (m *Module) usersRaw(ctx server.HandlerContext) (server.Result[UserList], server.IAPIError) {
	var v server.Result[UserList]
	return v, nil
}

func (m *Module) stamp(ctx server.HandlerContext) (server.Result[Stamp], server.IAPIError) {
	var v server.Result[Stamp]
	return v, nil
}

func (m *Module) rawa(ctx server.HandlerContext) (server.Result[RawA], server.IAPIError) {
	var v server.Result[RawA]
	return v, nil
}

func (m *Module) anyd(ctx server.HandlerContext) (server.Result[AnyD], server.IAPIError) {
	var v server.Result[AnyD]
	return v, nil
}

func (m *Module) pc(ctx server.HandlerContext) (server.Result[PC], server.IAPIError) {
	var v server.Result[PC]
	return v, nil
}

func (m *Module) pair(ctx server.HandlerContext) (server.Result[Pair], server.IAPIError) {
	var v server.Result[Pair]
	return v, nil
}

func (m *Module) bCents(ctx server.HandlerContext) (server.Result[b.Cents], server.IAPIError) {
	var v server.Result[b.Cents]
	return v, nil
}

func (m *Module) bTags(ctx server.HandlerContext) (server.Result[b.Tags], server.IAPIError) {
	var v server.Result[b.Tags]
	return v, nil
}

func (m *Module) mapInt(ctx server.HandlerContext) (server.Result[map[string]int64], server.IAPIError) {
	var v server.Result[map[string]int64]
	return v, nil
}

func (m *Module) mapCents(ctx server.HandlerContext) (server.Result[map[string]Cents], server.IAPIError) {
	var v server.Result[map[string]Cents]
	return v, nil
}

func (m *Module) mapAddress(ctx server.HandlerContext) (server.Result[map[string]Address], server.IAPIError) {
	var v server.Result[map[string]Address]
	return v, nil
}

func (m *Module) grid(ctx server.HandlerContext) (server.Result[[][]string], server.IAPIError) {
	var v server.Result[[][]string]
	return v, nil
}

func (m *Module) centsGrid(ctx server.HandlerContext) (server.Result[[][]Cents], server.IAPIError) {
	var v server.Result[[][]Cents]
	return v, nil
}

func (m *Module) pstrings(ctx server.HandlerContext) (server.Result[*[]string], server.IAPIError) {
	var v server.Result[*[]string]
	return v, nil
}

func (m *Module) maps(ctx server.HandlerContext) (server.Result[[]map[string]int64], server.IAPIError) {
	var v server.Result[[]map[string]int64]
	return v, nil
}

func (m *Module) mapCentsList(ctx server.HandlerContext) (server.Result[map[string][]Cents], server.IAPIError) {
	var v server.Result[map[string][]Cents]
	return v, nil
}

func (m *Module) mapIntMeta(ctx server.HandlerContext) (server.ResultWithMeta[map[string]int64], server.IAPIError) {
	var v server.ResultWithMeta[map[string]int64]
	return v, nil
}

func (m *Module) mapIntRaw(ctx server.HandlerContext) (server.Result[map[string]int64], server.IAPIError) {
	var v server.Result[map[string]int64]
	return v, nil
}

func (m *Module) pbytes(ctx server.HandlerContext) (server.Result[[]*byte], server.IAPIError) {
	var v server.Result[[]*byte]
	return v, nil
}

func (m *Module) pflags(ctx server.HandlerContext) (server.Result[[]*Flag], server.IAPIError) {
	var v server.Result[[]*Flag]
	return v, nil
}

func (m *Module) flagArray(ctx server.HandlerContext) (server.Result[[4]Flag], server.IAPIError) {
	var v server.Result[[4]Flag]
	return v, nil
}

func (m *Module) pbyteSlice(ctx server.HandlerContext) (server.Result[*[]byte], server.IAPIError) {
	var v server.Result[*[]byte]
	return v, nil
}

func (m *Module) pbyteArray(ctx server.HandlerContext) (server.Result[*[4]byte], server.IAPIError) {
	var v server.Result[*[4]byte]
	return v, nil
}

func (m *Module) bareCents(ctx server.HandlerContext) (Cents, server.IAPIError) {
	var v Cents
	return v, nil
}

func (m *Module) bareUsers(ctx server.HandlerContext) (UserList, server.IAPIError) {
	var v UserList
	return v, nil
}

func (m *Module) piface(ctx server.HandlerContext) (server.Result[*interface{}], server.IAPIError) {
	var v server.Result[*interface{}]
	return v, nil
}

// create takes no request and returns a map: it is still found as the
// handler, so its 201 and inferred 404 are documented.
func (m *Module) create(ctx server.HandlerContext) (server.Result[map[string]int64], server.IAPIError) {
	if ctx.Echo == nil {
		return server.Result[map[string]int64]{}, server.NewNotFoundError("x")
	}
	return server.Created(map[string]int64{}), nil
}

// bareMap returns a bare map: its inferred 409 is documented.
func (m *Module) bareMap(ctx server.HandlerContext) (map[string]int64, server.IAPIError) {
	return nil, server.NewConflictError("x")
}
