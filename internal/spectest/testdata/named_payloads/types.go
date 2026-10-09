package namedpayloads

import (
	"encoding/json"
	"time"
)

// Cents is an int64 amount: a payload of it documents as an int64.
type Cents int64

// Chain is a defined type over Cents: it documents as Cents does.
type Chain Cents

// Alias is an alias of int64.
type Alias = int64

// Dur is a defined type over time.Duration, an int64 nanosecond count.
type Dur time.Duration

// Flag is a byte: an unsigned integer, and base64 only in a []Flag.
type Flag byte

// Status is a string.
type Status string

// Ratio is a float64.
type Ratio float64

// Enabled is a bool.
type Enabled bool

// Blob is a byte slice: a base64 string.
type Blob []byte

// Raw is a defined type over json.RawMessage: it drops RawMessage's methods,
// so encoding/json writes it as the []byte it is, a base64 string.
type Raw json.RawMessage

// Tags is a string list.
type Tags []string

// UserList is a list of User: an array of $ref User.
type UserList []User

// Stamp is an alias of time.Time: a date-time string.
type Stamp = time.Time

// RawA is an alias of json.RawMessage: any JSON value.
type RawA = json.RawMessage

// AnyD is a defined type over any: any JSON value.
type AnyD any

// PC is a pointer to int64: the payload root sheds it and is never nullable.
type PC *int64

// Pair is a fixed-size int array: an integer array.
type Pair [2]int

// User is reached only through payloads (UserList, *UserList).
type User struct {
	ID int64 `json:"id"`
}

// Address is reached only through a map payload.
type Address struct {
	City string `json:"city"`
}
