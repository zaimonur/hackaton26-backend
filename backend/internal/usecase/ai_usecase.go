package usecase

import (
	"context"
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

	prompt := fmt.Sprintf(
		"Sen uzman bir e-ticaret metin yazarısın. Ürün adı: '%s', Kategorisi: '%s', Özellikleri/Anahtar Kelimeler: '%s'. "+
			"Bu bilgileri kullanarak satışı artıracak, SEO uyumlu, profesyonel ama samimi bir dille, "+
			"en fazla 2-3 kısa paragraf olacak şekilde bir ürün açıklaması yaz. "+
			"Çıktı sadece açıklama metni olsun, gereksiz sohbet, selamlama veya markdown başlıkları ekleme.",
		req.Title, req.Category, req.Keywords,
	)

	generatedText, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return &domain.GenerateDescriptionResponse{
		GeneratedDescription: strings.TrimSpace(generatedText),
	}, nil
}

func (u *aiUsecase) SmartSearch(ctx context.Context, req *domain.SmartSearchRequest) (*domain.SmartSearchResponse, error) {
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return nil, errors.New("arama metni boş olamaz")
	}

	products, err := u.productRepo.Fetch(ctx, "", "")
	if err != nil {
		return nil, errors.New("katalog okunamadı")
	}
	if len(products) == 0 {
		return &domain.SmartSearchResponse{Products: []domain.ProductResponse{}}, nil
	}

	type miniProduct struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Category string `json:"category"`
	}

	miniCatalog := make([]miniProduct, 0, len(products))
	productMap := make(map[string]domain.Product)

	for _, p := range products {
		miniCatalog = append(miniCatalog, miniProduct{ID: p.ID, Title: p.Title, Category: p.Category})
		productMap[p.ID] = p
	}

	catalogBytes, _ := json.Marshal(miniCatalog)

	matchedIDs, err := u.aiService.SmartSearch(ctx, string(catalogBytes), req.Query)
	if err != nil {
		return nil, err
	}

	matchedProducts := make([]domain.ProductResponse, 0)
	for _, id := range matchedIDs {
		if p, exists := productMap[id]; exists {
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
	history, err := u.historyRepo.GetByUserID(ctx, userID, 20)
	if err != nil {
		return nil, errors.New("geçmiş verisi alınamadı")
	}

	catalog, err := u.productRepo.GetAllForAI(ctx)
	if err != nil {
		return nil, errors.New("katalog verisi alınamadı")
	}

	historyJSON, _ := json.Marshal(history)
	catalogJSON, _ := json.Marshal(catalog)

	prompt := fmt.Sprintf(`Sen bir yapay zeka alışveriş asistanısın. Müşterinin son incelediği ürünler şunlardır: %s.
Sitemizdeki tüm aktif ürünler (Katalog) şunlardır: %s.
Müşterinin zevkine en uygun 12 ürünü katalogdan seç ve bu koleksiyon için yaratıcı, dikkat çekici bir başlık (hero_title) üret.
Çıktı SADECE raw JSON formatında olmalıdır. Başına veya sonuna "json" gibi markdown işaretleri, selamlaşma veya herhangi bir düz metin KESİNLİKLE ekleme.
Önereceğin ID'ler kesinlikle sana sağladığım katalogda var olan ID'ler olmalıdır.
Beklenen format: {"hero_title": "...", "recommended_product_ids": ["uuid-1", "uuid-2", "uuid-3"]}`, string(historyJSON), string(catalogJSON))

	aiResText, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		fmt.Println("❌ GEMINI API HATASI:", err)
		return nil, errors.New("yapay zeka servisi şu an yanıt veremiyor")
	}

	// TERMINALE GEMINI'NIN NE DÖNDÜĞÜNÜ BASIYORUZ (HATA AYIKLAMA İÇİN)
	fmt.Println("=== 🤖 GEMINI RAW RESPONSE ===")
	fmt.Println(aiResText)
	fmt.Println("==============================")

	cleanJSON := strings.TrimSpace(aiResText)
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var aiDTO domain.AIRecommendationJSON
	if err := json.Unmarshal([]byte(cleanJSON), &aiDTO); err != nil {
		fmt.Println("❌ JSON PARSE ERROR:", err)
		fmt.Println("🧼 TEMİZLENMEYE ÇALIŞILAN JSON:", cleanJSON)
		return nil, errors.New("ai yanıtı uygun formatta okunamadı")
	}

	recommendedProducts, err := u.productRepo.GetByIDs(ctx, aiDTO.RecommendedProductIDs)
	if err != nil {
		return nil, errors.New("önerilen ürünler veritabanından çekilemedi")
	}

	resProducts := make([]domain.ProductResponse, 0, len(recommendedProducts))
	for _, p := range recommendedProducts {
		resProducts = append(resProducts, domain.ProductResponse{
			ID: p.ID, StoreID: p.StoreID, StoreName: p.StoreName, Title: p.Title,
			Description: p.Description, Price: p.Price, Stock: p.Stock,
			Category: p.Category, ImagePath: p.ImagePath, Gallery: []string{},
		})
	}

	return &domain.HeroRecommendationResponse{
		HeroTitle:           aiDTO.HeroTitle,
		RecommendedProducts: resProducts,
	}, nil
}
