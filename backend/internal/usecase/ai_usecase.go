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
	"time"
)

type aiUsecase struct {
	aiService   domain.AIService
	productRepo domain.ProductRepository
	reviewRepo  domain.ReviewRepository
	historyRepo domain.HistoryRepository
	cacheRepo   domain.CacheRepository
}

type CachedIntent struct {
	SearchQuery string    `json:"search_query"`
	MaxPrice    float64   `json:"max_price"`
	InStockOnly bool      `json:"in_stock_only"`
	Vector      []float32 `json:"vector"`
}

func NewAIUsecase(aiService domain.AIService, productRepo domain.ProductRepository, reviewRepo domain.ReviewRepository, historyRepo domain.HistoryRepository, cacheRepo domain.CacheRepository) domain.AIUsecase {
	return &aiUsecase{
		aiService:   aiService,
		productRepo: productRepo,
		reviewRepo:  reviewRepo,
		historyRepo: historyRepo,
		cacheRepo:   cacheRepo,
	}
}

func (u *aiUsecase) GenerateDescription(ctx context.Context, req *domain.GenerateDescriptionRequest) (*domain.GenerateDescriptionResponse, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)
	req.Keywords = strings.TrimSpace(req.Keywords)

	if req.Title == "" || req.Category == "" {
		return nil, errors.New("ürün adı ve kategorisi zorunludur")
	}

	prompt := fmt.Sprintf(config.GenerateDescriptionPrompt, req.Title, req.Category, req.Keywords)

	response, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("ai açıklama üretemedi: %w", err)
	}

	return &domain.GenerateDescriptionResponse{
		GeneratedDescription: response,
	}, nil
}

func (u *aiUsecase) SmartSearch(ctx context.Context, req *domain.SmartSearchRequest) (*domain.SmartSearchResponse, error) {
	req.Query = strings.TrimSpace(strings.ToLower(req.Query))
	if req.Query == "" {
		return nil, errors.New("arama metni boş olamaz")
	}

	cacheKey := "smartsearch:intent:" + req.Query
	var intent CachedIntent

	err := u.cacheRepo.Get(ctx, cacheKey, &intent)
	if err != nil {
		log.Printf("NLP Cache Miss: LLM ile çözümleniyor -> %s", req.Query)

		parsedIntent, err := u.aiService.ParseSearchIntent(ctx, req.Query)
		if err != nil || parsedIntent == nil || parsedIntent.SearchQuery == "" {
			log.Printf("⚠️ NLP Fallback devreye girdi. Hata/Boş Sorgu: %v", err)
			parsedIntent = &domain.SearchIntent{SearchQuery: req.Query, MaxPrice: 0, InStockOnly: false}
		}

		vector, vecErr := u.aiService.CreateEmbedding(ctx, parsedIntent.SearchQuery)
		if vecErr != nil {
			log.Printf("🚨 Vektör API Hatası: %v", vecErr)
			return &domain.SmartSearchResponse{Products: []domain.ProductResponse{}}, nil
		}

		intent = CachedIntent{
			SearchQuery: parsedIntent.SearchQuery,
			MaxPrice:    parsedIntent.MaxPrice,
			InStockOnly: parsedIntent.InStockOnly,
			Vector:      vector,
		}

		_ = u.cacheRepo.Set(ctx, cacheKey, intent, 30*24*time.Hour)
	}

	similarProducts, err := u.productRepo.SearchBySimilarity(ctx, intent.Vector, 5, intent.MaxPrice, intent.InStockOnly)
	if err != nil {
		return nil, errors.New("ürün arama başarısız oldu")
	}

	matchedProducts := make([]domain.ProductResponse, 0, len(similarProducts))
	for _, p := range similarProducts {
		matchedProducts = append(matchedProducts, domain.ProductResponse{
			ID:          p.ID,
			StoreID:     p.StoreID,
			StoreName:   p.StoreName,
			Title:       p.Title,
			Description: p.Description,
			Price:       p.Price,
			Category:    p.Category,
			ImagePath:   p.ImagePath,
			SellerID:    p.SellerID,
			Stock:       p.Stock,
			Gallery:     []string{},
		})
	}

	return &domain.SmartSearchResponse{Products: matchedProducts}, nil
}

func (u *aiUsecase) SummarizeProductReviews(ctx context.Context, productID string) (string, error) {
	reviews, err := u.reviewRepo.GetByProductID(ctx, productID)
	if err != nil {
		return "", err
	}

	if len(reviews) == 0 {
		return "Henüz değerlendirme yapılmamış.", nil
	}

	var fullText strings.Builder
	for _, r := range reviews {
		fullText.WriteString(fmt.Sprintf("- %s\n", r.Comment))
	}

	prompt := fmt.Sprintf(config.ReviewSummaryPrompt, fullText.String())
	summary, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("AI özet oluştururken hata: %v", err)
	}

	return strings.TrimSpace(summary), nil
}

func (u *aiUsecase) GenerateDashboardSummary(ctx context.Context, salesData *domain.SalesDashboardResponse, lowStock []domain.Product, recentReviews []domain.ReviewResponse) (string, error) {
	var sb strings.Builder

	sb.WriteString("SATIŞ VERİLERİ:\n")
	sb.WriteString(fmt.Sprintf("- Toplam Ciro: %.2f TL\n", salesData.TotalRevenue))
	sb.WriteString(fmt.Sprintf("- Başarılı Sipariş: %d\n", salesData.SuccessfulOrders))
	sb.WriteString(fmt.Sprintf("- İptal Edilen Sipariş: %d\n", salesData.CancelledOrders))
	sb.WriteString(fmt.Sprintf("- Ortalama Sepet Tutarı: %.2f TL\n", salesData.AverageOrderValue))

	sb.WriteString("\nKRİTİK STOKLAR:\n")
	if len(lowStock) == 0 {
		sb.WriteString("- Kritik stokta ürün yok.\n")
	} else {
		for _, p := range lowStock {
			sb.WriteString(fmt.Sprintf("- %s (Stok: %d)\n", p.Title, p.Stock))
		}
	}

	sb.WriteString("\nSON YORUMLAR:\n")
	if len(recentReviews) == 0 {
		sb.WriteString("- Henüz yorum yok.\n")
	} else {
		for _, r := range recentReviews {
			sb.WriteString(fmt.Sprintf("- %d Yıldız: %s\n", r.Rating, r.Comment))
		}
	}

	prompt := fmt.Sprintf(`Sen Drewisy e-ticaret platformunda uzman bir iş analistisin. Sana satıcının son 30 günlük satış verilerini, kritik stoklarını ve son müşteri yorumlarını veriyorum. Satıcıya doğrudan 'Sen' diye hitap ederek analiz yap. Çıktın KESİNLİKLE şu formatta bir Markdown olmalı:

1. 💰 Finansal Durum: (Satışlara göre yorum)
2. 🚨 Acil Aksiyonlar: (Stok durumuna göre uyarı)
3. 🕵️ Müşteri Nabzı: (Yorumlara göre duygu analizi ve tavsiye)
4. 💡 Haftanın Büyüme Fikri: (Çapraz satış veya kampanya tavsiyesi)
5. 📱 Sosyal Medya Gönderisi: (Satıcının ürünlerinden birini pazarlaması için kopyalayabileceği emojili ve hashtagli bir post metni).

VERİLER:
%s`, sb.String())

	summary, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("AI analiz hatası: %v", err)
	}

	return strings.TrimSpace(summary), nil
}

func (u *aiUsecase) GetHeroRecommendations(ctx context.Context, userID string) (*domain.HeroRecommendationResponse, error) {
	cacheKey := "hero_recs:" + userID
	var cachedResp domain.HeroRecommendationResponse

	err := u.cacheRepo.Get(ctx, cacheKey, &cachedResp)
	if err == nil && len(cachedResp.RecommendedProducts) > 0 {
		return &cachedResp, nil
	}

	go u.asyncComputeAndCacheRecommendations(userID)

	bestsellers, _ := u.productRepo.GetBestsellers(ctx, 6)
	resProducts := make([]domain.ProductResponse, 0, len(bestsellers))
	for _, p := range bestsellers {
		resProducts = append(resProducts, domain.ProductResponse{
			ID:          p.ID,
			StoreID:     p.StoreID,
			StoreName:   p.StoreName,
			Title:       p.Title,
			Description: p.Description,
			Price:       p.Price,
			Category:    p.Category,
			ImagePath:   p.ImagePath,
			// 🚀 EKSİKLER TAMAMLANDI
			SellerID: p.SellerID,
			Stock:    p.Stock,
			Gallery:  []string{},
		})
	}

	return &domain.HeroRecommendationResponse{
		HeroTitle:           "Senin İçin Seçtiklerimiz (Trendler)",
		RecommendedProducts: resProducts,
	}, nil
}

func (u *aiUsecase) asyncComputeAndCacheRecommendations(userID string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🚨 KRİTİK HATA (Panic) yakalandı [asyncComputeAndCacheRecommendations]: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	history, err := u.historyRepo.GetByUserID(ctx, userID, 20)
	if err != nil || len(history) == 0 {
		return
	}

	historyJSON, _ := json.Marshal(history)
	themePrompt := fmt.Sprintf(config.HeroThemePrompt, string(historyJSON))

	theme, err := u.aiService.GenerateText(ctx, themePrompt)
	if err != nil {
		theme = "Yeni Sezon Trendleri"
	}

	vector, err := u.aiService.CreateEmbedding(ctx, theme)
	if err != nil {
		return
	}

	recommendedProducts, err := u.productRepo.SearchBySimilarity(ctx, vector, 6, 0, true)
	if err != nil || len(recommendedProducts) == 0 {
		return
	}

	var titles []string
	for _, p := range recommendedProducts {
		titles = append(titles, p.Title)
	}
	titlePrompt := fmt.Sprintf(config.HeroTitlePrompt, strings.Join(titles, ", "))
	heroTitle, _ := u.aiService.GenerateText(ctx, titlePrompt)

	resProducts := make([]domain.ProductResponse, 0, len(recommendedProducts))
	for _, p := range recommendedProducts {
		resProducts = append(resProducts, domain.ProductResponse{
			ID:          p.ID,
			StoreID:     p.StoreID,
			StoreName:   p.StoreName,
			Title:       p.Title,
			Description: p.Description,
			Price:       p.Price,
			Category:    p.Category,
			ImagePath:   p.ImagePath,
			SellerID:    p.SellerID,
			Stock:       p.Stock,
			Gallery:     []string{},
		})
	}

	finalResponse := domain.HeroRecommendationResponse{
		HeroTitle:           strings.TrimSpace(heroTitle),
		RecommendedProducts: resProducts,
	}

	u.cacheRepo.Set(ctx, "hero_recs:"+userID, finalResponse, 1*time.Hour)
}
