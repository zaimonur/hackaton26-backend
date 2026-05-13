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
}

// DI: AIService içeri enjekte ediliyor
func NewAIUsecase(aiService domain.AIService, productRepo domain.ProductRepository) domain.AIUsecase {
	return &aiUsecase{
		aiService:   aiService,
		productRepo: productRepo,
	}
}

func (u *aiUsecase) GenerateDescription(ctx context.Context, req *domain.GenerateDescriptionRequest) (*domain.GenerateDescriptionResponse, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)
	req.Keywords = strings.TrimSpace(req.Keywords)

	if req.Title == "" || req.Category == "" {
		return nil, errors.New("ürün adı ve kategorisi zorunludur")
	}

	// [KRİTİK - PROMPT ENGINEERING]
	prompt := fmt.Sprintf(
		"Sen uzman bir e-ticaret metin yazarısın. Ürün adı: '%s', Kategorisi: '%s', Özellikleri/Anahtar Kelimeler: '%s'. "+
			"Bu bilgileri kullanarak satışı artıracak, SEO uyumlu, profesyonel ama samimi bir dille, "+
			"en fazla 2-3 kısa paragraf olacak şekilde bir ürün açıklaması yaz. "+
			"Çıktı sadece açıklama metni olsun, gereksiz sohbet, selamlama veya markdown başlıkları ekleme.",
		req.Title, req.Category, req.Keywords,
	)

	// AIService (Infrastructure) çağrısı
	generatedText, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Sadece JSON (DTO) dönecek, veritabanına kayıt yok
	return &domain.GenerateDescriptionResponse{
		GeneratedDescription: strings.TrimSpace(generatedText),
	}, nil
}

func (u *aiUsecase) SmartSearch(ctx context.Context, req *domain.SmartSearchRequest) (*domain.SmartSearchResponse, error) {
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return nil, errors.New("arama metni boş olamaz")
	}

	// 1. Tüm kataloğu çek (Pagination olmadığı için limitsiz)
	products, err := u.productRepo.Fetch(ctx, "", "")
	if err != nil {
		return nil, errors.New("katalog okunamadı")
	}
	if len(products) == 0 {
		return &domain.SmartSearchResponse{Products: []domain.ProductResponse{}}, nil
	}

	// 2. Token tasarrufu için minimal JSON ve hızlı erişim için HashMap
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

	// 3. AI Servisine Gönder
	matchedIDs, err := u.aiService.SmartSearch(ctx, string(catalogBytes), req.Query)
	if err != nil {
		return nil, err
	}

	// 4. Dönen UUID'leri ProductResponse objelerine eşle
	matchedProducts := make([]domain.ProductResponse, 0)
	// (Not: mapProductToResponse metodu product_usecase'de private olduğu için kod tekrarı olmasın diye burada manuel mapliyoruz veya domain.go'ya taşıyabilirsin, hackathon için burada manuel maplemek en güvenlisi)
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
