// Package http is declared in the transport/httpapi directory: its `package`
// clause name (http) deliberately differs from the import path's last segment
// (httpapi). A handler in the api package references this type as http.Money, so
// fileImports must key the unaliased import under the DECLARED name (http), not the
// path base (httpapi), or the cross-package type is never resolved to a component.
package http

// Money is referenced cross-package as http.Money despite living under
// transport/httpapi.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}
