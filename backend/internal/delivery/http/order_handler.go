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

	// Müşteri (Customer) rotaları
	e.POST("/orders", handler.Create, AuthMiddleware(), RBACMiddleware("customer"))

	// Satıcı (Seller) rotaları
	e.GET("/seller/orders", handler.FetchSellerOrders, AuthMiddleware(), RBACMiddleware("seller"))
	e.PATCH("/seller/orders/:id/status", handler.UpdateStatus, AuthMiddleware(), RBACMiddleware("seller"))

	e.GET("/customer/orders", handler.FetchCustomerOrders, AuthMiddleware(), RBACMiddleware("customer"))
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

// FetchSellerOrders: Satıcıya ait siparişleri listeleyen uç nokta
func (h *OrderHandler) FetchSellerOrders(c echo.Context) error {
	sellerID := c.Get("user_id").(string) // JWT Context'ten güvenli okuma

	res, err := h.usecase.FetchSellerOrders(c.Request().Context(), sellerID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "Siparişler getirilirken sunucu hatası oluştu")
	}

	return respondSuccess(c, http.StatusOK, res)
}

// UpdateStatus: Satıcının sipariş statüsünü güncelleyen uç nokta
func (h *OrderHandler) UpdateStatus(c echo.Context) error {
	orderID := c.Param("id")
	sellerID := c.Get("user_id").(string) // JWT Context

	var req domain.UpdateOrderStatusRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz payload formatı")
	}

	err := h.usecase.UpdateOrderStatus(c.Request().Context(), sellerID, orderID, &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, "Sipariş statüsü başarıyla güncellendi")
}

func (h *OrderHandler) FetchCustomerOrders(c echo.Context) error {
	customerID := c.Get("user_id").(string) // JWT Context'ten güvenli okuma

	res, err := h.usecase.FetchCustomerOrders(c.Request().Context(), customerID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "Siparişler getirilirken sunucu hatası oluştu")
	}

	return respondSuccess(c, http.StatusOK, res)
}
