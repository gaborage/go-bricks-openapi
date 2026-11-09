package analyzer

import "strings"

// The helpers below mirror how go-bricks joins a route group's prefix and a
// route's path (server/route_registrar.go and internal/pathutil, identical
// from v0.53.0 through v0.71.1), so a route is documented at the path the
// service actually serves rather than at the two strings concatenated.

// normalizeGroupPrefix is go-bricks' pathutil.NormalizePrefix: the canonical
// form of a Group(...) argument. "" and "/" both become "", anything else
// gains a leading slash and loses its trailing slashes. A registrar's prefix
// is the concatenation of its groups' canonical prefixes, so it is itself
// canonical: "" or "/seg..." with no trailing slash.
func normalizeGroupPrefix(prefix string) string {
	if prefix == "" || prefix == "/" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}

// joinRoutePath is go-bricks' RouteRegistrar.FullPath for a group whose
// canonical prefix is prefix. The path gains a leading slash; "" and "/" name
// the group itself; a path that repeats the group's whole prefix (equal to it,
// or followed by "/") has that prefix stripped, but a partial-word match such
// as "/apix" under "/api" does not. The result always starts with "/".
func joinRoutePath(prefix, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path == "/" {
		path = ""
	} else if prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
		path = path[len(prefix):]
	}
	if prefix == "" && path == "" {
		return "/"
	}
	return prefix + path
}
