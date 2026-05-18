package http

import (
	"drewisy/internal/domain"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

type NotificationHandler struct {
	usecase domain.NotificationUsecase
}

func NewNotificationHandler(e *echo.Group, u domain.NotificationUsecase) {
	handler := &NotificationHandler{usecase: u}
	e.GET("/notifications", handler.GetMyNotifications, AuthMiddleware(os.Getenv("JWT_SECRET")))
	e.PATCH("/notifications/:id/read", handler.MarkAsRead, AuthMiddleware(os.Getenv("JWT_SECRET")))
}

func (h *NotificationHandler) GetMyNotifications(c echo.Context) error {
	userID := c.Get("user_id").(string)

	resp, err := h.usecase.GetMyNotifications(c.Request().Context(), userID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "bildirimler getirilemedi")
	}

	return respondSuccess(c, http.StatusOK, resp)
}

func (h *NotificationHandler) MarkAsRead(c echo.Context) error {
	id := c.Param("id")
	userID := c.Get("user_id").(string)

	err := h.usecase.MarkAsRead(c.Request().Context(), id, userID)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, "bildirim okundu olarak işaretlendi")
}
