// Package ledger declares a Marshaler type inside a type group, carrying its
// own //openapi:schema directive.
package ledger

type (
	// Rate is a basis-point rate written as a decimal number.
	//
	//openapi:schema number double
	Rate struct{ BP int }

	// Other is a plain scalar beside it.
	Other int
)

// MarshalJSON writes the rate as a number.
func (r Rate) MarshalJSON() ([]byte, error) { return nil, nil }
