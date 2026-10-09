// Package c is imported by b only; the route package never imports it.
package c

type Level int32

type Item struct {
	ID int `json:"id"`
}
