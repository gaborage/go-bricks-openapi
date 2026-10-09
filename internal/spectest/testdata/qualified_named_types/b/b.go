// Package b declares the named non-struct types the route package uses
// qualified (b.Cents) and through an import alias (money.Cents).
package b

import (
	"time"

	"github.com/example/qualifiednamedtypes/c"
)

type Cents int64
type Big uint64
type Flag bool

// Code is a string here; the route package declares its own Code int32.
type Code string
type Codes []Code

type Wrapped Cents
type Grade c.Level

// Base is embedded by the route package: its fields are promoted and resolve
// in b.
type Base struct {
	Grade Grade   `json:"grade"`
	Level c.Level `json:"level"`
}

type Tags []string

// User shares its name with the route package's own User struct.
type User struct {
	Name string `json:"name"`
}

type UserList []User
type Items []c.Item

type Stamp = time.Time
type Blob []byte
type Tree map[string]Tree
type Addr uintptr

// Status is a Marshaler type: its MarshalText is in status_text.go.
type Status int

// PStatus decodes through a method on its pointer.
type PStatus int

func (p *PStatus) UnmarshalJSON(b []byte) error { return nil }
