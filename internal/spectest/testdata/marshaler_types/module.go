// Package marshalertypes pins Marshaler types (#111): string-both-ways types
// are strings, every other Marshaler type is {} with a warning, and no
// struct Marshaler type registers a component.
package marshalertypes

import (
	"github.com/example/marshaler_types/domain"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "marshalertypes" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// WithCode promotes domain.Tag's text pair across packages.
type WithCode struct {
	domain.Tag
	X int `json:"x"`
}

// Fields holds one field per row.
type Fields struct {
	Level     Level             `json:"level" validate:"required,min=1,max=3" example:"high"`
	LevelPtr  *Level            `json:"levelPtr"`
	LevelA    LA                `json:"levelA"`
	LevelP    LevelP            `json:"levelP"`
	LevelOne  Level             `json:"levelOne" validate:"oneof=1 2 3" example:"2"`
	Levels    []Level           `json:"levels" validate:"min=1,dive,max=3"`
	Grid      [][]Level         `json:"grid"`
	LevelMap  map[string]Level  `json:"levelMap"`
	LevelPMap map[string]LevelP `json:"levelPMap"`
	Flags     []FlagT2          `json:"flags"`
	Code      Code              `json:"code" validate:"min=2,max=5,email"`
	LD        LD                `json:"ld"`
	StatusV   StatusV           `json:"statusV"`
	LevelU    LevelU            `json:"levelU"`
	Both      Both              `json:"both"`
	TextJ     TextJ             `json:"textJ"`
	TextApp   TextApp           `json:"textApp"`
	AppOnly   AppOnly           `json:"appOnly"`
	ToText    ToText            `json:"toText"`
	FromOnly  FromOnly          `json:"fromOnly"`

	Money     Money                   `json:"money"`
	MoneyPtr  *Money                  `json:"moneyPtr"`
	Moneys    []Money                 `json:"moneys"`
	MoneyMap  map[string]Money        `json:"moneyMap"`
	MoneyP    MoneyP                  `json:"moneyP"`
	MoneyU    MoneyU                  `json:"moneyU"`
	MA        MA                      `json:"ma"`
	Price     domain.Price            `json:"price"`
	MoneyPPtr *MoneyP                 `json:"moneyPPtr"`
	MoneyPs   []MoneyP                `json:"moneyPs"`
	MoneyPMap map[string]MoneyP       `json:"moneyPMap"`
	MoneyUPtr *MoneyU                 `json:"moneyUPtr"`
	MoneyUs   []MoneyU                `json:"moneyUs"`
	MoneyUMap map[string]MoneyU       `json:"moneyUMap"`
	MAPtr     *MA                     `json:"maPtr"`
	MAs       []MA                    `json:"mas"`
	MAMap     map[string]MA           `json:"maMap"`
	PricePtr  *domain.Price           `json:"pricePtr"`
	Prices    []domain.Price          `json:"prices"`
	PriceMap  map[string]domain.Price `json:"priceMap"`
	MoneyT    MoneyT                  `json:"moneyT"`
	MoneyTPtr *MoneyT                 `json:"moneyTPtr"`
	MoneyTs   []MoneyT                `json:"moneyTs"`
	Amount    domain.Amount           `json:"amount"`
	EmbP      EmbP                    `json:"embP"`
	PtrEmbP   PtrEmbP                 `json:"ptrEmbP"`
	MoneyOdd  MoneyOdd                `json:"moneyOdd"`
	MD        MD                      `json:"md"`
	Pair      Pair                    `json:"pair"`
	Wrapper   Wrapper                 `json:"wrapper"`
	Stamped   Stamped                 `json:"stamped"`
	Twin      Twin                    `json:"twin"`
	Shadow    Shadow                  `json:"shadow"`
	WithInner MoneyWithInner          `json:"withInner"`
	WrapLevel WrapLevel               `json:"wrapLevel"`
	PtrEmb    PtrEmb                  `json:"ptrEmb"`
	TaggedEmb TaggedEmb               `json:"taggedEmb"`
	Deep      Deep                    `json:"deep"`
	WithID    WithID                  `json:"withID"`
	WithCode  WithCode                `json:"withCode"`
	Tag       domain.Tag              `json:"tag"`
	Raw       Raw                     `json:"raw"`
}

// RegisterRoutes registers one route per payload row.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/fields", m.getFields)
	server.POST(hr, r, "/fields", m.postFields)
	server.GET(hr, r, "/params", m.params)
	server.POST(hr, r, "/custom/:id", m.custom)
	server.GET(hr, r, "/money", m.money)
	server.GET(hr, r, "/money-ptr", m.moneyPtr)
	server.GET(hr, r, "/moneys", m.moneys)
	server.GET(hr, r, "/stamped", m.stamped)
	server.GET(hr, r, "/money-t", m.moneyT)
	server.GET(hr, r, "/wrap-level", m.wrapLevel)
	server.GET(hr, r, "/level", m.level)
	server.GET(hr, r, "/levels", m.levels)
	server.POST(hr, r, "/jose", m.jose)
	server.POST(hr, r, "/cmd", m.cmd)
	server.GET(hr, r, "/items/:id", m.items)
}

func (m *Module) getFields(ctx server.HandlerContext) (server.Result[Fields], server.IAPIError) {
	var v server.Result[Fields]
	return v, nil
}

func (m *Module) postFields(req Fields, ctx server.HandlerContext) (server.Result[Fields], server.IAPIError) {
	var v server.Result[Fields]
	return v, nil
}

func (m *Module) params(req Params, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) custom(req CustomBody, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) money(ctx server.HandlerContext) (server.Result[Money], server.IAPIError) {
	var v server.Result[Money]
	return v, nil
}

func (m *Module) moneyPtr(ctx server.HandlerContext) (server.Result[*Money], server.IAPIError) {
	var v server.Result[*Money]
	return v, nil
}

func (m *Module) moneys(ctx server.HandlerContext) (server.Result[[]Money], server.IAPIError) {
	var v server.Result[[]Money]
	return v, nil
}

func (m *Module) stamped(ctx server.HandlerContext) (server.Result[Stamped], server.IAPIError) {
	var v server.Result[Stamped]
	return v, nil
}

func (m *Module) moneyT(ctx server.HandlerContext) (server.Result[MoneyT], server.IAPIError) {
	var v server.Result[MoneyT]
	return v, nil
}

func (m *Module) wrapLevel(ctx server.HandlerContext) (server.Result[WrapLevel], server.IAPIError) {
	var v server.Result[WrapLevel]
	return v, nil
}

func (m *Module) level(ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) levels(ctx server.HandlerContext) (server.Result[[]Level], server.IAPIError) {
	var v server.Result[[]Level]
	return v, nil
}

func (m *Module) jose(req JoseReq, ctx server.HandlerContext) (server.Result[JoseResp], server.IAPIError) {
	var v server.Result[JoseResp]
	return v, nil
}

func (m *Module) cmd(req CmdBody, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}

func (m *Module) items(req ListReq, ctx server.HandlerContext) (server.Result[Level], server.IAPIError) {
	var v server.Result[Level]
	return v, nil
}
