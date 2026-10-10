//go:build ignore

// A generator program that sorts first in b. Its package main clause neither
// names b's import nor contributes a declaration to b: its Addr is not b's.
package main

// Addr is the generator's own type.
type Addr struct {
	Wrong int `json:"wrong"`
}

func main() {}
