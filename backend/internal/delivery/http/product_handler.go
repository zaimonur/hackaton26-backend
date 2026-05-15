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

func NewProductHandler(e *echo.Group, u domain.ProductUsecase) {
	handler := &ProductHandler{usecase: u}

	// Mevcut Rotalar
	e.GET("/products", handler.Fetch)
	e.GET("/seller/products", handler.FetchBySeller, AuthMiddleware(), RBACMiddleware("seller"))
	e.POST("/products", handler.Store, AuthMiddleware(), RBACMiddleware("seller"))
	e.PATCH("/seller/products/:id", handler.UpdatePriceAndStock, AuthMiddleware(), RBACMiddleware("seller"))
	e.DELETE("/products/:id", handler.Delete, AuthMiddleware(), RBACMiddleware("seller"))

	// PDP (Public) Rotaları
	e.GET("/products/:id", handler.GetProductDetail)
	e.POST("/products/:id/ask", handler.AskQuestion)
}

func (h *ProductHandler) GetProductDetail(c echo.Context) error {
	productID := c.Param("id")

	res, err := h.usecase.GetProductDetail(c.Request().Context(), productID)
	if err != nil {
		return respondError(c, http.StatusNotFound, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}

func (h *ProductHandler) AskQuestion(c echo.Context) error {
	productID := c.Param("id")

	var req domain.ProductAskRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz istek formatı")
	}

	res, err := h.usecase.AskQuestion(c.Request().Context(), productID, &req)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}

// Ortak Response Helper'ları
func respondError(c echo.Context, code int, message string) error {
	return c.JSON(code, domain.APIResponse{
		Success: false,
		Error:   message,
		Code:    code,
	})
}

func respondSuccess(c echo.Context, code int, data interface{}) error {
	return c.JSON(code, domain.APIResponse{
		Success: true,
		Data:    data,
		Code:    code,
	})
}

func (h *ProductHandler) FetchBySeller(c echo.Context) error {
	sellerID := c.Get("user_id").(string)

	res, err := h.usecase.FetchBySeller(c.Request().Context(), sellerID)
	if err != nil {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	return respondSuccess(c, http.StatusOK, res)
}

func (h *ProductHandler) Fetch(c echo.Context) error {
	category := c.QueryParam("category")
	query := c.QueryParam("q")

	res, err := h.usecase.Fetch(c.Request().Context(), category, query)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "Sunucu hatası")
	}
	return respondSuccess(c, http.StatusOK, res)
}

func (h *ProductHandler) Store(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return respondError(c, http.StatusBadRequest, "Form verisi okunamadı")
	}

	price, _ := strconv.ParseFloat(c.FormValue("price"), 64)
	stock, _ := strconv.Atoi(c.FormValue("stock"))

	files := form.File["images"]
	if len(files) == 0 {
		return respondError(c, http.StatusBadRequest, "En az bir ürün görseli eklenmelidir")
	}

	req := domain.CreateProductRequest{
		Title:       c.FormValue("title"),
		Description: c.FormValue("description"),
		Category:    c.FormValue("category"),
		Price:       price,
		Stock:       stock,
		Images:      files,
	}

	sellerID := c.Get("user_id").(string)

	res, err := h.usecase.Store(c.Request().Context(), sellerID, &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusCreated, res)
}

func (h *ProductHandler) UpdatePriceAndStock(c echo.Context) error {
	productID := c.Param("id")
	sellerID := c.Get("user_id").(string)

	var req domain.UpdateProductRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz istek formatı")
	}

	res, err := h.usecase.UpdatePriceAndStock(c.Request().Context(), sellerID, productID, &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}

func (h *ProductHandler) Delete(c echo.Context) error {
	productID := c.Param("id")
	sellerID := c.Get("user_id").(string)

	err := h.usecase.Delete(c.Request().Context(), sellerID, productID)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, "Ürün başarıyla silindi")
}
