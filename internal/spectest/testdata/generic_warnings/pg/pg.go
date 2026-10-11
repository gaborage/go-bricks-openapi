// Package pg declares the in-module generic bases the pg.-qualified rows use:
// a struct and a slice.
package pg

// Page is an in-module generic struct.
type Page[T any] struct {
	Items []T `json:"items"`
}

// List is an in-module generic slice.
type List[T any] []T
