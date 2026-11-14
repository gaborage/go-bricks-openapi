// Package responseheaders pins issue 71: the header names a handler writes
// to its result's Headers field with a string-literal key, in its own body
// and outside function literals, are documented on the success response as
// `headers: {<Name>: {schema: {type: string}}}`, canonicalized and with
// Content-Type dropped. Every other route documents no headers key.
package responseheaders

import (
	"net/http"
	nethttp "net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
	"github.com/labstack/echo/v4"
)

// Module is the response_headers module.
type Module struct{}

func (m *Module) Name() string                    { return "response_headers" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// Item is the payload.
type Item struct {
	ID string `json:"id"`
}

// ItemReq is the request body.
type ItemReq struct {
	Name string `json:"name"`
}

// headerLocation is a package-level constant key: not a string literal at
// the write, so not documented.
const headerLocation = "Location"

// RegisterRoutes registers one route per write shape and per negative.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/literal", m.literal)
	server.POST(hr, r, "/set", m.set)
	server.POST(hr, r, "/add", m.add)
	server.POST(hr, r, "/index", m.index)
	server.POST(hr, r, "/map-literal", m.mapLiteral)
	server.POST(hr, r, "/aliased-net-http", m.aliasedNetHTTP)
	server.POST(hr, r, "/bound-result", m.boundResult)
	server.POST(hr, r, "/bound-meta", m.boundMeta)
	server.POST(hr, r, "/lowercase", m.lowercase)
	server.GET(hr, r, "/conditional", m.conditional)
	server.POST(hr, r, "/returned-result", m.returnedResult)
	server.POST(hr, r, "/returned-meta", m.returnedMeta)
	server.POST(hr, r, "/named-bare", m.namedBare)
	server.POST(hr, r, "/pointer", m.pointer)
	server.POST(hr, r, "/raw", m.raw, server.WithRawResponse())
	server.GET(hr, r, "/none", m.none)
	server.POST(hr, r, "/constant-keys", m.constantKeys)
	server.POST(hr, r, "/content-type", m.contentType)
	server.POST(hr, r, "/closure", m.closure)
	server.POST(hr, r, "/deferred", m.deferred)
	server.DELETE(hr, r, "/no-content", m.noContent)
	server.DELETE(hr, r, "/no-content-result", m.noContentResult)
	server.POST(hr, r, "/local-map", m.localMap)
	server.POST(hr, r, "/error-only", m.errorOnly)
	server.POST(hr, r, "/inner-shadow", m.innerShadow)
	server.POST(hr, r, "/before-error", m.beforeError)
	server.POST(hr, r, "/named-bare-error", m.namedBareError)
	server.POST(hr, r, "/nil-before-error", m.nilBeforeError)
	server.POST(hr, r, "/sibling-case", m.siblingCase)
	server.POST(hr, r, "/init-write", m.initWrite)
	server.POST(hr, r, "/map-split-paths", m.mapSplitPaths)
	server.POST(hr, r, "/fallthrough", m.fallthroughCase)
	server.POST(hr, r, "/map-exclusive-arms", m.mapExclusiveArms)
}

// fallthroughCase sets Location in a clause that falls through to one that
// returns the result -> Location on 200.
func (m *Module) fallthroughCase(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Result[Item]{Data: Item{ID: req.Name}, Headers: http.Header{}}
	switch req.Name {
	case "a":
		res.Headers.Set("Location", "/items/1")
		fallthrough
	case "b":
		return res, nil
	}
	return res, server.NewNotFoundError("item")
}

// mapExclusiveArms gives h to the result in one arm and writes h in the
// other, so the response never holds X-Map -> no headers key.
func (m *Module) mapExclusiveArms(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	h := http.Header{}
	res := server.Result[Item]{Data: Item{ID: req.Name}}
	if req.Name == "" {
		res.Headers = h
	} else {
		h.Set("X-Map", "1")
	}
	return res, nil
}

// initWrite sets Location in an if initializer, which runs before the arm
// that returns the result -> Location on 200.
func (m *Module) initWrite(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Result[Item]{Data: Item{ID: req.Name}}
	if res.Headers = (http.Header{"Location": {"/items/1"}}); req.Name != "" {
		return res, nil
	}
	return server.Result[Item]{}, nil
}

// mapSplitPaths gives h to the result on one path and writes h on another,
// so no response carries X-Map -> no headers key.
func (m *Module) mapSplitPaths(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	h := http.Header{}
	res := server.Result[Item]{Data: Item{ID: req.Name}}
	if req.Name == "" {
		res.Headers = h
		return res, nil
	}
	h.Set("X-Map", "1")
	return res, nil
}

// siblingCase sets Retry-After in one switch arm and returns with a nil
// error from another; the arm that writes falls through to the error return,
// so the header never reaches a success response -> no headers key.
func (m *Module) siblingCase(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Result[Item]{Headers: http.Header{}}
	switch req.Name {
	case "bad":
		res.Headers.Set("Retry-After", "30")
	case "ok":
		return res, nil
	}
	return res, server.NewNotFoundError("item")
}

// nilBeforeError sets Location, then returns the result with a nil error on
// one path before the block's terminal error return -> Location on 201.
func (m *Module) nilBeforeError(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"Location": {"/items/1"}}
	if req.Name != "" {
		return res, nil
	}
	return res, server.NewNotFoundError("item")
}

// beforeError writes Retry-After only in a block that ends in an error
// return, which go-bricks answers with the error -> no headers key.
func (m *Module) beforeError(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	if req.Name == "" {
		res.Headers = http.Header{"Retry-After": {"30"}}
		return res, server.NewServiceUnavailableError("down")
	}
	return res, nil
}

// namedBareError assigns its named error result a non-nil value, so a bare
// return does not count its named result -> no headers key.
func (m *Module) namedBareError(req ItemReq, ctx server.HandlerContext) (res server.Result[Item], apiErr server.IAPIError) {
	res = server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"Location": {"/items/1"}}
	if req.Name == "" {
		apiErr = server.NewNotFoundError("item")
	}
	return
}

// localMap builds the header map in a local and hands it to the returned
// literal -> Location on 200.
func (m *Module) localMap(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	h := http.Header{}
	h.Set("Location", "/items/1")
	return server.Result[Item]{Data: Item{ID: req.Name}, Headers: h}, nil
}

// errorOnly writes a header only on a result returned with an error, which
// go-bricks drops -> no headers key.
func (m *Module) errorOnly(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	if req.Name == "" {
		bad := server.Result[Item]{}
		bad.Headers = http.Header{"X-Error-Only": {"1"}}
		return bad, server.NewNotFoundError("item")
	}
	return server.Created(Item{ID: req.Name}), nil
}

// innerShadow writes a header on an inner-block variable that only shares
// the returned one's name -> no headers key. The outer result is a plain
// 200 literal so the row isolates header scoping from success-status
// inference.
func (m *Module) innerShadow(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Result[Item]{Data: Item{ID: req.Name}}
	if req.Name != "" {
		res := server.Result[Item]{}
		res.Headers = http.Header{"X-Shadowed": {"1"}}
		_ = res
	}
	return res, nil
}

// literal assigns a composite literal -> Location on 201.
func (m *Module) literal(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"Location": []string{"/items/1"}}
	return res, nil
}

// set allocates the map then calls Set -> Location on 201.
func (m *Module) set(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = make(http.Header)
	res.Headers.Set("Location", "/items/1")
	return res, nil
}

// add calls Add -> Retry-After on 202.
func (m *Module) add(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Accepted(Item{ID: req.Name})
	res.Headers = http.Header{}
	res.Headers.Add("Retry-After", "30")
	return res, nil
}

// index assigns through an index expression -> Location on 201.
func (m *Module) index(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{}
	res.Headers["Location"] = []string{"/items/1"}
	return res, nil
}

// mapLiteral assigns a map[string][]string literal -> Location on 201.
func (m *Module) mapLiteral(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = map[string][]string{"Location": {"/items/1"}}
	return res, nil
}

// aliasedNetHTTP assigns a literal of net/http imported as nethttp -> Location on 201.
func (m *Module) aliasedNetHTTP(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = nethttp.Header{"Location": {"/items/1"}}
	return res, nil
}

// boundResult binds a Result literal with a Headers element -> Location on 201.
func (m *Module) boundResult(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Result[Item]{Status: http.StatusCreated, Data: Item{ID: req.Name}, Headers: http.Header{"Location": {"/items/1"}}}
	return res, nil
}

// boundMeta binds a ResultWithMeta literal with a Headers element -> Location on 201.
func (m *Module) boundMeta(req ItemReq, ctx server.HandlerContext) (server.ResultWithMeta[Item], server.IAPIError) {
	res := server.ResultWithMeta[Item]{Status: http.StatusCreated, Data: Item{ID: req.Name}, Headers: http.Header{"Location": {"/items/1"}}}
	return res, nil
}

// lowercase writes "location", which the framework sends as Location -> Location on 201.
func (m *Module) lowercase(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"location": {"/items/1"}}
	return res, nil
}

// conditional sets ETag only in a branch -> Etag on 200, with no required key.
func (m *Module) conditional(ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Result[Item]{Data: Item{}}
	res.Headers = http.Header{}
	if ctx.Echo != nil {
		res.Headers.Set("ETag", `"v1"`)
	}
	return res, nil
}

// returnedResult returns a Result literal directly -> Location on 201.
func (m *Module) returnedResult(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.Result[Item]{Status: http.StatusCreated, Data: Item{ID: req.Name}, Headers: http.Header{"Location": {"/items/1"}}}, nil
}

// returnedMeta returns a ResultWithMeta literal directly -> Location on 202.
func (m *Module) returnedMeta(req ItemReq, ctx server.HandlerContext) (server.ResultWithMeta[Item], server.IAPIError) {
	return server.ResultWithMeta[Item]{Status: http.StatusAccepted, Data: Item{ID: req.Name}, Headers: http.Header{"Location": {"/items/1"}}}, nil
}

// namedBare writes its named result and returns bare -> Location on 201.
func (m *Module) namedBare(req ItemReq, ctx server.HandlerContext) (res server.Result[Item], apiErr server.IAPIError) {
	res = server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"Location": {"/items/1"}}
	return
}

// pointer returns &res from a *server.Result[T] handler -> Location on 201.
func (m *Module) pointer(req ItemReq, ctx server.HandlerContext) (*server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"Location": {"/items/1"}}
	return &res, nil
}

// raw is a WithRawResponse route -> Location on 201 beside the bare payload.
func (m *Module) raw(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"Location": {"/items/1"}}
	return res, nil
}

// none writes no header -> no headers key.
func (m *Module) none(ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	return server.Result[Item]{Data: Item{}}, nil
}

// constantKeys keys by echo.HeaderLocation and a package const -> no headers key.
func (m *Module) constantKeys(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{echo.HeaderLocation: {"/items/1"}}
	res.Headers.Set(headerLocation, "/items/1")
	return res, nil
}

// contentType writes Content-Type, which OpenAPI 3.0 ignores -> no headers key.
func (m *Module) contentType(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = http.Header{"Content-Type": {"application/vnd.item+json"}}
	return res, nil
}

// closure writes only inside a called function literal -> no headers key.
func (m *Module) closure(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	f := func() { res.Headers = http.Header{"Location": {"/items/1"}} }
	f()
	return res, nil
}

// deferred sets on an allocated map only in a deferred function literal -> no headers key.
func (m *Module) deferred(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.Created(Item{ID: req.Name})
	res.Headers = make(http.Header)
	defer func() { res.Headers.Set("Location", "/items/1") }()
	return res, nil
}

// noContent sets a header on a NewResult 204 -> Location on the bodiless 204.
func (m *Module) noContent(req ItemReq, ctx server.HandlerContext) (server.Result[Item], server.IAPIError) {
	res := server.NewResult(http.StatusNoContent, Item{})
	res.Headers = http.Header{"Location": {"/items"}}
	return res, nil
}

// noContentResult returns server.NoContent() -> no headers key.
func (m *Module) noContentResult(req ItemReq, ctx server.HandlerContext) (server.NoContentResult, server.IAPIError) {
	return server.NoContent(), nil
}
