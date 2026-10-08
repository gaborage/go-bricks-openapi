// Package domain holds structs referenced only through named types declared
// in a sibling file of the struct that uses them.
package domain

// Member is referenced through Members, PMember and MemberMap.
type Member struct {
	Name string `json:"name"`
}

// Item is referenced by Holder's fields.
type Item struct {
	SKU string `json:"sku"`
}
