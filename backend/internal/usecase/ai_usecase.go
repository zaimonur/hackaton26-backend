package usecase

import (
	"context"
	"drewisy/internal/config"
	"drewisy/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
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

var (
	priceRegex = regexp.MustCompile(`(?:max|en fazla|altı|<\s*)\s*(\d+)\s*(?:tl|lira)`)
	stockRegex = regexp.MustCompile(`(?i)(stokta|mevcut)`)
)

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

	//Promptları merkezi Config üzerinden alıyoruz
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
	// 1. Input Sanitization
	req.Query = strings.TrimSpace(strings.ToLower(req.Query))
	if req.Query == "" {
		return nil, errors.New("arama metni boş olamaz")
	}

	cacheKey := "smartsearch:intent:" + req.Query
	var intent CachedIntent

	// 2. Cache Control (Önce Redis'e bak)
	err := u.cacheRepo.Get(ctx, cacheKey, &intent)
	if err != nil {
		// ---> CACHE MISS DURUMU <---

		// ADIM 1: LLM'i çöpe attık. Niyeti Işık Hızında Regex ile Çıkarıyoruz (0.5 ms)
		parsedIntent := extractIntentFast(req.Query)

		// ADIM 2: Sadece temizlenmiş arama kelimelerini Vektöre (Embedding) çeviriyoruz
		// (Örn: "100 tl altı kırmızı elbise" yerine sadece "kırmızı elbise" vektöre gider)
		vector, vecErr := u.aiService.CreateEmbedding(ctx, parsedIntent.SearchQuery)
		if vecErr != nil {
			return nil, errors.New("arama vektörü oluşturulamadı")
		}

		// Cache yapısını doldur
		intent = CachedIntent{
			SearchQuery: parsedIntent.SearchQuery,
			MaxPrice:    parsedIntent.MaxPrice,
			InStockOnly: parsedIntent.InStockOnly,
			Vector:      vector,
		}

		// 24 saat TTL ile Redis'e kaydet ki bir daha sormasın
		_ = u.cacheRepo.Set(ctx, cacheKey, intent, 24*time.Hour)
	}

	// 3. Hybrid Search (DB - Hem vektör hem fiyat/stok filtreleri birlikte çalışır)
	similarProducts, err := u.productRepo.SearchBySimilarity(ctx, intent.Vector, 5, intent.MaxPrice, intent.InStockOnly)
	if err != nil {
		return nil, errors.New("ürün arama başarısız oldu")
	}

	// 4. API DTO Mapping
	matchedProducts := make([]domain.ProductResponse, 0, len(similarProducts))
	for _, p := range similarProducts {
		matchedProducts = append(matchedProducts, domain.ProductResponse{
			ID: p.ID, StoreID: p.StoreID, StoreName: p.StoreName, Title: p.Title,
			Description: p.Description, Price: p.Price, Category: p.Category, ImagePath: p.ImagePath,
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

	// Gömülü string silindi, merkezi config bağlandı
	prompt := fmt.Sprintf(config.ReviewSummaryPrompt, fullText.String())

	summary, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("AI özet oluştururken hata: %v", err)
	}

	return strings.TrimSpace(summary), nil
}

func (u *aiUsecase) GenerateDashboardSummary(ctx context.Context, salesData *domain.SalesDashboardResponse, lowStock []domain.Product, recentReviews []domain.ReviewResponse) (string, error) {
	// Token optimizasyonu için gereksiz JSON datalarını kırpalım
	salesJSON, _ := json.Marshal(salesData)

	type miniStock struct {
		Title string `json:"title"`
		Stock int    `json:"stock"`
	}
	var minStockList []miniStock
	for _, p := range lowStock {
		minStockList = append(minStockList, miniStock{Title: p.Title, Stock: p.Stock})
	}
	stockJSON, _ := json.Marshal(minStockList)

	type miniReview struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	var minReviewList []miniReview
	for _, r := range recentReviews {
		minReviewList = append(minReviewList, miniReview{Rating: r.Rating, Comment: r.Comment})
	}
	reviewsJSON, _ := json.Marshal(minReviewList)

	prompt := fmt.Sprintf(`Sen Drewisy e-ticaret platformunda uzman bir iş analistisin. Sana satıcının son 30 günlük satış verilerini, kritik stoklarını ve son müşteri yorumlarını veriyorum. Satıcıya doğrudan 'Sen' diye hitap ederek analiz yap. Çıktın KESİNLİKLE şu formatta bir Markdown olmalı:

1. 💰 Finansal Durum: (Satışlara göre yorum)
2. 🚨 Acil Aksiyonlar: (Stok durumuna göre uyarı)
3. 🕵️ Müşteri Nabzı: (Yorumlara göre duygu analizi ve tavsiye)
4. 💡 Haftanın Büyüme Fikri: (Çapraz satış veya kampanya tavsiyesi)
5. 📱 Sosyal Medya Gönderisi: (Satıcının ürünlerinden birini pazarlaması için kopyalayabileceği emojili ve hashtagli bir post metni).

VERİLER:
Satış Verisi: %s
Kritik Stok: %s
Son Yorumlar: %s`, string(salesJSON), string(stockJSON), string(reviewsJSON))

	summary, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("AI analiz hatası: %v", err)
	}

	return strings.TrimSpace(summary), nil
}

func (u *aiUsecase) GetHeroRecommendations(ctx context.Context, userID string) (*domain.HeroRecommendationResponse, error) {
	cacheKey := "hero_recs:" + userID
	var cachedResp domain.HeroRecommendationResponse

	// 1. Önce Redis'e Bak (In-Memory Hız - <5ms)
	err := u.cacheRepo.Get(ctx, cacheKey, &cachedResp)
	if err == nil && len(cachedResp.RecommendedProducts) > 0 {
		return &cachedResp, nil // Cache Hit! LLM'e gitmeye gerek yok.
	}

	// 2. Cache Miss (Veri Yok) Durumu:
	// Müşteriyi 5-10 saniye Gemini'ı beklemekle yormuyoruz.
	// Arka planda gizlice bir goroutine tetikleyip hesaplamayı başlatıyoruz.
	go u.asyncComputeAndCacheRecommendations(userID)

	// 3. Fallback (Yedek Plan): Müşteriye anında "En Çok Satanlar"ı gösteriyoruz.
	bestsellers, _ := u.productRepo.GetBestsellers(ctx, 6)
	resProducts := make([]domain.ProductResponse, 0, len(bestsellers))
	for _, p := range bestsellers {
		resProducts = append(resProducts, domain.ProductResponse{
			ID: p.ID, StoreID: p.StoreID, StoreName: p.StoreName, Title: p.Title,
			Description: p.Description, Price: p.Price, Category: p.Category, ImagePath: p.ImagePath,
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

	// Arka plan işlemi olduğu için kendi timeout'unu oluşturur (API isteğinden bağımsız)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	history, err := u.historyRepo.GetByUserID(ctx, userID, 20)
	if err != nil || len(history) == 0 {
		return // Geçmiş yoksa öneri de hesaplayamayız
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

	// Not: Burada daha önce Faz 2'de güncellediğimiz hibrit yapıyı kullanıyoruz (maxPrice: 0, inStock: true)
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
			ID: p.ID, StoreID: p.StoreID, StoreName: p.StoreName, Title: p.Title,
			Description: p.Description, Price: p.Price, Category: p.Category, ImagePath: p.ImagePath,
		})
	}

	finalResponse := domain.HeroRecommendationResponse{
		HeroTitle:           strings.TrimSpace(heroTitle),
		RecommendedProducts: resProducts,
	}

	// 4. Sonucu Redis'e yaz (1 saat boyunca önbellekte kalacak)
	u.cacheRepo.Set(ctx, "hero_recs:"+userID, finalResponse, 1*time.Hour)
}

func extractIntentFast(query string) CachedIntent {
	intent := CachedIntent{
		SearchQuery: query,
		MaxPrice:    0,
		InStockOnly: false,
	}

	// 1. Fiyat Yakalama (Örn: "100 tl altı elbise")
	if matches := priceRegex.FindStringSubmatch(query); len(matches) > 1 {
		if price, err := strconv.ParseFloat(matches[1], 64); err == nil {
			intent.MaxPrice = price
			// Fiyat kısmını arama metninden temizle ki vektör kafası karışmasın
			intent.SearchQuery = priceRegex.ReplaceAllString(query, "")
		}
	}

	// 2. Stok Yakalama
	if stockRegex.MatchString(query) {
		intent.InStockOnly = true
		intent.SearchQuery = stockRegex.ReplaceAllString(intent.SearchQuery, "")
	}

	intent.SearchQuery = strings.TrimSpace(intent.SearchQuery)
	return intent
}
