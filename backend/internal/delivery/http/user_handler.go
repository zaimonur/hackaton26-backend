package http

import (
	"net/http"
	"seledec/internal/domain"

	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	usecase domain.UserUsecase
}

func NewUserHandler(e *echo.Group, u domain.UserUsecase) {
	handler := &UserHandler{usecase: u}
	e.POST("/register", handler.Register)
	e.POST("/login", handler.Login)
}

func (h *UserHandler) Register(c echo.Context) error {
	var req domain.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz veri")
	}

	res, err := h.usecase.Register(c.Request().Context(), &req)
	if err != nil {
		return respondError(c, http.StatusConflict, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, res)
}

func (h *UserHandler) Login(c echo.Context) error {
	var req domain.LoginRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz format")
	}

	res, err := h.usecase.Login(c.Request().Context(), &req)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}
