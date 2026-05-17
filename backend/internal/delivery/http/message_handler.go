package http

import (
	"drewisy/internal/domain"
	"net/http"

	"github.com/labstack/echo/v4"
)

type MessageHandler struct {
	usecase domain.MessageUsecase
}

func NewMessageHandler(e *echo.Group, u domain.MessageUsecase) {
	handler := &MessageHandler{usecase: u}
	e.POST("/messages", handler.SendMessage, AuthMiddleware())
	e.GET("/messages/history/:target_id", handler.GetChatHistory, AuthMiddleware())
}

func (h *MessageHandler) SendMessage(c echo.Context) error {
	var req domain.SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "geçersiz istek formatı")
	}

	senderID := c.Get("user_id").(string)

	resp, err := h.usecase.SendMessage(c.Request().Context(), senderID, &req)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, resp)
}

func (h *MessageHandler) GetChatHistory(c echo.Context) error {
	targetID := c.Param("target_id")
	currentUserID := c.Get("user_id").(string)

	resp, err := h.usecase.GetChatHistory(c.Request().Context(), currentUserID, targetID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "sohbet geçmişi alınamadı")
	}

	return respondSuccess(c, http.StatusOK, resp)
}
