package http

import (
	"drewisy/internal/domain"
	"net/http"

	"github.com/labstack/echo/v4"
)

type AIHandler struct {
	usecase domain.AIUsecase
}

func NewAIHandler(e *echo.Group, u domain.AIUsecase) {
	handler := &AIHandler{usecase: u}

	// AuthMiddleware ve RBACMiddleware ile sadece giriş yapmış satıcılara izin veriyoruz
	e.POST("/ai/generate-description", handler.GenerateDescription, AuthMiddleware(), RBACMiddleware("seller"))

	e.POST("/ai/search", handler.SmartSearch)
}

func (h *AIHandler) GenerateDescription(c echo.Context) error {
	var req domain.GenerateDescriptionRequest

	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "geçersiz istek formatı")
	}

	res, err := h.usecase.GenerateDescription(c.Request().Context(), &req)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}

func (h *AIHandler) SmartSearch(c echo.Context) error {
	var req domain.SmartSearchRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "geçersiz istek formatı")
	}

	res, err := h.usecase.SmartSearch(c.Request().Context(), &req)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}
