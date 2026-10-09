// Package b declares named non-struct types used as payloads from another
// package of the module.
package b

// Cents is an int64 amount; Result[b.Cents] documents as an int64.
type Cents int64

// Tags is a string list; Result[b.Tags] documents as an array of strings.
type Tags []string
