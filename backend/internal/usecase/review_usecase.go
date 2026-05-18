package usecase

import (
	"context"
	"drewisy/internal/config"
	"drewisy/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type reviewUsecase struct {
	reviewRepo  domain.ReviewRepository
	aiService   domain.AIService
	productRepo domain.ProductRepository
	wg          *sync.WaitGroup
}

func NewReviewUsecase(rr domain.ReviewRepository, ai domain.AIService, pr domain.ProductRepository, wg *sync.WaitGroup) domain.ReviewUsecase {
	return &reviewUsecase{
		reviewRepo:  rr,
		aiService:   ai,
		productRepo: pr,
		wg:          wg,
	}
}

func (u *reviewUsecase) CreateReview(ctx context.Context, customerID string, productID string, req *domain.CreateReviewRequest) error {
	isEligible, err := u.reviewRepo.CheckEligibility(ctx, customerID, productID)
	if err != nil {
		return err
	}

	if !isEligible {
		return errors.New("yorum yapabilmek için ürünü satın almış ve teslim almış olmanız gerekmektedir")
	}

	review := &domain.Review{
		ProductID: productID,
		UserID:    customerID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}

	err = u.reviewRepo.Create(ctx, review)
	if err != nil {
		return err
	}

	//Async AI Worker Başlatılıyor (Graceful Shutdown destekli)
	u.wg.Add(1)
	go func(bgCtx context.Context, pID string) {
		defer u.wg.Done()

		// Arka plan işlemine 30 saniye süre veriyoruz
		aiCtx, cancel := context.WithTimeout(bgCtx, 30*time.Second)
		defer cancel()

		err := u.SummarizeProductReviews(aiCtx, pID)
		if err != nil {
			log.Printf("AI Özetleme hatası (Ürün ID: %s): %v\n", pID, err)
		}
	}(context.Background(), productID)

	return nil
}

func (u *reviewUsecase) SummarizeProductReviews(ctx context.Context, productID string) error {
	// 1. Ürün bilgilerini al
	product, err := u.productRepo.GetByID(ctx, productID)
	if err != nil {
		return err
	}

	// 2. Ürüne ait yorumları al
	reviews, err := u.reviewRepo.GetByProductID(ctx, productID)
	if err != nil {
		return err
	}

	if len(reviews) == 0 {
		return nil
	}

	var reviewText strings.Builder
	for i, r := range reviews {
		if i > 50 { // Token israfını önlemek için son 50 yorumla sınırlandırıldı
			break
		}
		reviewText.WriteString(fmt.Sprintf("- Puan: %d/5 | Yorum: %s\n", r.Rating, r.Comment))
	}

	// 3. AI Prompt'unu merkezi config'den alıp hazırla ()
	prompt := fmt.Sprintf(config.ReviewSummaryPrompt, product.Description, reviewText.String())

	// 4. LLM'den yanıt al
	aiResponse, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return err
	}

	// 5. JSON Sanitize İşlemi
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

// Interface'e uygun hale getirildi ve eksik veriler repodan çekildi.
func (u *reviewUsecase) GetProductReviews(ctx context.Context, productID string) (*domain.ProductReviewsSummary, error) {
	// Yorumları çek (: GetByProductID)
	reviews, err := u.reviewRepo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	// Ortalama puan ve toplam yorum sayısını çek
	avgRating, totalReviews, err := u.reviewRepo.GetAverageRating(ctx, productID)
	if err != nil {
		// Hata olursa en azından yorum listesi dönsün diye default değerler atanıyor
		avgRating = 0
		totalReviews = len(reviews)
	}

	return &domain.ProductReviewsSummary{
		AverageRating: avgRating,
		TotalReviews:  totalReviews,
		Reviews:       reviews,
	}, nil
}
