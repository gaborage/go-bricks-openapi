// Package byteformat is a regression fixture for issue #79: a []byte field is a
// base64 string (`format: byte`, what encoding/json produces), and kin-openapi
// validates an `example:` against that format. Only standard, padded base64 is
// kept. CreateBlobReq covers, field by field, a padded base64 example (Payload,
// kept), an unpadded URL-safe one that kin-openapi would pass but encoding/json
// cannot decode (URLSafe, dropped), text outside the base64 alphabet (Raw,
// dropped), padding in the middle of the value on a *[]byte (Maybe, dropped),
// and the same rule on a string field whose validate:"base64" tag yields
// format: byte (Token kept, Bad dropped). ListBlobsReq's Cursor drops a
// non-base64 example from both the query parameter and its schema, which
// kin-openapi validates independently.
package byteformat

import (
	"net/http"

	"github.com/gaborage/go-bricks/app"
	"github.com/gaborage/go-bricks/server"
)

// Module is the byteformat module.
type Module struct{}

func (m *Module) Name() string                    { return "byteformat" }
func (m *Module) Init(deps *app.ModuleDeps) error { return nil }
func (m *Module) Shutdown() error                 { return nil }

// ListBlobsReq carries a base64 query parameter whose example is not base64.
type ListBlobsReq struct {
	Cursor string `query:"cursor" validate:"base64" example:"next page"`
}

// CreateBlobReq exercises every byte-format example outcome.
type CreateBlobReq struct {
	Payload []byte  `json:"payload" example:"aGVsbG8="`
	URLSafe []byte  `json:"urlSafe" example:"_-8"`
	Raw     []byte  `json:"raw" example:"not base64!"`
	Maybe   *[]byte `json:"maybe" example:"a=b"`
	Token   string  `json:"token" validate:"base64" example:"c2VjcmV0"`
	Bad     string  `json:"bad" validate:"base64" example:"hello world"`
}

// Blob is returned by both routes.
type Blob struct {
	ID   string `json:"id"`
	Data []byte `json:"data"`
}

// RegisterRoutes registers the module's HTTP routes.
func (m *Module) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/v1/blobs", m.listBlobs, server.WithTags("blobs"))
	server.POST(hr, r, "/v1/blobs", m.createBlob, server.WithTags("blobs"))
}

func (m *Module) listBlobs(req ListBlobsReq, ctx server.HandlerContext) (server.Result[[]Blob], server.IAPIError) {
	return server.NewResult(http.StatusOK, []Blob{}), nil
}

func (m *Module) createBlob(req CreateBlobReq, ctx server.HandlerContext) (server.Result[Blob], server.IAPIError) {
	return server.NewResult(http.StatusOK, Blob{}), nil
}
