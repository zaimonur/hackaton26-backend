package http

import (
	"drewisy/internal/domain"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
)

type ProductHandler struct {
	usecase domain.ProductUsecase
}

func NewProductHandler(e *echo.Group, u domain.ProductUsecase) {
	handler := &ProductHandler{usecase: u}

	// Mevcut Rotalar
	e.GET("/products", handler.FetchProducts) //  1: Fetch yerine FetchProducts yapıldı
	e.GET("/seller/products", handler.FetchBySeller, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("seller"))
	e.POST("/products", handler.Store, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("seller"))
	e.PATCH("/seller/products/:id", handler.UpdatePriceAndStock, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("seller"))
	e.DELETE("/products/:id", handler.Delete, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("seller"))

	// PDP (Public) Rotaları
	e.GET("/products/:id", handler.GetProductDetail)
	e.POST("/products/:id/ask", handler.AskQuestion)

	e.GET("/products/bestsellers", handler.GetBestsellers)
	e.GET("/categories", handler.GetCategories)

	e.PUT("/seller/products/:id", handler.UpdateProductFull, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("seller"))
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

func (h *ProductHandler) FetchProducts(c echo.Context) error {
	category := c.QueryParam("category")
	searchQuery := c.QueryParam("q")

	// Query'den string olarak gelen page ve limit değerlerini Integer'a çeviriyoruz
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	// Usecase'e iletiyoruz. (0 gelse bile GetOffset fonksiyonumuz onu 1 ve 20'ye yuvarlayacak)
	products, err := h.usecase.Fetch(c.Request().Context(), category, searchQuery, page, limit)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	//  2: respondJSON yerine senin helper'ın olan respondSuccess kullanıldı.
	return respondSuccess(c, http.StatusOK, products)
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

func (h *ProductHandler) GetBestsellers(c echo.Context) error {
	res, err := h.usecase.GetBestsellers(c.Request().Context())
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "En çok satanlar getirilemedi")
	}
	return respondSuccess(c, http.StatusOK, res)
}

func (h *ProductHandler) GetCategories(c echo.Context) error {
	res, err := h.usecase.GetCategories(c.Request().Context())
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "Kategoriler getirilemedi")
	}
	return respondSuccess(c, http.StatusOK, res)
}

func (h *ProductHandler) UpdateProductFull(c echo.Context) error {
	productID := c.Param("id")
	sellerID := c.Get("user_id").(string)

	form, err := c.MultipartForm()
	if err != nil {
		return respondError(c, http.StatusBadRequest, "Form verisi okunamadı")
	}

	price, _ := strconv.ParseFloat(c.FormValue("price"), 64)
	stock, _ := strconv.Atoi(c.FormValue("stock"))

	req := domain.UpdateProductFullRequest{
		Title:       c.FormValue("title"),
		Description: c.FormValue("description"),
		Category:    c.FormValue("category"),
		Price:       price,
		Stock:       stock,
		KeptImages:  c.FormValue("kept_images"),
		Images:      form.File["images"],
	}

	res, err := h.usecase.UpdateFull(c.Request().Context(), sellerID, productID, &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}
