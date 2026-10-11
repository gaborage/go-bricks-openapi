// Package jsonstringoption pins the json ",string" option (#115).
// encoding/json writes a bool, integer, float or json.Number field tagged
// ",string" as a JSON string, through at most one pointer, and reads only the
// quoted form, so each such body field is documented as {type: string}: no
// format, no uint minimum: 0, no validate keyword, its example kept as a
// string, and its required membership and description kept. The option leaves
// every other shape alone. One struct is both the request body and the
// response, as in the issue's reproduction.
package jsonstringoption

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the json_string_option module.
type Module struct{}

func (m *Module) Name() string                    { return "json_string_option" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Cents is a named integer: the option applies to what it resolves to.
type Cents int64

// Prio is a named uint8, quoted like the uint8 it resolves to.
type Prio uint8

// PI is a named pointer type. The classic encoding/json (Go 1.26 and
// earlier) ignores ",string" on it while Go 1.27's default engine quotes it,
// so it keeps its unquoted, nullable integer schema.
type PI *int64

// Level is written and read as text through its own methods, which win over
// the option: it stays a string either way.
type Level int

// MarshalText writes the level name.
func (l Level) MarshalText() ([]byte, error) { return []byte("high"), nil }

// UnmarshalText reads the level name.
func (l *Level) UnmarshalText(b []byte) error { return nil }

// Status writes itself through MarshalJSON, which wins over the option: it
// stays {} either way.
type Status int

// MarshalJSON writes the status as an object.
func (s Status) MarshalJSON() ([]byte, error) { return []byte(`{"code":1}`), nil }

// Audit is embedded in Account: its fields, and their json options, are
// promoted.
type Audit struct {
	Rev int64 `json:"rev,string"`
}

// Account is both the request body and the response.
type Account struct {
	Audit

	// Retyped: quoted by encoding/json. An example is kept only when the Go
	// scalar can hold it, as the text encoding/json reads back.
	ID       int64          `json:"id,string" validate:"required,min=1" example:"5" doc:"account id"`
	Balance  Cents          `json:"balance,string"`
	Active   bool           `json:"active,string" example:"true"`
	Rate     float64        `json:"rate,string" validate:"gt=0" example:"2.50"`
	Limit    *int64         `json:"limit,string"`
	Hold     *Cents         `json:"hold,string"`
	Timeout  time.Duration  `json:"timeout,string" example:"5s"`
	Wait     *time.Duration `json:"wait,string"`
	Seq      uint64         `json:"seq,omitempty,string" validate:"max=99"`
	Amount   json.Number    `json:"amount,string"`
	Priority uint8          `json:"priority,string"`
	Prio     Prio           `json:"prio,string"`

	// Unchanged: the option is ignored, or would double-encode a string.
	Name   string  `json:"name,string" validate:"min=1"`
	Tags   []int64 `json:"tags,string" validate:"min=1"`
	Deep   **int64 `json:"deep,string"`
	Ref    PI      `json:"ref,string"`
	Level  Level   `json:"level,string"`
	Status Status  `json:"status,string"`
	Plain  int64   `json:"plain" validate:"min=1"`
	Extras any     `json:"extras,string"`
}

// GetAccountReq binds a path parameter: the option does not reach parameters.
type GetAccountReq struct {
	ID int64 `param:"id" json:"id,string" validate:"min=1"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/accounts", m.createAccount, server.WithTags("accounts"))
	server.GET(hr, r, "/accounts/:id", m.getAccount, server.WithTags("accounts"))
}

func (m *Module) createAccount(req Account, ctx server.HandlerContext) (server.Result[Account], server.IAPIError) {
	return server.NewResult(http.StatusCreated, req), nil
}

func (m *Module) getAccount(req GetAccountReq, ctx server.HandlerContext) (server.Result[Account], server.IAPIError) {
	return server.NewResult(http.StatusOK, Account{}), nil
}
