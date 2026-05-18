package http

import (
	"drewisy/internal/domain"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

type ReviewHandler struct {
	usecase domain.ReviewUsecase
}

func NewReviewHandler(e *echo.Group, u domain.ReviewUsecase) {
	handler := &ReviewHandler{usecase: u}

	// Yorum Ekleme (Korumalı - Sadece müşteri rolü)
	e.POST("/products/:id/reviews", handler.Create, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("customer"))

	// Ürün Yorumlarını Getirme (Açık - Public)
	e.GET("/products/:id/reviews", handler.GetProductReviews)
}

func (h *ReviewHandler) Create(c echo.Context) error {
	productID := c.Param("id")
	customerID := c.Get("user_id").(string) // JWT AuthMiddleware'den geliyor

	var req domain.CreateReviewRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz istek formatı")
	}

	err := h.usecase.CreateReview(c.Request().Context(), customerID, productID, &req)
	if err != nil {
		if err.Error() == "Yorum yapabilmek için ürünü satın almış ve teslim almış olmanız gerekmektedir." {
			return respondError(c, http.StatusForbidden, err.Error())
		}
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, "Yorum başarıyla eklendi")
}

func (h *ReviewHandler) GetProductReviews(c echo.Context) error {
	productID := c.Param("id")

	res, err := h.usecase.GetProductReviews(c.Request().Context(), productID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "Yorumlar getirilirken bir hata oluştu")
	}

	return respondSuccess(c, http.StatusOK, res)
}
