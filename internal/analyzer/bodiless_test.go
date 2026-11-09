package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Issue 144: go-bricks writes a 204 with no body whatever the payload, so a
// route whose resolved success status is 204 is bodiless: its payload is
// neither registered nor resolved, and raises no payload warning.

// bodilessSrc declares 204 routes over a struct only they return (Gone), a
// struct a 200 route also returns (Shared), a Marshaler struct, a generic
// instantiation, a third-party type and a local named non-struct.
const bodilessSrc = `package mod

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/shopspring/decimal"
)

type Module struct{}

func (m *Module) Name() string                    { return "mod" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

type IDReq struct {
	ID string ` + "`param:\"id\"`" + `
}

type Gone struct {
	ID string ` + "`json:\"id\"`" + `
}

type Shared struct {
	Name string ` + "`json:\"name\"`" + `
}

type Sealed struct {
	V string ` + "`json:\"v\"`" + `
}

func (s Sealed) MarshalJSON() ([]byte, error) { return nil, nil }

type Page[T any] struct {
	Items []T ` + "`json:\"items\"`" + `
}

type Tier func()

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.PUT(hr, r, "/gone/:id", m.gone)
	server.PUT(hr, r, "/shared/:id", m.sharedNoContent)
	server.GET(hr, r, "/shared/:id", m.sharedOK)
	server.PUT(hr, r, "/sealed/:id", m.sealed)
	server.PUT(hr, r, "/page/:id", m.page)
	server.PUT(hr, r, "/decimal/:id", m.dec)
	server.PUT(hr, r, "/tier/:id", m.tier)
	server.DELETE(hr, r, "/gone/:id", m.del)
}

func (m *Module) gone(req IDReq, ctx server.HandlerContext) (server.Result[Gone], server.IAPIError) {
	return server.NewResult(http.StatusNoContent, Gone{}), nil
}

func (m *Module) sharedNoContent(req IDReq, ctx server.HandlerContext) (server.Result[Shared], server.IAPIError) {
	return server.NewResult(204, Shared{}), nil
}

func (m *Module) sharedOK(req IDReq, ctx server.HandlerContext) (server.Result[Shared], server.IAPIError) {
	return server.NewResult(http.StatusOK, Shared{}), nil
}

func (m *Module) sealed(req IDReq, ctx server.HandlerContext) (server.Result[Sealed], server.IAPIError) {
	return server.NewResult(http.StatusNoContent, Sealed{}), nil
}

func (m *Module) page(req IDReq, ctx server.HandlerContext) (server.Result[Page[Gone]], server.IAPIError) {
	return server.NewResult(http.StatusNoContent, Page[Gone]{}), nil
}

func (m *Module) dec(req IDReq, ctx server.HandlerContext) (server.Result[decimal.Decimal], server.IAPIError) {
	return server.NewResult(http.StatusNoContent, decimal.Decimal{}), nil
}

func (m *Module) tier(req IDReq, ctx server.HandlerContext) (server.Result[Tier], server.IAPIError) {
	return server.NewResult(http.StatusNoContent, Tier(nil)), nil
}

func (m *Module) del(req IDReq, ctx server.HandlerContext) (server.NoContentResult, server.IAPIError) {
	return server.NoContent(), nil
}
`

func TestBodilessStatusPayload(t *testing.T) {
	a, routes := analyzeProjectRoutes(t, map[string]string{
		"go.mod":                          resolveGoMod,
		filepath.Join("mod", "module.go"): bodilessSrc,
	})
	require.Len(t, routes, 8)

	for _, path := range []string{"/gone/{id}", "/sealed/{id}", "/page/{id}", "/decimal/{id}", "/tier/{id}"} {
		route := routeByPath(t, routes, path)
		require.NotNil(t, route.Response, path)
		assert.True(t, route.Response.NoContent, path)
		assert.Equal(t, 204, route.SuccessStatus, path)
		assert.Nil(t, route.Response.Resolution, path)
	}

	var shared204, shared200 bool
	for i := range routes {
		if routes[i].Path != "/shared/{id}" {
			continue
		}
		require.NotNil(t, routes[i].Response)
		switch routes[i].Method {
		case "PUT":
			shared204 = routes[i].Response.NoContent
		case "GET":
			shared200 = !routes[i].Response.NoContent && routes[i].Response.Name == "Shared"
		}
	}
	assert.True(t, shared204, "the 204 route returning Shared is bodiless")
	assert.True(t, shared200, "the 200 route returning Shared keeps its body")

	registry := a.typeRegistry
	assert.NotContains(t, registry, "Gone", "a payload reached only from 204 routes is not registered")
	assert.NotContains(t, registry, "Sealed")
	assert.NotContains(t, registry, "Page")
	assert.Contains(t, registry, "Shared", "a payload a 200 route also returns keeps its component")

	assert.Empty(t, a.Warnings(t.Context()), "a bodiless payload raises no payload warning")
}
