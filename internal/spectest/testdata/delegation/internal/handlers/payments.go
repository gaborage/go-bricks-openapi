// Package handlers hosts the delegated route registrations.
package handlers

import (
	"net/http"

	"github.com/gaborage/go-bricks/server"
)

// PaymentHandler registers and serves the payment routes.
type PaymentHandler struct{}

// Payment is the payment resource.
type Payment struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount" validate:"required,min=1"`
}

func (h *PaymentHandler) RegisterRoutes(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/payments/:id", h.getPayment, server.WithTags("payments"))
	server.POST(hr, r, "/payments", h.createPayment, server.WithTags("payments"))
}

func (h *PaymentHandler) RegisterAdmin(hr *server.HandlerRegistry, r server.RouteRegistrar) {
	server.GET(hr, r, "/payments/pending", h.listPending, server.WithTags("admin"))
}

type getPaymentRequest struct {
	ID string `param:"id" validate:"required"`
}

func (h *PaymentHandler) getPayment(req getPaymentRequest, ctx server.HandlerContext) (server.Result[Payment], server.IAPIError) {
	return server.NewResult(http.StatusOK, Payment{ID: req.ID}), nil
}

func (h *PaymentHandler) createPayment(req Payment, ctx server.HandlerContext) (server.Result[Payment], server.IAPIError) {
	return server.Created(req), nil
}

func (h *PaymentHandler) listPending(ctx server.HandlerContext) (server.Result[[]Payment], server.IAPIError) {
	return server.NewResult(http.StatusOK, []Payment{}), nil
}
