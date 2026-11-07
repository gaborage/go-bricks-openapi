// Package statedschema pins the //openapi:schema directive (#117): a
// Marshaler type's Stated schema replaces {} in every body position, with no
// component and no warning.
package statedschema

import (
	"github.com/example/stated_schema/ledger"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "statedschema" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Money is written as a decimal string by its own MarshalJSON.
//
//openapi:schema string decimal
type Money struct{ Units int64 }

// MarshalJSON writes "12.50".
func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

// UnmarshalJSON reads "12.50".
func (m *Money) UnmarshalJSON(b []byte) error { return nil }

// MA is a plain alias: it uses Money's Stated schema.
type MA = Money

// Count is written as a bare integer by its own MarshalJSON.
//
//openapi:schema integer
type Count int

// MarshalJSON writes the count.
func (c Count) MarshalJSON() ([]byte, error) { return nil, nil }

// Day encodes as text both ways; the directive adds a format.
//
//openapi:schema string date
type Day struct{ Y, M, D int }

// MarshalText writes the day.
func (d Day) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the day.
func (d *Day) UnmarshalText(b []byte) error { return nil }

// Status is a plain integer: the tool sees no method on it.
type Status int

// SA is an alias that is itself a marshal-method receiver, so it takes the
// directive.
//
//openapi:schema string
type SA = Status

// MarshalJSON writes the status name.
func (SA) MarshalJSON() ([]byte, error) { return nil, nil }

// Ledger holds one field per body position.
type Ledger struct {
	Amount   Money            `json:"amount" validate:"required,min=1,max=3" example:"12.50"`
	MoneyPtr *Money           `json:"moneyPtr"`
	Amounts  []Money          `json:"amounts" validate:"min=1,dive,max=3"`
	Grid     [][]Money        `json:"grid"`
	ByName   map[string]Money `json:"byName"`
	Alias    MA               `json:"alias"`
	Count    Count            `json:"count"`
	Due      Day              `json:"due" example:"2024-02-31"`
	Start    Day              `json:"start" example:"2024-02-01"`
	Status   Status           `json:"status"`
	Named    SA               `json:"named"`
	Rate     ledger.Rate      `json:"rate"`
}

// RegisterRoutes registers one route per payload position.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/ledger", m.getLedger)
	server.GET(hr, r, "/money", m.money)
	server.GET(hr, r, "/money-meta", m.moneyMeta)
	server.GET(hr, r, "/money-ptr", m.moneyPtr)
	server.GET(hr, r, "/moneys", m.moneys)
	server.GET(hr, r, "/count", m.count)
	server.POST(hr, r, "/pay", m.pay)
}

func (m *Module) getLedger(ctx server.HandlerContext) (server.Result[Ledger], server.IAPIError) {
	var v server.Result[Ledger]
	return v, nil
}

func (m *Module) money(ctx server.HandlerContext) (server.Result[Money], server.IAPIError) {
	var v server.Result[Money]
	return v, nil
}

func (m *Module) moneyMeta(ctx server.HandlerContext) (server.ResultWithMeta[Money], server.IAPIError) {
	var v server.ResultWithMeta[Money]
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

func (m *Module) count(ctx server.HandlerContext) (server.Result[Count], server.IAPIError) {
	var v server.Result[Count]
	return v, nil
}

func (m *Module) pay(req Money, ctx server.HandlerContext) (server.Result[Money], server.IAPIError) {
	var v server.Result[Money]
	return v, nil
}
