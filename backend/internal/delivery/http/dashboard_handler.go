package http

import (
	"drewisy/internal/domain"
	"net/http"

	"github.com/labstack/echo/v4"
)

type DashboardHandler struct {
	usecase domain.DashboardUsecase
}

func NewDashboardHandler(e *echo.Group, u domain.DashboardUsecase) {
	handler := &DashboardHandler{usecase: u}
	e.GET("/seller/dashboard/sales", handler.GetSalesDashboard, AuthMiddleware(), RBACMiddleware("seller"))
	e.GET("/seller/dashboard/ai-summary", handler.GetAISummary, AuthMiddleware(), RBACMiddleware("seller"))
}

func (h *DashboardHandler) GetSalesDashboard(c echo.Context) error {
	var query domain.SalesDashboardQuery
	if err := c.Bind(&query); err != nil {
		return respondError(c, http.StatusBadRequest, "geçersiz query parametreleri")
	}

	sellerID, ok := c.Get("user_id").(string)
	if !ok || sellerID == "" {
		return respondError(c, http.StatusUnauthorized, "yetkisiz erişim")
	}

	res, err := h.usecase.GetDashboardData(c.Request().Context(), sellerID, &query)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}

func (h *DashboardHandler) GetAISummary(c echo.Context) error {
	sellerID, ok := c.Get("user_id").(string)
	if !ok || sellerID == "" {
		return respondError(c, http.StatusUnauthorized, "yetkisiz erişim")
	}

	res, err := h.usecase.GetAISummary(c.Request().Context(), sellerID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}
