package namedresolution

import (
	"encoding/json"
	"time"

	"github.com/example/namedresolution/domain"
	"github.com/google/uuid"
)

// User and Address are structs reached through named composites.
type User struct {
	Name string `json:"name"`
}

type Address struct {
	City string `json:"city"`
}

type Cents int64
type Flag byte
type U8 = uint8

type Tags []string
type TagsA = []string
type Chain Tags
type Attrs map[string]string
type Stamps []time.Time
type Ids []Cents

type UserList []User
type PAddr *Address
type PC *int64
type FlagB bool
type AnyD any
type Shaper interface{ Area() float64 }

type Pair [2]int
type Key [4]byte
type Arr [3]byte

type Blob []byte
type BlobA = []byte
type B2 Blob
type Flags []Flag
type Raw json.RawMessage
type RawOfRaw Raw

type Stamp = time.Time
type IDA = uuid.UUID
type RawA = json.RawMessage
type Amount json.Number

// Status is a Marshaler type: here it appears only as a query parameter
// (exempt) and under the defined type StatusD (which drops the method).
type Status int

func (s Status) MarshalText() ([]byte, error) { return nil, nil }

type StatusD Status

// Odd's MarshalText has the wrong signature, so it is not a Marshaler type.
type Odd int

func (Odd) MarshalText() string { return "" }

// FlagU only decodes through its own method, so []FlagU stays base64.
type FlagU byte

func (f *FlagU) UnmarshalJSON(b []byte) error { return nil }

// TagsM is a Marshaler type, used only as a query parameter (exempt).
type TagsM []string

func (TagsM) MarshalJSON() ([]byte, error) { return nil, nil }

// This is the only file that imports domain: the named types below must
// resolve against these imports, not the field struct's file.
type Members []domain.Member
type PMember *domain.Member
type MemberMap map[string]domain.Member

// Holder is a struct declared beside the import its fields need.
type Holder struct {
	Item  domain.Item   `json:"item"`
	Items []domain.Item `json:"items"`
}
