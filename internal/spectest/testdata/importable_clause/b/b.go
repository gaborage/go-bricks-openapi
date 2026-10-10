// Package b is imported unaliased (and once aliased) by the module. A
// //go:build ignore generator in package main sorts first in this directory.
package b

// Addr is a struct: a component.
type Addr struct {
	Street string `json:"street"`
}

// Base is embedded by value: its fields are promoted.
type Base struct {
	Created string `json:"created"`
}

// Cents is a named scalar.
type Cents int64

// Tags is a named slice.
type Tags []string

// Level is written and read as text: a string.
type Level int

// MarshalText writes the level.
func (l Level) MarshalText() ([]byte, error) { return []byte("high"), nil }

// UnmarshalText reads the level.
func (l *Level) UnmarshalText(b []byte) error { return nil }

// Money is written through its own MarshalJSON: untyped.
type Money struct {
	Amount int64 `json:"amount"`
}

// MarshalJSON writes the money.
func (m Money) MarshalJSON() ([]byte, error) { return nil, nil }
