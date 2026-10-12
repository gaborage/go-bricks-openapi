package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gaborage/go-bricks-openapi/internal/models"
)

const delegateConstGoMod = "module github.com/example/app\n\ngo 1.25\n\nrequire github.com/gaborage/go-bricks v0.53.0\n"

// delegateConstModule is a module package that forwards registration to an
// alpha.Handler field. body is its RegisterRoutes body after the forwarding
// call; decls are extra package-level declarations.
func delegateConstModule(decls, body string) string {
	return `package widgets

import (
	"github.com/example/app/alpha"
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

` + decls + `

type Module struct{ handler *alpha.Handler }

func (m *Module) Name() string                 { return "widgets" }
func (m *Module) Init(d *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error              { return nil }

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	m.handler.RegisterRoutes(hr, r)
	` + body + `
}

func (m *Module) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`
}

// routePaths returns "METHOD path" for every route, in walk order.
func routePaths(routes []models.Route) []string {
	keys := make([]string, 0, len(routes))
	for i := range routes {
		keys = append(keys, routes[i].Method+" "+routes[i].Path)
	}
	return keys
}

// TestDelegateRoutePathResolvesOwnPackageConstants pins that a route a
// delegate in another in-module package registers resolves a bare-identifier
// path or group prefix against the delegate's own package: a constant in the
// same file or a sibling file, on every registration form, and through the
// same-receiver helpers, package functions and same-package field delegates
// the delegate calls in turn.
func TestDelegateRoutePathResolvesOwnPackageConstants(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":    delegateConstGoMod,
		"module.go": delegateConstModule("", ""),
		"alpha/handler.go": `package alpha

import "github.com/gaborage/go-bricks/server"

const widgetByID = "/widgets/:id"

type Handler struct{ sub *Sub }

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, widgetByID, h.get)
	server.GET(hr, r, siblingPath, h.get)
	api := r.Group(apiPrefix)
	server.GET(hr, api, widgetByID, h.get)
	r.Add("GET", rawPath, h.get)
	h.helper(hr, r)
	registerFree(hr, r)
	h.sub.RegisterRoutes(hr, r)
}

func (h *Handler) helper(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, helperPath, h.get)
}

func registerFree(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, freePath, nil)
}

func (h *Handler) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`,
		"alpha/paths.go": `package alpha

import "github.com/gaborage/go-bricks/server"

const (
	siblingPath = "/sibling"
	apiPrefix   = "/api"
	rawPath     = "/raw"
	helperPath  = "/helper"
	freePath    = "/free"
	subPath     = "/sub"
)

type Sub struct{}

func (s *Sub) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, subPath, s.get)
}

func (s *Sub) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`,
	})

	assert.ElementsMatch(t, []string{
		"GET /widgets/{id}",
		"GET /sibling",
		"GET /api/widgets/{id}",
		"GET /raw",
		"GET /helper",
		"GET /free",
		"GET /sub",
	}, routePaths(routes))
	assert.Empty(t, a.UnresolvedRoutes(t.Context()))
}

// TestDelegateConstantsDoNotLeakAcrossPackages pins that each package's
// routes read only that package's constants: a name both packages declare
// takes each one's own value, and a name only the other package declares is
// an Unresolved route on either side rather than borrowing that value.
func TestDelegateConstantsDoNotLeakAcrossPackages(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod": delegateConstGoMod,
		"module.go": delegateConstModule(
			`const (
	shared     = "/module-shared"
	moduleOnly = "/module-only"
)`,
			"server.GET(hr, r, shared, m.get)\n\tserver.GET(hr, r, alphaOnly, m.get)"),
		"alpha/handler.go": `package alpha

import "github.com/gaborage/go-bricks/server"

const (
	shared    = "/alpha-shared"
	alphaOnly = "/alpha-only"
)

type Handler struct{}

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, shared, h.get)
	server.POST(hr, r, moduleOnly, h.get)
}

func (h *Handler) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`,
	})

	assert.ElementsMatch(t, []string{"GET /alpha-shared", "GET /module-shared"}, routePaths(routes))
	unresolved := a.UnresolvedRoutes(t.Context())
	require.Len(t, unresolved, 2)
	forms := []string{unresolved[0].Form, unresolved[1].Form}
	assert.ElementsMatch(t, []string{"server.GET", "server.POST"}, forms)
}

// TestDelegateWithoutConstantsBorrowsNone pins that a delegate whose package
// declares no constants at all still reads none of the module's.
func TestDelegateWithoutConstantsBorrowsNone(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":    delegateConstGoMod,
		"module.go": delegateConstModule(`const moduleOnly = "/module-only"`, ""),
		"alpha/handler.go": `package alpha

import "github.com/gaborage/go-bricks/server"

type Handler struct{}

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, moduleOnly, h.get)
}

func (h *Handler) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`,
	})

	assert.Empty(t, routes)
	assert.Len(t, a.UnresolvedRoutes(t.Context()), 1)
}

// TestNestedDelegateResolvesItsOwnPackageConstants pins that a delegate's
// delegate in a third package reads that third package's constants, and that
// returning from it restores the caller's: alpha's route after the beta call
// still takes alpha's value of a name beta also declares.
func TestNestedDelegateResolvesItsOwnPackageConstants(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":    delegateConstGoMod,
		"module.go": delegateConstModule("", ""),
		"alpha/handler.go": `package alpha

import (
	"github.com/example/app/beta"
	"github.com/gaborage/go-bricks/server"
)

const p = "/alpha"

type Handler struct{ deep *beta.Handler }

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	h.deep.RegisterRoutes(hr, r)
	server.GET(hr, r, p, h.get)
}

func (h *Handler) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`,
		"beta/handler.go": `package beta

import "github.com/gaborage/go-bricks/server"

const p = "/beta"

type Handler struct{}

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, p, h.get)
}

func (h *Handler) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`,
	})

	assert.Equal(t, []string{"GET /beta", "GET /alpha"}, routePaths(routes))
	assert.Empty(t, a.UnresolvedRoutes(t.Context()))
}

// TestDelegateNonConstantPathStaysUnresolved pins that only constants
// resolve in a delegate too: a struct-field path (h.path) and a local
// variable shadowing a package constant are Unresolved routes.
func TestDelegateNonConstantPathStaysUnresolved(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":    delegateConstGoMod,
		"module.go": delegateConstModule("", ""),
		"alpha/handler.go": `package alpha

import "github.com/gaborage/go-bricks/server"

const p = "/alpha"

type Handler struct{ path string }

func (h *Handler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, h.path, h.get)
	p := h.path
	server.POST(hr, r, p, h.get)
}

func (h *Handler) get(ctx server.HandlerContext) (string, server.IAPIError) { return "", nil }
`,
	})

	assert.Empty(t, routes)
	assert.Len(t, a.UnresolvedRoutes(t.Context()), 2)
}
