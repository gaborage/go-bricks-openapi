// Package pagination holds the cursor meta a route of the catalog module
// documents through //openapi:meta pagination.Meta.
package pagination

// Meta is the cursor-paging meta a handler supplies.
type Meta struct {
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
}
