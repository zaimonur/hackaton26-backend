package http

import (
	"drewisy/internal/domain"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

type AIHandler struct {
	usecase domain.AIUsecase
}

func NewAIHandler(e *echo.Group, u domain.AIUsecase) {
	handler := &AIHandler{usecase: u}

	// AuthMiddleware ve RBACMiddleware ile sadece giriş yapmış satıcılara izin veriyoruz
	e.POST("/ai/generate-description", handler.GenerateDescription, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("seller"))

	e.POST("/ai/search", handler.SmartSearch)
	// AI Özetleme Endpoint'i (Public)
	e.GET("/products/:id/reviews/ai-summary", handler.SummarizeProductReviews)

	e.GET("/ai/recommendations", handler.GetHeroRecommendations, AuthMiddleware(os.Getenv("JWT_SECRET")), RBACMiddleware("customer"))
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

func (h *AIHandler) SummarizeProductReviews(c echo.Context) error {
	productID := c.Param("id")

	summary, err := h.usecase.SummarizeProductReviews(c.Request().Context(), productID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, echo.Map{
		"summary": summary,
	})
}

func (h *AIHandler) GetHeroRecommendations(c echo.Context) error {
	userID := c.Get("user_id").(string)

	res, err := h.usecase.GetHeroRecommendations(c.Request().Context(), userID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, err.Error())
	}

	return respondSuccess(c, http.StatusOK, res)
}
