package http

import (
	"drewisy/internal/domain"
	"net/http"

	"github.com/labstack/echo/v4"
)

type OrderHandler struct {
	usecase domain.OrderUsecase
}

func NewOrderHandler(e *echo.Group, u domain.OrderUsecase) {
	handler := &OrderHandler{usecase: u}
	// Sadece customer (müşteri) rolü sipariş verebilir
	e.POST("/orders", handler.Create, AuthMiddleware(), RBACMiddleware("customer"))
}

func (h *OrderHandler) Create(c echo.Context) error {
	var req domain.CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "geçersiz istek formatı")
	}

	customerID := c.Get("user_id").(string) // JWT Context'ten geliyor

	res, err := h.usecase.CreateOrder(c.Request().Context(), customerID, &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, res)
}
