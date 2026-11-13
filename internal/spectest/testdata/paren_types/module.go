// Package parentypes demonstrates type expressions written in parentheses,
// in the forms gofmt keeps. A parenthesized type is the same type, so every
// request, payload, declaration, field and query parameter here documents
// exactly as it does with the parentheses removed.
package parentypes

import (
	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

type Module struct{}

func (m *Module) Name() string                    { return "parentypes" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// CreateReq is a request written *(CreateReq): a $ref CreateReq body.
type CreateReq struct {
	Name string `json:"name"`
}

// Named is the payload type written in parentheses below.
type Named struct {
	A int `json:"a"`
}

// Base is reached through parenthesized declarations and field types.
type Base struct {
	X int `json:"x"`
}

// Cents is a named scalar, documented as its underlying int64.
type Cents int64

// PB, PBA and PS are declared with a parenthesized right-hand side and keep
// their own components; PC is an integer and PSl an array of $ref Base.
type PB (Base)

type PBA = (Base)

type PC (Cents)

type PS (struct {
	Y string `json:"y"`
})

type PSl ([]Base)

// Resp holds fields typed through those declarations, and fields whose own
// type is written in parentheses.
type Resp struct {
	B      PB        `json:"b"`
	BA     PBA       `json:"ba"`
	C      PC        `json:"c"`
	S      PS        `json:"s"`
	Sl     PSl       `json:"sl"`
	Paren  (int)     `json:"paren"`
	PParen *(string) `json:"pparen"`
	PElem  [](Base)  `json:"pelem"`
	PC
}

// Req binds query parameters whose types are written in parentheses.
type Req struct {
	Paren  (int)     `query:"paren"`
	PParen *(string) `query:"pparen"`
	Body   (int)     `json:"body"`
}

func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/create", m.create, server.WithTags("parentypes"))
	server.POST(hr, r, "/text", m.text, server.WithTags("parentypes"))
	server.POST(hr, r, "/resp", m.resp, server.WithTags("parentypes"))
	server.GET(hr, r, "/pb", m.pb, server.WithTags("parentypes"))
	server.GET(hr, r, "/paren-arg", m.parenArg, server.WithTags("parentypes"))
	server.GET(hr, r, "/ptr-paren-arg", m.ptrParenArg, server.WithTags("parentypes"))
	server.GET(hr, r, "/ptr-paren-ptr", m.ptrParenPtr, server.WithTags("parentypes"))
	server.GET(hr, r, "/paren-ptr", m.parenPtr, server.WithTags("parentypes"))
	server.GET(hr, r, "/paren-map", m.parenMap, server.WithTags("parentypes"))
	server.GET(hr, r, "/paren-meta", m.parenMeta, server.WithTags("parentypes"))
	server.GET(hr, r, "/ptr-paren-bare", m.ptrParenBare, server.WithTags("parentypes"))
	server.GET(hr, r, "/paren-slice-elem", m.parenSliceElem, server.WithTags("parentypes"))
	server.GET(hr, r, "/ptr-paren-slice", m.ptrParenSlice, server.WithTags("parentypes"))
	server.GET(hr, r, "/paren-string", m.parenString, server.WithTags("parentypes"))
	server.GET(hr, r, "/paren-any", m.parenAny, server.WithTags("parentypes"))
}

func (m *Module) create(req *(CreateReq), ctx server.HandlerContext) (server.Result[Named], server.IAPIError) { // requestBody: $ref CreateReq
	return server.Result[Named]{}, nil
}

func (m *Module) text(req *(string), ctx server.HandlerContext) (server.Result[Named], server.IAPIError) { // F26, no requestBody
	return server.Result[Named]{}, nil
}

func (m *Module) resp(req Req, ctx server.HandlerContext) (server.Result[Resp], server.IAPIError) {
	return server.Result[Resp]{}, nil
}

func (m *Module) pb(ctx server.HandlerContext) (server.Result[PB], server.IAPIError) { // data: $ref PB
	return server.Result[PB]{}, nil
}

func (m *Module) parenArg(ctx server.HandlerContext) (server.Result[(Named)], server.IAPIError) { // data: $ref Named
	return server.Result[(Named)]{}, nil
}

func (m *Module) ptrParenArg(ctx server.HandlerContext) (server.Result[*(Named)], server.IAPIError) { // data: $ref Named
	return server.Result[*(Named)]{}, nil
}

func (m *Module) ptrParenPtr(ctx server.HandlerContext) (server.Result[*(*Named)], server.IAPIError) { // data: $ref Named
	return server.Result[*(*Named)]{}, nil
}

func (m *Module) parenPtr(ctx server.HandlerContext) (server.Result[(*Named)], server.IAPIError) { // data: $ref Named
	return server.Result[(*Named)]{}, nil
}

func (m *Module) parenMap(ctx server.HandlerContext) (server.Result[(map[string]int)], server.IAPIError) { // data: map of integer
	return server.Result[(map[string]int)]{}, nil
}

func (m *Module) parenMeta(ctx server.HandlerContext) (server.ResultWithMeta[(Named)], server.IAPIError) { // data: $ref Named
	return server.ResultWithMeta[(Named)]{}, nil
}

func (m *Module) ptrParenBare(ctx server.HandlerContext) (*(Named), server.IAPIError) { // data: $ref Named
	return nil, nil
}

func (m *Module) parenSliceElem(ctx server.HandlerContext) (server.Result[[](Named)], server.IAPIError) { // data: array of $ref Named
	return server.Result[[](Named)]{}, nil
}

func (m *Module) ptrParenSlice(ctx server.HandlerContext) (server.Result[*([]Named)], server.IAPIError) { // data: array of $ref Named
	return server.Result[*([]Named)]{}, nil
}

func (m *Module) parenString(ctx server.HandlerContext) (server.Result[(string)], server.IAPIError) { // data: string
	return server.Result[(string)]{}, nil
}

func (m *Module) parenAny(ctx server.HandlerContext) (server.Result[(interface{})], server.IAPIError) { // data: {}
	return server.Result[(interface{})]{}, nil
}
