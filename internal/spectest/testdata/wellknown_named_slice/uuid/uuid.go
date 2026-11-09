// Package uuid is an in-module package whose UUID is a project struct, not
// the well-known google/uuid type it is spelled like.
package uuid

// UUID is a project struct that shares its qualified name with uuid.UUID.
type UUID struct {
	V string `json:"v"`
}
