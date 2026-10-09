// Package domain declares Marshaler types another package of the module uses.
package domain

// Tag is an int written and read as text: a string both ways.
type Tag int

// MarshalText writes the tag name.
func (t Tag) MarshalText() ([]byte, error) { return []byte("t"), nil }

// UnmarshalText reads the tag name.
func (t *Tag) UnmarshalText(b []byte) error { return nil }

// Price is a struct written through its own MarshalJSON.
type Price struct {
	Cents int64 `json:"cents"`
}

// MarshalJSON writes the price.
func (p Price) MarshalJSON() ([]byte, error) { return []byte(`1`), nil }

// Amount is a struct written and read as text: a string both ways.
type Amount struct {
	Value int64 `json:"value"`
}

// MarshalText writes the amount.
func (a Amount) MarshalText() ([]byte, error) { return []byte("1 USD"), nil }

// UnmarshalText reads the amount.
func (a *Amount) UnmarshalText(b []byte) error { return nil }
