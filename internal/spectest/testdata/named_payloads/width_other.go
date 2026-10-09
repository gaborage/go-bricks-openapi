//go:build !linux

package namedpayloads

// Width is int64 off linux; see width_linux.go.
type Width int64
