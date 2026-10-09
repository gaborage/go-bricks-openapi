package namedfallback

import (
	j "encoding/json"
	"encoding/json/jsontext"
	"time"
	t "time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Status is a Marshaler type: encoding/json writes it through MarshalText.
type Status int

func (s Status) MarshalText() ([]byte, error) { return nil, nil }

// StatusA aliases Status, methods included.
type StatusA = Status

// StatusP decodes through its own method on the pointer receiver.
type StatusP int

func (s *StatusP) UnmarshalJSON(b []byte) error { return nil }

// TagsM is a Marshaler slice.
type TagsM []string

func (TagsM) MarshalJSON() ([]byte, error) { return nil, nil }

// FlagV is a byte-sized Marshaler type: a slice of it is not base64.
type FlagV byte

func (f *FlagV) MarshalJSON() ([]byte, error) { return nil, nil }

// Wire and Appended use the encoding/json/v2 and encoding.TextAppender methods.
type Wire int

func (Wire) MarshalJSONTo(enc *jsontext.Encoder) error { return nil }

type Appended int

func (Appended) AppendText(b []byte) ([]byte, error) { return b, nil }

// User is reachable only under the Marshaler type Users, so no User component
// is emitted.
type User struct {
	Name string `json:"name"`
}

type Users []User

func (Users) MarshalJSON() ([]byte, error) { return nil, nil }

// Recursive named types are cut where they recur.
type Tree map[string]Tree
type List []List
type Left []Right
type Right map[string]Left

// Defined types over well-known structs, an untyped builtin, and a uintptr.
type StampD time.Time
type IDD uuid.UUID
type C complex128
type Addr uintptr

// Fallback holds one field per warned row. Each comment is the schema the
// field emits; every field raises one analyzer warning.
type Fallback struct {
	Status      Status             `json:"status"`                                   // -> {}
	StatusA     StatusA            `json:"statusA"`                                  // -> {}
	StatusP     StatusP            `json:"statusP"`                                  // -> {}
	StatusPtr   *Status            `json:"statusPtr"`                                // -> {} (no type, so no nullable)
	StatusBound Status             `json:"statusBound" validate:"min=1,max=3"`       // -> {}
	StatusEnum  Status             `json:"statusEnum" validate:"oneof=1 2"`          // -> {}
	StatusEx    Status             `json:"statusEx" example:"active"`                // -> {example: active}
	Statuses    []Status           `json:"statuses" validate:"min=1,dive,oneof=1 2"` // -> {array, items: {}, minItems 1}
	TagsM       TagsM              `json:"tagsM"`                                    // -> {}
	FlagVs      []FlagV            `json:"flagVs"`                                   // -> {array, items: {}}
	Wire        Wire               `json:"wire"`                                     // -> {}
	Appended    Appended           `json:"appended"`                                 // -> {}
	Users       Users              `json:"users"`                                    // -> {}
	Tree        Tree               `json:"tree"`                                     // -> {object, additionalProperties: {}}
	List        List               `json:"list"`                                     // -> {array, items: {}}
	Left        Left               `json:"left"`                                     // -> {array, items: {object, additionalProperties: {}}}
	Decimal     decimal.Decimal    `json:"decimal"`                                  // -> {object}
	AliasedTime t.Time             `json:"aliasedTime"`                              // -> {object}
	AliasedRaw  j.RawMessage       `json:"aliasedRaw"`                               // -> {object}
	StampD      StampD             `json:"stampD"`                                   // -> {object}
	IDD         IDD                `json:"idd"`                                      // -> {object}
	Err         error              `json:"err"`                                      // -> {object}
	CW          C                  `json:"cw"`                                       // -> {object}
	Addr        Addr               `json:"addr"`                                     // -> {object}
	AddrMap     map[string]uintptr `json:"addrMap"`                                  // -> {object, additionalProperties: {object}}
}
