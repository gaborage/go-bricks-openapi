// Package b declares the in-module types the module embeds qualified.
package b

// Cents is a named scalar.
type Cents int64

// Tags is a named slice.
type Tags []string

// Base is a struct: embedded untagged, its fields are promoted.
type Base struct {
	ID int64 `json:"id"`
}

// Base2 is an alias of Base declared in b: it promotes Base's fields too.
type Base2 = Base
