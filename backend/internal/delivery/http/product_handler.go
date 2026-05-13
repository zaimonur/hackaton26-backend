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

	// Satıcının kendi ürünlerini listelemesi
	e.GET("/seller/products", handler.FetchBySeller, AuthMiddleware(), RBACMiddleware("seller"))

	// Sadece seller
	e.POST("/products", handler.Store, AuthMiddleware(), RBACMiddleware("seller"))

	e.DELETE("/products/:id", handler.Delete, AuthMiddleware(), RBACMiddleware("seller"))
}

func (h *ProductHandler) FetchBySeller(c echo.Context) error {
	sellerID := c.Get("user_id").(string)

	res, err := h.usecase.FetchBySeller(c.Request().Context(), sellerID)
	if err != nil {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	return respondSuccess(c, http.StatusOK, res)
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
		return respondError(c, http.StatusInternalServerError, "Sunucu hatası")
	}
	return respondSuccess(c, http.StatusOK, res)
}

// Store yeni ürün ekler.
func (h *ProductHandler) Store(c echo.Context) error {
	c.Request().ParseMultipartForm(5 << 20)

	price, _ := strconv.ParseFloat(c.FormValue("price"), 64)
	file, err := c.FormFile("image")
	if err != nil {
		return respondError(c, http.StatusBadRequest, "Ürün görseli eklenmelidir")
	}

	req := domain.CreateProductRequest{
		Title:       c.FormValue("title"),
		Description: c.FormValue("description"),
		Category:    c.FormValue("category"),
		Price:       price,
		Image:       file,
	}

	sellerID := c.Get("user_id").(string) // JWT Context

	res, err := h.usecase.Store(c.Request().Context(), sellerID, &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, res)
}

func (h *ProductHandler) Delete(c echo.Context) error {
	productID := c.Param("id")
	sellerID := c.Get("user_id").(string) // AuthMiddleware'den geliyor

	err := h.usecase.Delete(c.Request().Context(), sellerID, productID)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, "Ürün başarıyla silindi")
}
