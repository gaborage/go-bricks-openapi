package marshalertypes

import (
	"encoding/json"
	"encoding/json/jsontext"
	"time"

	"github.com/google/uuid"
)

// Level is written through a value MarshalText and read through a pointer
// UnmarshalText: a string both ways.
type Level int

// MarshalText writes the level name.
func (l Level) MarshalText() ([]byte, error) { return []byte("high"), nil }

// UnmarshalText reads the level name.
func (l *Level) UnmarshalText(b []byte) error { return nil }

// LA is an alias of Level: it keeps Level's methods.
type LA = Level

// LD is a defined type over Level: it drops Level's methods.
type LD Level

// LevelP has the text pair on its pointer only: encoding/json skips its
// MarshalText on a non-addressable value (a map value, a Result[T] payload),
// so it is not a string both ways.
type LevelP int

// MarshalText writes the level name.
func (l *LevelP) MarshalText() ([]byte, error) { return []byte("p"), nil }

// UnmarshalText reads the level name.
func (l *LevelP) UnmarshalText(b []byte) error { return nil }

// StatusV only encodes as text: it decodes from a number.
type StatusV int

// MarshalText writes the status name.
func (s StatusV) MarshalText() ([]byte, error) { return []byte("active"), nil }

// LevelU only decodes as text.
type LevelU int

// UnmarshalText reads the level name.
func (l *LevelU) UnmarshalText(b []byte) error { return nil }

// Both has the text pair and a MarshalJSON, which wins on encode.
type Both int

// MarshalText writes the name.
func (b Both) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the name.
func (b *Both) UnmarshalText(p []byte) error { return nil }

// MarshalJSON wins over MarshalText.
func (b Both) MarshalJSON() ([]byte, error) { return nil, nil }

// TextJ has the text pair and an UnmarshalJSON, which wins on decode.
type TextJ int

// MarshalText writes the name.
func (x TextJ) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the name.
func (x *TextJ) UnmarshalText(p []byte) error { return nil }

// UnmarshalJSON wins over UnmarshalText.
func (x *TextJ) UnmarshalJSON(p []byte) error { return nil }

// Code is a string kind with the text pair.
type Code string

// MarshalText writes the code.
func (c Code) MarshalText() ([]byte, error) { return []byte(c), nil }

// UnmarshalText reads the code.
func (c *Code) UnmarshalText(b []byte) error { return nil }

// FlagT2 is a byte kind with the text pair: never base64.
type FlagT2 byte

// MarshalText writes the flag.
func (f FlagT2) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the flag.
func (f *FlagT2) UnmarshalText(b []byte) error { return nil }

// TextApp has MarshalText, AppendText and UnmarshalText: a string.
type TextApp int

// MarshalText writes the value.
func (x TextApp) MarshalText() ([]byte, error) { return nil, nil }

// AppendText appends the value.
func (x TextApp) AppendText(b []byte) ([]byte, error) { return b, nil }

// UnmarshalText reads the value.
func (x *TextApp) UnmarshalText(b []byte) error { return nil }

// AppOnly has AppendText and UnmarshalText but no MarshalText.
type AppOnly int

// AppendText appends the value.
func (x AppOnly) AppendText(b []byte) ([]byte, error) { return b, nil }

// UnmarshalText reads the value.
func (x *AppOnly) UnmarshalText(b []byte) error { return nil }

// ToText has MarshalJSONTo beside the text pair: the JSON method wins.
type ToText int

// MarshalJSONTo writes the value.
func (x ToText) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }

// MarshalText writes the value.
func (x ToText) MarshalText() ([]byte, error) { return nil, nil }

// UnmarshalText reads the value.
func (x *ToText) UnmarshalText(b []byte) error { return nil }

// FromOnly has only UnmarshalJSONFrom.
type FromOnly int

// UnmarshalJSONFrom reads the value.
func (x *FromOnly) UnmarshalJSONFrom(dec *jsontext.Decoder) error { return nil }

// Money is a struct written through a value MarshalJSON.
type Money struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON writes the money.
func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

// MoneyP is a struct written through a pointer MarshalJSON.
type MoneyP struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON writes the money.
func (m *MoneyP) MarshalJSON() ([]byte, error) { return nil, nil }

// MoneyU is a struct read through its own UnmarshalJSON only.
type MoneyU struct {
	Amount int64 `json:"amount"`
}

// UnmarshalJSON reads the money.
func (m *MoneyU) UnmarshalJSON(b []byte) error { return nil }

// MA is an alias of Money: it keeps Money's methods.
type MA = Money

// MD is a defined type over Money: it drops Money's declared methods.
type MD Money

// MoneyOdd has a MarshalJSON with the wrong signature: not a Marshaler type.
type MoneyOdd struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON returns a string, so encoding/json ignores it.
func (MoneyOdd) MarshalJSON() string { return "" }

// MoneyT is a struct written and read as text: a string both ways.
type MoneyT struct {
	Amount int64 `json:"amount"`
}

// MarshalText writes the money.
func (m MoneyT) MarshalText() ([]byte, error) { return []byte("1 USD"), nil }

// UnmarshalText reads the money.
func (m *MoneyT) UnmarshalText(b []byte) error { return nil }

// EmbP embeds LevelP by value, so LevelP's pointer-only MarshalText stays
// out of its value method set: not a string both ways.
type EmbP struct {
	LevelP
	N int `json:"n"`
}

// PtrEmbP embeds *LevelP, which puts the text pair in its value method set:
// a string both ways.
type PtrEmbP struct {
	*LevelP
	N int `json:"n"`
}

// Pair's embedded Level and Code cancel every method: an object both ways.
type Pair struct {
	Level
	Code
}

// Wrapper promotes StatusV's MarshalText only.
type Wrapper struct {
	StatusV
	Name string `json:"name"`
}

// Stamped promotes time.Time's four methods.
type Stamped struct {
	time.Time
	Note string `json:"note"`
}

// Twin's two MarshalText methods cancel; Level's UnmarshalText is promoted.
type Twin struct {
	Level
	StatusV
}

// Shadow's own MarshalJSON outranks the text pair promoted from Level.
type Shadow struct {
	Level
	Note string `json:"note"`
}

// MarshalJSON writes the shadow.
func (s Shadow) MarshalJSON() ([]byte, error) { return nil, nil }

// OnlyInner is reached only through MoneyWithInner.
type OnlyInner struct {
	In string `json:"in"`
}

// MoneyWithInner is written through its own MarshalJSON.
type MoneyWithInner struct {
	Inner OnlyInner `json:"inner"`
}

// MarshalJSON writes the money.
func (m MoneyWithInner) MarshalJSON() ([]byte, error) { return nil, nil }

// WrapLevel promotes Level's text pair: a string both ways.
type WrapLevel struct {
	Level
	Note string `json:"note"`
}

// PtrEmb promotes the text pair through an embedded pointer.
type PtrEmb struct {
	*Level
	Note string `json:"note"`
}

// TaggedEmb promotes the text pair through a json-tagged embed.
type TaggedEmb struct {
	Level `json:"lvl"`
	Note  string `json:"note"`
}

// Deep promotes the text pair two levels down.
type Deep struct {
	WrapLevel
	X int `json:"x"`
}

// WithID promotes uuid.UUID's text pair.
type WithID struct {
	uuid.UUID
	Note string `json:"note"`
}

// CustomBody is a request read through its own UnmarshalJSON; its id is
// still a path parameter.
type CustomBody struct {
	ID   string `param:"id"`
	Name string `json:"name"`
}

// UnmarshalJSON reads the body.
func (c *CustomBody) UnmarshalJSON(b []byte) error { return nil }

// Raw promotes json.RawMessage's JSON pair.
type Raw struct {
	json.RawMessage
	N int `json:"n"`
}

// Params types a Level query and header parameter by kind.
type Params struct {
	Level  Level `query:"level"`
	HLevel Level `header:"X-Level"`
}

// JoseReq is a JOSE request whose plaintext is read through its own
// UnmarshalJSON: the route stays application/jose, naming no component.
type JoseReq struct {
	_    struct{} `jose:"decrypt=k1,verify=k2"`
	Name string   `json:"name"`
}

// UnmarshalJSON reads the plaintext.
func (j *JoseReq) UnmarshalJSON(b []byte) error { return nil }

// JoseResp is a JOSE response whose plaintext is written through its own
// MarshalJSON.
type JoseResp struct {
	_ struct{} `jose:"sign=k1,encrypt=k2"`
	A int64    `json:"a"`
}

// MarshalJSON writes the plaintext.
func (j JoseResp) MarshalJSON() ([]byte, error) { return nil, nil }

// Filter is params-only: reached only as a parameter of CmdBody.
type Filter struct {
	A string `query:"a"`
}

// Empty has no serializable property: reached only as a parameter.
type Empty struct{}

// CmdBody is read through its own UnmarshalJSON; its struct-typed
// parameters keep their components.
type CmdBody struct {
	F    Filter `query:"f"`
	E    Empty  `query:"e"`
	Name string `json:"name"`
}

// UnmarshalJSON reads the body.
func (c *CmdBody) UnmarshalJSON(b []byte) error { return nil }

// Defaults promotes a pointer UnmarshalJSON.
type Defaults struct{}

// UnmarshalJSON reads the defaults.
func (d *Defaults) UnmarshalJSON(b []byte) error { return nil }

// ListReq is params-only: no body is documented, although it promotes
// Defaults' UnmarshalJSON.
type ListReq struct {
	Defaults
	Page int    `query:"page"`
	ID   string `param:"id"`
}
