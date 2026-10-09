//go:build linux

package namedpayloads

// Width is int32 on linux and int64 elsewhere: its declarations agree only on
// their kind, so Result[Width] documents as a bare integer (#92).
type Width int32
