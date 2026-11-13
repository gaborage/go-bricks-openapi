package analyzer

import (
	"strings"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// duplicateRouteWarning names a registration dropped because an earlier one
// claimed the same route node. It deliberately makes no claim about startup:
// go-bricks v0.69+ refuses to start such a service, but older versions start
// and serve the later registration, and registrations in exclusive branches
// or build-tagged files are never both live.
const duplicateRouteWarning = "route %s %s at %s is the same route node as %s %s at %s " +
	"(path parameter names, and anything after a \"*\" wildcard, do not distinguish routes) " +
	"— documenting the first; the second is dropped from the spec"

// routeNodeKey identifies the echo router node a method and a raw go-bricks
// path land on. It is go-bricks v0.71.1's server.routeNodeKey, ported
// verbatim: a missing leading slash is added; an unescaped ':' starts a
// parameter anywhere in the path and its name runs to the next '/', so the
// name is dropped; an escaped "\:" is a literal colon and is kept verbatim;
// the first '*' outside a parameter name is the wildcard and ends the path,
// so anything after it is dropped. Two paths with the same key are one route.
//
// path must be the raw joined path (joinRoutePath's result), never a
// normalizePath'd one: "{id}" is not a parameter to this grammar.
func routeNodeKey(method, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	var b strings.Builder
	b.WriteString(method)
	b.WriteByte(' ')
	for i := 0; i < len(path); i++ {
		switch c := path[i]; {
		case c == ':' && path[i-1] != '\\': // path[0] is '/', so i >= 1 here
			b.WriteByte(':')
			for i+1 < len(path) && path[i+1] != '/' {
				i++
			}
		case c == '*':
			b.WriteByte('*')
			return b.String()
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// routeNodes is the set of route-node keys one module's walk has claimed, so
// a losing registration skips registering its payload types. It is created
// once per module and shared by every walker of that module, delegates
// included. A nil set tracks nothing (direct body walks in tests).
type routeNodes map[string]bool

// claim reports whether key is new to the set, recording it. A nil set
// claims every key.
func (s routeNodes) claim(key string) bool {
	if s == nil {
		return true
	}
	if s[key] {
		return false
	}
	s[key] = true
	return true
}

// dropDuplicateRoutes keeps the first registration of each route node across
// the whole project and drops every later one, raising one warning per loser.
// "First" is module discovery order, then each module's walk order (its
// package files in sorted path order, source order within a file, delegates
// where they are called). A route with no node key (built outside a module
// walk) is never compared.
func (a *ProjectAnalyzer) dropDuplicateRoutes(modules []models.Module) {
	firsts := map[string]*models.Route{}
	for i := range modules {
		modules[i].Routes = a.keepFirstRoutes(modules[i].Routes, firsts)
	}
}

// keepFirstRoutes filters routes against firsts (node key -> the route that
// claimed it), recording each new key and warning on each loser.
func (a *ProjectAnalyzer) keepFirstRoutes(routes []models.Route, firsts map[string]*models.Route) []models.Route {
	kept := routes[:0]
	for i := range routes {
		r := routes[i]
		if r.NodeKey == "" {
			kept = append(kept, r)
			continue
		}
		if first, dup := firsts[r.NodeKey]; dup {
			a.addWarningf(duplicateRouteWarning, r.Method, r.SourcePath, r.Site, first.Method, first.SourcePath, first.Site)
			continue
		}
		firsts[r.NodeKey] = &r
		kept = append(kept, r)
	}
	return kept
}
