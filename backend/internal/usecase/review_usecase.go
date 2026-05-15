package usecase

import (
	"context"
	"drewisy/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type reviewUsecase struct {
	repo        domain.ReviewRepository
	aiService   domain.AIService
	productRepo domain.ProductRepository
}

func NewReviewUsecase(r domain.ReviewRepository, ai domain.AIService, pr domain.ProductRepository) domain.ReviewUsecase {
	return &reviewUsecase{
		repo:        r,
		aiService:   ai,
		productRepo: pr,
	}
}

func (u *reviewUsecase) CreateReview(ctx context.Context, customerID string, productID string, req *domain.CreateReviewRequest) error {
	isEligible, err := u.repo.CheckEligibility(ctx, customerID, productID)
	if err != nil {
		return err
	}

	if !isEligible {
		return errors.New("Yorum yapabilmek için ürünü satın almış ve teslim almış olmanız gerekmektedir.")
	}

	review := &domain.Review{
		ProductID: productID,
		UserID:    customerID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}

	err = u.repo.Create(ctx, review)
	if err != nil {
		return err
	}

	//  Async AI Worker Başlatılıyor
	// context.WithoutCancel Go 1.21 ile geldi. İstek kapansa bile background thread ölmeyecek.
	detachedCtx := context.WithoutCancel(ctx)

	go func(bgCtx context.Context, pID string) {
		// Goroutine içindeki işlemlere maksimum 30 saniye süre veriyoruz.
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 30*time.Second)
		defer cancel()

		if err := u.processAIInsights(timeoutCtx, pID); err != nil {
			log.Printf("Async AI Worker Hatası (ProductID: %s): %v", pID, err)
		}
	}(detachedCtx, productID)

	return nil
}

// Background Worker İşlem Mantığı
func (u *reviewUsecase) processAIInsights(ctx context.Context, productID string) error {
	// 1. Ürün bilgilerini çek
	product, err := u.productRepo.GetByID(ctx, productID)
	if err != nil {
		return err
	}

	// 2. Yorumları çek
	reviewsSummary, err := u.GetProductReviews(ctx, productID)
	if err != nil {
		return err
	}

	if len(reviewsSummary.Reviews) == 0 {
		return nil
	}

	// Yorumları metne çevir
	var reviewText strings.Builder
	for _, r := range reviewsSummary.Reviews {
		reviewText.WriteString(fmt.Sprintf("- [%d Yıldız] %s\n", r.Rating, r.Comment))
	}

	// 3. Prompt Hazırla (Kesin JSON formatı istiyoruz)
	prompt := fmt.Sprintf(`Sen bir duygu analizi uzmanısın. Aşağıdaki ürün açıklamasını ve müşteri yorumlarını analiz et.
Bana sadece 1 adet emojili kısa genel değerlendirme cümlesi ve formatı KESİNLİKLE '%%98 Memnuniyet' gibi bir sentiment skoru üret.

ÜRÜN:
%s

YORUMLAR:
%s

ÇIKTI KURALI:
- Asla Markdown bloğu ("""json) KULLANMA.
- Sadece raw JSON dön.
- JSON formatı şu şekilde olmalı: {"summary": "harika ürün 🚀", "badge": "%%95 Memnuniyet"}`, product.Description, reviewText.String())

	// 4. LLM'den yanıt al
	aiResponse, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return err
	}

	// 5. JSON Sanitize İşlemi (Markdown kalıntılarını temizle)
	cleanJSON := strings.TrimSpace(aiResponse)
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	// 6. JSON Parse
	var aiData struct {
		Summary string `json:"summary"`
		Badge   string `json:"badge"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &aiData); err != nil {
		return fmt.Errorf("ai json parse hatası: %v | raw: %s", err, cleanJSON)
	}

	// 7. DB'yi Güncelle
	return u.productRepo.UpdateAIInsights(ctx, productID, aiData.Summary, aiData.Badge)
}

func (u *reviewUsecase) GetProductReviews(ctx context.Context, productID string) (*domain.ProductReviewsSummary, error) {
	avgRating, totalReviews, err := u.repo.GetAverageRating(ctx, productID)
	if err != nil {
		return nil, err
	}

	reviews, err := u.repo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	return &domain.ProductReviewsSummary{
		AverageRating: avgRating,
		TotalReviews:  totalReviews,
		Reviews:       reviews,
	}, nil
}
