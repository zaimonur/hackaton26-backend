package http

import (
	"drewisy/internal/domain"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

type StoreHandler struct {
	usecase domain.StoreUsecase
}

func NewStoreHandler(e *echo.Group, u domain.StoreUsecase) {
	handler := &StoreHandler{usecase: u}
	e.POST("/stores", handler.Create, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("seller"))
}

func (h *StoreHandler) Create(c echo.Context) error {
	var req domain.CreateStoreRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "geçersiz istek formatı")
	}

	sellerID := c.Get("user_id").(string) // JWT'den geliyor

	res, err := h.usecase.Create(c.Request().Context(), sellerID, &req)
	if err != nil {
		return respondError(c, http.StatusConflict, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, res)
}
