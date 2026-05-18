package usecase

import (
	"context"
	"drewisy/internal/config"
	"drewisy/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type aiUsecase struct {
	aiService   domain.AIService
	productRepo domain.ProductRepository
	reviewRepo  domain.ReviewRepository
	historyRepo domain.HistoryRepository
}

func NewAIUsecase(aiService domain.AIService, productRepo domain.ProductRepository, reviewRepo domain.ReviewRepository, historyRepo domain.HistoryRepository) domain.AIUsecase {
	return &aiUsecase{
		aiService:   aiService,
		productRepo: productRepo,
		reviewRepo:  reviewRepo,
		historyRepo: historyRepo,
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
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return nil, errors.New("arama metni boş olamaz")
	}

	// 1. Sorguyu vektöre çevir (Artık API'ye tüm DB'yi değil, sadece bu metni yolluyoruz)
	vector, err := u.aiService.CreateEmbedding(ctx, req.Query)
	if err != nil {
		return nil, err
	}

	// 2. Vektörü pgvector'de cosine similarity ile en yakın 5 ürünü bulacak şekilde arat
	similarProducts, err := u.productRepo.SearchBySimilarity(ctx, vector, 5)
	if err != nil {
		return nil, errors.New("ürün arama başarısız oldu")
	}

	// 3. Bulunan RAG bağlamını Frontend formatına çevir (LLM'i hiç metin üretiminde bile kullanmadan çok daha hızlı dönebiliriz ya da doğrudan prompt'a yedirebiliriz. Senin mevcut dönüş formatın için doğrudan mapping yapıyoruz).
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

	prompt := fmt.Sprintf(
		"Aşağıdaki müşteri yorumlarını analiz et ve bu ürünün artı/eksi yönlerini vurgulayan "+
			"2-3 cümlelik çok kısa ve etkileyici bir özet oluştur:\n\n%s",
		fullText.String(),
	)

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
	// 1. Müşterinin son 20 işlem geçmişini al
	history, err := u.historyRepo.GetByUserID(ctx, userID, 20)
	if err != nil {
		return nil, errors.New("geçmiş verisi alınamadı")
	}

	// 2. Geçmişi analiz edip LLM'e anlamlı bir "tema" bulduruyoruz
	historyJSON, _ := json.Marshal(history)
	themePrompt := fmt.Sprintf(config.HeroThemePrompt, string(historyJSON)) //Config kullanımı

	theme, err := u.aiService.GenerateText(ctx, themePrompt)
	if err != nil {
		theme = "Yeni Sezon Trendleri" // Hata olursa fallback
	}

	// 3. Çıkan bu "tema" cümlesini vektöre çevir
	vector, err := u.aiService.CreateEmbedding(ctx, theme)
	if err != nil {
		return nil, err
	}

	// 4. Tema vektörüne en yakın 6 ürünü veritabanından çek (RAG)
	recommendedProducts, err := u.productRepo.SearchBySimilarity(ctx, vector, 6)
	if err != nil {
		return nil, errors.New("önerilen ürünler veritabanından çekilemedi")
	}

	// 5. Koleksiyona havalı bir başlık üretmesi için sadece bulunan 6 ürünü veriyoruz
	var titles []string
	for _, p := range recommendedProducts {
		titles = append(titles, p.Title)
	}

	titlePrompt := fmt.Sprintf(config.HeroTitlePrompt, strings.Join(titles, ", ")) //Config kullanımı
	heroTitle, _ := u.aiService.GenerateText(ctx, titlePrompt)

	resProducts := make([]domain.ProductResponse, 0, len(recommendedProducts))
	for _, p := range recommendedProducts {
		resProducts = append(resProducts, domain.ProductResponse{
			ID: p.ID, StoreID: p.StoreID, StoreName: p.StoreName, Title: p.Title,
			Description: p.Description, Price: p.Price, Stock: p.Stock,
			Category: p.Category, ImagePath: p.ImagePath, Gallery: []string{},
		})
	}

	return &domain.HeroRecommendationResponse{
		HeroTitle:           strings.TrimSpace(heroTitle),
		RecommendedProducts: resProducts,
	}, nil
}
