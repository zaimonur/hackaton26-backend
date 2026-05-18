package usecase

import (
	"context"
	"drewisy/internal/config"
	"drewisy/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	// 1. Müşteri bu ürünü gerçekten aldı ve teslim edildi mi?
	isEligible, err := u.reviewRepo.CheckEligibility(ctx, customerID, productID)
	if err != nil || !isEligible {
		return errors.New("Yorum yapabilmek için ürünü satın almış ve teslim almış olmanız gerekmektedir.")
	}

	// 2. Yorumu veritabanına kaydet
	review := &domain.Review{
		ProductID: productID,
		UserID:    customerID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}

	if err := u.reviewRepo.Create(ctx, review); err != nil {
		return errors.New("Yorum kaydedilemedi veya bu ürüne zaten yorum yaptınız")
	}

	// AI Özeti üretme işini buradan kopardık! Artık Gece Vardiyası (Worker) yapacak.
	return nil
}

func (u *reviewUsecase) SummarizeProductReviews(ctx context.Context, productID string) error {
	reviews, err := u.reviewRepo.GetByProductID(ctx, productID)
	if err != nil {
		return err
	}

	if len(reviews) == 0 {
		return nil
	}

	var fullText strings.Builder
	for _, r := range reviews {
		fullText.WriteString(fmt.Sprintf("- %s\n", r.Comment))
	}

	// Gömülü string silindi, JSON üreten merkezi config bağlandı
	prompt := fmt.Sprintf(config.ReviewSummaryJSONPrompt, fullText.String())

	aiResponse, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return err
	}

	cleanJSON := strings.TrimSpace(aiResponse)
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var aiData struct {
		Summary string `json:"summary"`
		Badge   string `json:"badge"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &aiData); err != nil {
		return fmt.Errorf("ai json parse hatası: %v | raw: %s", err, cleanJSON)
	}

	product, err := u.productRepo.GetByID(ctx, productID)
	if err != nil {
		return fmt.Errorf("rag için ürün bilgileri çekilemedi: %v", err)
	}

	richText := fmt.Sprintf("Kategori: %s, Başlık: %s, Açıklama: %s, Müşteri Deneyimi ve AI Özeti: %s",
		product.Category, product.Title, product.Description, aiData.Summary)

	newEmbedding, err := u.aiService.CreateEmbedding(ctx, richText)
	if err != nil {
		return fmt.Errorf("rag vektörü üretilemedi: %v", err)
	}

	return u.productRepo.UpdateAIInsights(ctx, productID, aiData.Summary, aiData.Badge, newEmbedding)
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
