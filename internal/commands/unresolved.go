package commands

import (
	"fmt"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

// The Unresolved-routes slot of the shared analysis reading (see CONTEXT.md,
// "Unresolved route"): the analyzer collects the list, and BOTH commands render
// it through the two functions below rather than recomputing anything. The
// count is derived from the list — it is never stored — and neither renderer
// speaks when the list is empty, which is what keeps a clean project's output
// byte-identical to what it was before the slot existed.

// unresolvedRouteCountLine returns the `Unresolved routes: K` summary line, or
// "" when there are none. Callers print it only when it is non-empty.
func unresolvedRouteCountLine(routes []models.UnresolvedRoute) string {
	if len(routes) == 0 {
		return ""
	}
	return fmt.Sprintf("Unresolved routes: %d", len(routes))
}

// unresolvedRouteLocations renders one located line per Unresolved route —
// registration form, source location, and why it could not be resolved — for
// the command that lists them individually (doctor).
func unresolvedRouteLocations(routes []models.UnresolvedRoute) []string {
	if len(routes) == 0 {
		return nil
	}
	lines := make([]string, 0, len(routes))
	for i := range routes {
		lines = append(lines, fmt.Sprintf("%s at %s:%d:%d — %s",
			routes[i].Form, routes[i].File, routes[i].Line, routes[i].Col, routes[i].Reason))
	}
	return lines
}
