package http

import (
	"drewisy/internal/domain"
	"net/http"

	"github.com/labstack/echo/v4"
)

type HistoryHandler struct {
	usecase domain.HistoryUsecase
}

func NewHistoryHandler(e *echo.Group, u domain.HistoryUsecase) {
	handler := &HistoryHandler{usecase: u}

	// Sadece customer için yetkilendirilmiş uçlar
	hGroup := e.Group("/users/history", AuthMiddleware(), RBACMiddleware("customer"))
	hGroup.POST("", handler.LogHistory)
	hGroup.GET("", handler.GetHistory)
}

func (h *HistoryHandler) LogHistory(c echo.Context) error {
	var req domain.HistoryLogRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "geçersiz istek formatı")
	}

	userID := c.Get("user_id").(string)

	if err := h.usecase.LogHistory(c.Request().Context(), userID, &req); err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, "Geçmiş kaydedildi")
}

func (h *HistoryHandler) GetHistory(c echo.Context) error {
	userID := c.Get("user_id").(string)

	res, err := h.usecase.GetHistory(c.Request().Context(), userID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "Görüntüleme geçmişi alınamadı")
	}

	return respondSuccess(c, http.StatusOK, res)
}
