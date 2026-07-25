// Package literalforms exercises a struct whose tags are written as
// interpreted (double-quoted) Go string literals rather than the conventional
// raw (backtick) form. go/parser hands the tag's source text back verbatim —
// delimiters and escapes included — so a naive strings.Trim(v, "`") is a no-op
// on an interpreted literal (there are no backticks to strip) and
// reflect.StructTag.Lookup then sees a string starting with `"` and returns
// nothing for every key. The analyzer must decode the literal instead of
// trimming a delimiter, or this struct's tags — including its `jose:` key —
// are silently invisible.
package literalforms

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the literal_forms module.
type Module struct{}

func (m *Module) Name() string                    { return "literal_forms" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// CreateReq writes its struct tags as interpreted string literals. The jose
// tag on PAN is required, not decoration: it is what makes the analyzer's
// hasJOSETag detection observable through this fixture, and it covers the
// originally-reported bug end-to-end (an interpreted-tag struct losing every
// tag key, including JOSE detection).
type CreateReq struct {
	PAN   string "json:\"pan\" jose:\"decrypt=k\" validate:\"required\""
	Limit int    "json:\"limit\" validate:\"min=1,max=99\""
}

// CreateResp carries no jose tag — a plain plaintext response.
type CreateResp struct {
	ID string `json:"id"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.POST(hr, r, "/v1/creates", m.create, server.WithTags("literal_forms"))
}

func (m *Module) create(req CreateReq, ctx server.HandlerContext) (server.Result[CreateResp], server.IAPIError) {
	return server.NewResult(http.StatusOK, CreateResp{}), nil
}
