package api

import (
	// Imported WITHOUT an alias: Go references this package as `http` (its declared
	// `package http` clause in transport/httpapi/money.go), NOT `httpapi` (the path
	// base). The analyzer must key the import under the declared name to resolve the
	// http.Money reference below into a Money component schema.
	"github.com/example/mismatch/transport/httpapi"

	"github.com/gaborage/go-bricks/server"
)

// Handler holds the wallet HTTP handlers.
type Handler struct{}

// Balance has a field of a cross-package type whose owning package is declared
// `package http` (in transport/httpapi) -> the field resolves to a $ref to the
// Money component.
type Balance struct {
	ID    int64      `json:"id"`
	Funds http.Money `json:"funds"`
}

// IDReq identifies a resource by path parameter.
type IDReq struct {
	ID int64 `param:"id" validate:"required"`
}

func (h *Handler) getBalance(req IDReq, ctx server.HandlerContext) (server.Result[Balance], server.IAPIError) {
	return server.NewResult(200, Balance{}), nil
}
