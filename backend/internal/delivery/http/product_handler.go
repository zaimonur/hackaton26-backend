package http

import (
	"drewisy/internal/domain"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type ProductHandler struct {
	usecase domain.ProductUsecase
}

// NewProductHandler rotaları Echo grubuna bağlar.
func NewProductHandler(e *echo.Group, u domain.ProductUsecase) {
	handler := &ProductHandler{usecase: u}

	e.GET("/products", handler.Fetch)

	// BURASI GÜNCELLENDİ: Artık hem admin hem staff ürün ekleyebilir
	e.POST("/products", handler.Store, AuthMiddleware(), RBACMiddleware("admin", "staff"))
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
	// Güvenlik: Maximum 5MB dosya boyutu (OOM Koruması)
	c.Request().ParseMultipartForm(5 << 20)

	price, _ := strconv.ParseFloat(c.FormValue("price"), 64)

	// Dosyanın metadata'sını (Header) formdan çek
	file, err := c.FormFile("image")
	if err != nil {
		return respondError(c, http.StatusBadRequest, "Ürün görseli eklenmelidir")
	}

	// Sadece referansları DTO'ya bind ediyoruz
	req := domain.CreateProductRequest{
		Title:       c.FormValue("title"),
		Description: c.FormValue("description"),
		Category:    c.FormValue("category"),
		Price:       price,
		Image:       file, // Fiziksel dosya değil, Header pointer'ı
	}

	res, err := h.usecase.Store(c.Request().Context(), &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, res)
}
