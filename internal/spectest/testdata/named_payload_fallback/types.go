package namedpayloadfallback

import "time"

// Tier writes itself as text through a value MarshalText (and has no
// UnmarshalText, so #111's string rule leaves it {}).
type Tier int

// MarshalText writes the tier name.
func (t Tier) MarshalText() ([]byte, error) { return []byte("lvl"), nil }

// PTier writes itself as text through a pointer MarshalText.
type PTier int

// MarshalText writes the tier name.
func (t *PTier) MarshalText() ([]byte, error) { return []byte("plvl"), nil }

// FlagV is a byte with its own MarshalJSON.
type FlagV byte

// MarshalJSON writes a fixed string.
func (f FlagV) MarshalJSON() ([]byte, error) { return []byte(`"v"`), nil }

// Tree contains itself: the recursion is cut to {}.
type Tree map[string]Tree

// Addr is a uintptr: a machine address with no API contract.
type Addr uintptr

// StampD is a defined type over time.Time: it drops time.Time's methods, so
// it resolves to no schema.
type StampD time.Time

// Cx is a complex128, which has no JSON schema.
type Cx complex128
