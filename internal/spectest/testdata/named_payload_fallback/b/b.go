// Package b declares a Marshaler type used as a payload from another package
// of the module.
package b

// Tier writes itself as text: Result[b.Tier] documents as {} and warns.
type Tier int

// MarshalText writes the tier name.
func (t Tier) MarshalText() ([]byte, error) { return []byte("lvl"), nil }
