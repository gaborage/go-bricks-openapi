package qualifiednamedtypes

import (
	"net/http"
	t "time"

	"github.com/example/qualifiednamedtypes/b"
	money "github.com/example/qualifiednamedtypes/b"
	"github.com/example/qualifiednamedtypes/kinds"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/scheduler"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "qualified" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Code and User shadow b's own Code and User.
type Code int32

type User struct {
	ID int `json:"id"`
}

type Discount b.Cents
type Cents2 = b.Cents
type StatusA = b.Status
type StatusD b.Status

// Tags is a local slice of b's Tags: no recursion.
type Tags []b.Tags

// Qualified holds one field per row.
type Qualified struct {
	b.Base

	Cents      b.Cents     `json:"cents"`
	MoneyCents money.Cents `json:"moneyCents"`
	Big        b.Big       `json:"big"`
	Flag       b.Flag      `json:"flag"`
	Code       b.Code      `json:"code"`
	LocalCode  Code        `json:"localCode"`
	Wrapped    b.Wrapped   `json:"wrapped"`
	Grade      b.Grade     `json:"grade2"`
	Discount   Discount    `json:"discount"`
	Cents2     Cents2      `json:"cents2"`

	Bounded     b.Cents `json:"bounded" validate:"min=1,max=100"`
	BoundedCode b.Code  `json:"boundedCode" validate:"min=2,max=8"`

	PCents      *b.Cents             `json:"pCents"`
	CentsList   []b.Cents            `json:"centsList"`
	CentsGrid   [][]b.Cents          `json:"centsGrid"`
	CentsMap    map[string]b.Cents   `json:"centsMap"`
	CentsMapL   map[string][]b.Cents `json:"centsMapL"`
	CentsMapP   map[string]*b.Cents  `json:"centsMapP"`
	PCentsMap   *map[string]b.Cents  `json:"pCentsMap"`
	CentsMapArr []map[string]b.Cents `json:"centsMapArr"`

	BTags     b.Tags            `json:"bTags"`
	PBTags    *b.Tags           `json:"pBTags"`
	BTagsList []b.Tags          `json:"bTagsList"`
	BTagsMap  map[string]b.Tags `json:"bTagsMap"`
	MoneyTags money.Tags        `json:"moneyTags"`
	Labels    kinds.Labels      `json:"labels"`
	Codes     b.Codes           `json:"codes"`
	LocalTags Tags              `json:"localTags"`

	UserList  b.UserList `json:"userList"`
	BUser     b.User     `json:"bUser"`
	LocalUser User       `json:"localUser"`
	Items     b.Items    `json:"items"`

	Stamp b.Stamp `json:"stamp"`
	Blob  b.Blob  `json:"blob"`
	Tree  b.Tree  `json:"tree"`
	Addr  b.Addr  `json:"addr"`

	Status  b.Status   `json:"status"`
	PStatus *b.PStatus `json:"pStatus"`
	StatusA StatusA    `json:"statusA"`
	StatusD StatusD    `json:"statusD"`

	Width b.Width `json:"width"`

	Schedule    scheduler.ScheduleType `json:"schedule"`
	Decimal     decimal.Decimal        `json:"decimal"`
	AliasedTime t.Time                 `json:"aliasedTime"`
}

// QualifiedParams binds qualified named types as parameters.
type QualifiedParams struct {
	Code b.Code    `param:"code"`
	Min  b.Cents   `query:"min"`
	Many []b.Cents `query:"many"`
	Tags b.Tags    `query:"tags"`
	St   b.Status  `query:"st"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/qualified/:code", m.get)
	server.GET(hr, r, "/cents", m.cents)
}

func (m *Module) get(req QualifiedParams, ctx server.HandlerContext) (server.Result[Qualified], server.IAPIError) {
	return server.NewResult(http.StatusOK, Qualified{}), nil
}

// cents keeps today's payload fallback: payloads are resolved by #110.
func (m *Module) cents(ctx server.HandlerContext) (server.Result[b.Cents], server.IAPIError) {
	return server.NewResult(http.StatusOK, b.Cents(0)), nil
}
