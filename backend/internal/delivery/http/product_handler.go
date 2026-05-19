package http

import (
	"drewisy/internal/domain"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type ProductHandler struct {
	usecase domain.ProductUsecase
}

type PreSignedURLRequest struct {
	Count int `json:"count"`
}

type PreSignedURLResponse struct {
	UploadURL string `json:"upload_url"`
	FileURL   string `json:"file_url"`
}

func NewProductHandler(e *echo.Group, u domain.ProductUsecase) {
	handler := &ProductHandler{usecase: u}

	// Mevcut Rotalar
	e.GET("/products", handler.FetchProducts)
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

	e.POST("/s3/presigned-urls", handler.GeneratePresignedURLs)
	e.PUT("/mock-s3/:filename", handler.MockS3Upload)
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
	cursorTime := c.QueryParam("cursor")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	// Usecase'e iletiyoruz. (0 gelse bile GetOffset fonksiyonumuz onu 1 ve 20'ye yuvarlayacak)
	products, err := h.usecase.Fetch(c.Request().Context(), category, searchQuery, limit, cursorTime)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, products)
}

func (h *ProductHandler) Store(c echo.Context) error {
	sellerID := c.Get("user_id").(string)

	var req domain.CreateProductRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "geçersiz JSON formatı"})
	}

	// 1. EKLENEN DEBUG SATIRI: İOS'tan tam olarak ne gelmiş görelim
	fmt.Printf("📦 iOS'TAN GELEN PAYLOAD: %+v\n", req)

	res, err := h.usecase.Store(c.Request().Context(), sellerID, &req)
	if err != nil {
		// 2. EKLENEN DEBUG SATIRI: Hatanın tam olarak neden patladığını görelim
		fmt.Println("🚨 500 HATASININ NEDENİ:", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, res)
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

	var req domain.UpdateProductFullRequest

	// ARTIK c.MultipartForm() YOK. Sadece c.Bind() ile hafif JSON'u okuyoruz.
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz JSON formatı")
	}

	res, err := h.usecase.UpdateFull(c.Request().Context(), sellerID, productID, &req)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}

func (h *ProductHandler) GetUploadURL(c echo.Context) error {
	filename := c.QueryParam("filename")
	if filename == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "filename parametresi zorunludur"})
	}

	uploadURL, finalURL, err := h.usecase.GenerateUploadURL(c.Request().Context(), filename)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"upload_url": uploadURL, // Frontend dosyayı buraya PUT edecek
		"final_url":  finalURL,  // Frontend ürün eklerken (Store) bu URL'i JSON içinde gönderecek
	})
}

func (h *ProductHandler) GeneratePresignedURLs(c echo.Context) error {
	var req PreSignedURLRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "Geçersiz format")
	}

	if req.Count <= 0 || req.Count > 10 {
		return respondError(c, http.StatusBadRequest, "Count 1 ile 10 arasında olmalıdır")
	}

	var urls []PreSignedURLResponse
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	for i := 0; i < req.Count; i++ {
		// Context yerine doğrudan anlık UnixNano zamanını kullanıyoruz
		fileName := strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + strconv.Itoa(i) + ".jpg"

		urls = append(urls, PreSignedURLResponse{
			UploadURL: baseURL + "/api/v1/mock-s3/" + fileName,
			FileURL:   baseURL + "/static/products/" + fileName,
		})
	}

	return respondSuccess(c, http.StatusOK, urls)
}

// 2. AWS S3'ün PUT Davranışını Taklit Eden Handler
func (h *ProductHandler) MockS3Upload(c echo.Context) error {
	filename := c.Param("filename")
	if filename == "" {
		return c.String(http.StatusBadRequest, "Dosya adı eksik")
	}

	targetPath := "uploads/products/" + filename
	dst, err := os.Create(targetPath)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Dosya oluşturulamadı")
	}
	defer dst.Close()

	if c.Request().Body != nil {
		if _, err = io.Copy(dst, c.Request().Body); err != nil {
			return c.String(http.StatusInternalServerError, "Veri yazılamadı")
		}
	}
	return c.NoContent(http.StatusOK)
}
