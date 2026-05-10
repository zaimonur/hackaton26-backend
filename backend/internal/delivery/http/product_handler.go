package http

import (
	"net/http"

	"drewisy/internal/domain"

	"github.com/labstack/echo/v4"
)

type ProductHandler struct {
	usecase domain.ProductUsecase
}

// NewProductHandler rotaları Echo grubuna bağlar.
func NewProductHandler(e *echo.Group, u domain.ProductUsecase) {
	handler := &ProductHandler{usecase: u}

	// GET isteği (ürünleri listeleme) HERKESE AÇIK
	e.GET("/products", handler.Fetch)

	// POST isteği (ürün ekleme) SADECE ADMINLERE AÇIK (Auth + Admin Middleware'leri eklendi)
	e.POST("/products", handler.Store, AuthMiddleware(), AdminMiddleware())
}

// respondError standart JSON hata yanıtı döner.
func respondError(c echo.Context, code int, message string) error {
	return c.JSON(code, domain.APIResponse{
		Success: false,
		Error:   message,
		Code:    code,
	})
}

// respondSuccess standart JSON başarılı yanıt döner.
func respondSuccess(c echo.Context, code int, data interface{}) error {
	return c.JSON(code, domain.APIResponse{
		Success: true,
		Data:    data,
		Code:    code,
	})
}

// Fetch ürün listesini getirir.
func (h *ProductHandler) Fetch(c echo.Context) error {
	category := c.QueryParam("category") // Query'den kategoriyi al

	res, err := h.usecase.Fetch(c.Request().Context(), category)
	if err != nil {
		return respondError(c, 500, "Sunucu hatası") // Hardcoded http.StatusInternalServerError yerine 500
	}
	return respondSuccess(c, 200, res)
}

// Store yeni ürün ekler.
func (h *ProductHandler) Store(c echo.Context) error {
	var req domain.CreateProductRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz JSON formatı")
	}

	res, err := h.usecase.Store(c.Request().Context(), &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondSuccess(c, http.StatusCreated, res)
}
