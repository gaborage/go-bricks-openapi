// Package billing declares type arguments with no component of their own (a
// named non-struct, a struct Marshaler type, an alias of time.Time) and a
// second generic Page, whose Instantiations take a package-qualified base.
package billing

import "time"

// Cents is a named non-struct type.
type Cents int64

// Money is a struct Marshaler type.
type Money struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON makes Money a Marshaler type.
func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }

// Timestamp is an alias of time.Time: as a type argument it is time.Time.
type Timestamp = time.Time

// Page is a generic struct named like the module's own Page.
type Page[T any] struct {
	Items []T `json:"items"`
}
