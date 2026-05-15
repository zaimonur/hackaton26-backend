package usecase

import (
	"context"
	"drewisy/internal/domain"
	"drewisy/internal/infrastructure/storage"
	"errors"
	"fmt"
	"strings"
)

type productUsecase struct {
	repo       domain.ProductRepository
	storeRepo  domain.StoreRepository
	storage    storage.FileStorage
	reviewRepo domain.ReviewRepository
	aiService  domain.AIService
}

// Yeni bağımlılıklar eklendi
func NewProductUsecase(r domain.ProductRepository, sr domain.StoreRepository, s storage.FileStorage, rr domain.ReviewRepository, ai domain.AIService) domain.ProductUsecase {
	return &productUsecase{repo: r, storeRepo: sr, storage: s, reviewRepo: rr, aiService: ai}
}

func (u *productUsecase) FetchBySeller(ctx context.Context, sellerID string) ([]domain.ProductResponse, error) {
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, errors.New("mağaza bulunamadı, önce bir mağaza oluşturmalısınız")
	}

	products, err := u.repo.FetchByStoreId(ctx, store.ID)
	if err != nil {
		return nil, err
	}

	res := make([]domain.ProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, mapProductToResponse(p))
	}
	return res, nil
}

func (u *productUsecase) Fetch(ctx context.Context, category, searchQuery string) ([]domain.ProductResponse, error) {
	products, err := u.repo.Fetch(ctx, category, searchQuery)
	if err != nil {
		return nil, err
	}

	res := make([]domain.ProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, mapProductToResponse(p))
	}
	return res, nil
}

func (u *productUsecase) Store(ctx context.Context, sellerID string, req *domain.CreateProductRequest) (*domain.ProductResponse, error) {
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, errors.New("ürün eklemek için önce bir mağaza oluşturmalısınız")
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)

	if req.Title == "" || req.Category == "" || req.Price <= 0 {
		return nil, errors.New("eksik veya hatalı ürün bilgisi")
	}
	if len(req.Images) == 0 {
		return nil, errors.New("en az bir ürün görseli zorunludur")
	}

	var coverPath string
	var gallery []string

	for i, img := range req.Images {
		path, err := u.storage.UploadImage(img, "products")
		if err != nil {
			return nil, err
		}
		if i == 0 {
			coverPath = path
		}
		gallery = append(gallery, path)
	}

	product := domain.Product{
		StoreID:     store.ID,
		StoreName:   store.Name,
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    req.Category,
		ImagePath:   coverPath,
		Gallery:     gallery,
	}

	if err := u.repo.Store(ctx, &product); err != nil {
		return nil, errors.New("ürün veritabanına kaydedilemedi")
	}

	res := mapProductToResponse(product)
	return &res, nil
}

func (u *productUsecase) UpdatePriceAndStock(ctx context.Context, sellerID string, productID string, req *domain.UpdateProductRequest) (*domain.ProductResponse, error) {
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, errors.New("işlem yapılamadı: mağaza bulunamadı")
	}

	err = u.repo.UpdatePriceAndStock(ctx, productID, store.ID, req.Price, req.Stock)
	if err != nil {
		return nil, err
	}

	product, err := u.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}

	res := mapProductToResponse(*product)
	return &res, nil
}

func (u *productUsecase) Delete(ctx context.Context, sellerID string, productID string) error {
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return errors.New("işlem yapılamadı: mağaza bulunamadı")
	}

	return u.repo.Delete(ctx, productID, store.ID)
}

func (u *productUsecase) GetProductDetail(ctx context.Context, id string) (*domain.ProductDetailResponse, error) {
	product, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("ürün bulunamadı")
	}

	reviews, err := u.reviewRepo.GetByProductID(ctx, id)
	if err != nil {
		return nil, errors.New("ürün yorumları alınamadı")
	}

	// Güvenli Slicing (Bounds Checking)
	limit := 3
	if len(reviews) < 3 {
		limit = len(reviews)
	}
	recentReviews := reviews[:limit]

	// Nil Pointer Check (DB'den null gelebilecek AI alanları için)
	aiSummary := ""
	if product.AISummary != nil {
		aiSummary = *product.AISummary
	}

	aiSentimentBadge := ""
	if product.AISentimentScore != nil {
		aiSentimentBadge = *product.AISentimentScore
	}

	gallery := product.Gallery
	if gallery == nil {
		gallery = []string{}
	}

	return &domain.ProductDetailResponse{
		ID:               product.ID,
		StoreID:          product.StoreID,
		StoreName:        product.StoreName,
		Title:            product.Title,
		Description:      product.Description,
		Price:            product.Price,
		Stock:            product.Stock,
		Category:         product.Category,
		ImagePath:        product.ImagePath,
		Gallery:          gallery,
		AISummary:        aiSummary,
		AISentimentBadge: aiSentimentBadge,
		RecentReviews:    recentReviews,
	}, nil
}

func (u *productUsecase) AskQuestion(ctx context.Context, productID string, req *domain.ProductAskRequest) (*domain.ProductAskResponse, error) {
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		return nil, errors.New("soru boş olamaz")
	}

	// 1. Ürün bilgilerini çek
	product, err := u.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, errors.New("ürün bulunamadı")
	}

	// 2. KRİTİK EKSİK: Yorumları da çek (AI analiz edebilsin diye)
	reviews, _ := u.reviewRepo.GetByProductID(ctx, productID)

	// Yorumları bir metin haline getir (Sadece ilk 5 yorum yeterli bağlam sağlar)
	reviewsContext := ""
	for i, r := range reviews {
		if i >= 5 {
			break
		}
		reviewsContext += fmt.Sprintf("- %d Yıldız: %s\n", r.Rating, r.Comment)
	}
	if reviewsContext == "" {
		reviewsContext = "Henüz yorum yapılmamış."
	}

	// 3. PROMPT GÜNCELLEMESİ: Kimlik ve Bağlam
	prompt := fmt.Sprintf(`Sen bağımsız ve tarafsız bir e-ticaret alışveriş asistanısın. 
Görevin, aşağıdaki bilgilere dayanarak müşteriye yardımcı olmaktır. 
Lütfen "biz", "ürünümüz" gibi ifadeler yerine "bu ürün", "mağaza" gibi 3. şahıs ifadeleri kullan. 

ÜRÜN BİLGİLERİ:
Başlık: %s | Kategori: %s | Fiyat: %.2f TL | Stok Durumu: %d adet
Açıklama: %s

MÜŞTERİ YORUMLARI:
%s

KULLANICI SORUSU: %s

Yanıtını verirken hem ürün özelliklerini hem de kullanıcı yorumlarındaki genel havayı (memnuniyet, şikayet vb.) dikkate al.`,
		product.Title, product.Category, product.Price, product.Stock, product.Description, reviewsContext, req.Question,
	)

	answer, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return nil, errors.New("yapay zeka servisi şu an yanıt veremiyor")
	}

	return &domain.ProductAskResponse{
		Answer: strings.TrimSpace(answer),
	}, nil
}

func mapProductToResponse(p domain.Product) domain.ProductResponse {
	gallery := p.Gallery
	if gallery == nil {
		gallery = []string{}
	}

	return domain.ProductResponse{
		ID:          p.ID,
		StoreID:     p.StoreID,
		StoreName:   p.StoreName,
		Title:       p.Title,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		Category:    p.Category,
		ImagePath:   p.ImagePath,
		Gallery:     gallery,
	}
}
