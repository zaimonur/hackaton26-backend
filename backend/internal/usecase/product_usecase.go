package usecase

import (
	"context"
	"drewisy/internal/config"
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

func NewProductUsecase(r domain.ProductRepository, sr domain.StoreRepository, s storage.FileStorage, rr domain.ReviewRepository, ai domain.AIService) domain.ProductUsecase {
	return &productUsecase{repo: r, storeRepo: sr, storage: s, reviewRepo: rr, aiService: ai}
}

func (u *productUsecase) FetchBySeller(ctx context.Context, sellerID string) ([]domain.ProductResponse, error) {
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("mağaza bulunamadı, önce bir mağaza oluşturmalısınız: %w", err)
	}

	products, err := u.repo.FetchByStoreId(ctx, store.ID)
	if err != nil {
		return nil, fmt.Errorf("ürünler getirilirken veritabanı hatası oluştu: %w", err)
	}

	res := make([]domain.ProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, mapProductToResponse(p))
	}
	return res, nil
}

// Usecase: Offset/Page hesaplaması tamamen kaldırıldı. Keyset (Cursor) mantığına geçildi.
func (u *productUsecase) Fetch(ctx context.Context, category, searchQuery string, limit int, cursorTime string) ([]domain.ProductResponse, error) {
	// Güvenlik: İstemci limiti abartmasın diye makul bir üst sınır (Hard Limit) koyuyoruz
	if limit <= 0 || limit > 50 {
		limit = 20 // Default limitimiz 20 ürün
	}

	// Doğrudan cursorTime'ı repoya iletiyoruz (O(1) hızında indeks taraması için)
	products, err := u.repo.Fetch(ctx, category, searchQuery, limit, cursorTime)
	if err != nil {
		return nil, fmt.Errorf("katalog getirilemedi: %w", err)
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
		return nil, fmt.Errorf("ürün eklenemedi (Mağaza doğrulama hatası): %w", err)
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)

	if req.Title == "" || req.Category == "" || req.Price <= 0 {
		return nil, errors.New("eksik veya hatalı ürün bilgisi")
	}

	// DEĞİŞEN KISIM: Upload döngüsü silindi, doğrudan frontend'den gelen URL'leri alıyoruz
	if len(req.Images) == 0 {
		return nil, errors.New("ürünün en az bir görseli bulunmak zorundadır")
	}

	product := domain.Product{
		SellerID:    store.SellerID,
		StoreID:     store.ID,
		StoreName:   store.Name,
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    req.Category,
		ImagePath:   req.Images[0], // İlk gelen URL kapak resmidir
		Gallery:     req.Images,    // Hepsi galeri dizisi
	}

	embedText := fmt.Sprintf("Kategori: %s, Başlık: %s, Açıklama: %s", product.Category, product.Title, product.Description)
	embedding, err := u.aiService.CreateEmbedding(ctx, embedText)
	if err == nil {
		product.Embedding = embedding
	} else {
		fmt.Println("UYARI: RAG Vektör üretilemedi:", err)
	}

	err = u.repo.Store(ctx, &product)
	if err != nil {
		return nil, fmt.Errorf("ürün veritabanına kaydedilemedi: %w", err)
	}

	res := mapProductToResponse(product)
	return &res, nil
}

func (u *productUsecase) UpdatePriceAndStock(ctx context.Context, sellerID string, productID string, req *domain.UpdateProductRequest) (*domain.ProductResponse, error) {
	// Pointer yerine direkt değer kullanıldığı için <= 0 kontrolü yapılıyor.
	if req.Price <= 0 {
		return nil, errors.New("fiyat 0'dan büyük olmalıdır")
	}
	if req.Stock < 0 {
		return nil, errors.New("stok negatif olamaz")
	}

	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("satıcıya ait mağaza bulunamadı: %w", err)
	}

	err = u.repo.UpdatePriceAndStock(ctx, productID, store.ID, req.Price, req.Stock)
	if err != nil {
		return nil, fmt.Errorf("ürün güncellenemedi (Belki de size ait değil): %w", err)
	}

	updatedProduct, err := u.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("güncellenen ürün getirilemedi: %w", err)
	}

	res := mapProductToResponse(*updatedProduct)
	return &res, nil
}

func (u *productUsecase) Delete(ctx context.Context, sellerID string, productID string) error {
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return fmt.Errorf("yetkisiz işlem, mağaza bulunamadı: %w", err)
	}

	err = u.repo.Delete(ctx, productID, store.ID)
	if err != nil {
		return fmt.Errorf("ürün silinemedi: %w", err)
	}
	return nil
}

func (u *productUsecase) GetProductDetail(ctx context.Context, productID string) (*domain.ProductDetailResponse, error) {
	product, err := u.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("ürün detayları getirilemedi: %w", err)
	}

	reviews, err := u.reviewRepo.GetByProductID(ctx, productID)
	if err != nil {
		reviews = []domain.ReviewResponse{}
	}

	// Pointer kontrolleri yapılarak DTO'ya flatten (düz) olarak aktarılıyor
	aiSummary := ""
	if product.AISummary != nil {
		aiSummary = *product.AISummary
	}

	aiBadge := ""
	if product.AISentimentScore != nil {
		aiBadge = *product.AISentimentScore
	}

	// Struct eşleşmesi düzeltildi
	return &domain.ProductDetailResponse{
		ID:               product.ID,
		SellerID:         product.SellerID,
		StoreID:          product.StoreID,
		StoreName:        product.StoreName,
		Title:            product.Title,
		Description:      product.Description,
		Price:            product.Price,
		Stock:            product.Stock,
		Category:         product.Category,
		ImagePath:        product.ImagePath,
		Gallery:          product.Gallery,
		AISummary:        aiSummary,
		AISentimentBadge: aiBadge,
		RecentReviews:    reviews,
	}, nil
}

func (u *productUsecase) AskQuestion(ctx context.Context, productID string, req *domain.ProductAskRequest) (*domain.ProductAskResponse, error) {
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		return nil, errors.New("soru boş olamaz")
	}

	product, err := u.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("ürün bilgisi bulunamadı: %w", err)
	}

	// AI Prompt izole edildi ve config üzerinden çağrılıyor
	systemPrompt := fmt.Sprintf(config.ProductAssistantPrompt,
		product.Title, product.Category, product.Price, product.Description, req.Question)

	aiResponse, err := u.aiService.GenerateText(ctx, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("ai asistan yanıt üretemedi: %w", err)
	}

	return &domain.ProductAskResponse{
		Answer: aiResponse,
	}, nil
}

func (u *productUsecase) GetBestsellers(ctx context.Context) ([]domain.ProductResponse, error) {
	// Limit parametresi (örneğin 10) eklendi
	products, err := u.repo.GetBestsellers(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("çok satanlar çekilemedi: %w", err)
	}

	res := make([]domain.ProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, mapProductToResponse(p))
	}
	return res, nil
}

func (u *productUsecase) GetCategories(ctx context.Context) ([]string, error) {
	return []string{
		"Elektronik", "Moda", "Ev & Yaşam", "Kozmetik",
		"Spor & Outdoor", "Kitap & Kırtasiye", "Süpermarket",
		"Oto & Yapı Market", "Hobi & Oyuncak", "Takı & Saat",
		"Bebek & Anne", "Pet Shop", "Ofis & Büro",
		"Bahçe & Yapı Market", "Bitki & Çiçek",
	}, nil
}

func (u *productUsecase) UpdateFull(ctx context.Context, sellerID string, productID string, req *domain.UpdateProductFullRequest) (*domain.ProductResponse, error) {
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("işlem yapılamadı: mağaza bulunamadı: %w", err)
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)

	if req.Title == "" || req.Category == "" || req.Price <= 0 {
		return nil, errors.New("eksik veya hatalı ürün bilgisi")
	}

	var gallery []string

	// 1. Eskiden kalan S3 URL'lerini galeriye ekle
	if len(req.KeptImages) > 0 {
		gallery = append(gallery, req.KeptImages...)
	}

	// 2. Yeni yüklenen (ve bize URL olarak gelen) resimleri galeriye ekle
	// (u.storage.UploadImage çağrısı tamamen kaldırıldı çünkü dosya zaten bulutta!)
	if len(req.Images) > 0 {
		gallery = append(gallery, req.Images...)
	}

	if len(gallery) == 0 {
		return nil, errors.New("ürünün en az bir görseli bulunmak zorundadır")
	}

	product := domain.Product{
		ID:          productID,
		SellerID:    store.SellerID,
		StoreID:     store.ID,
		StoreName:   store.Name,
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    req.Category,
		ImagePath:   gallery[0], // İlk resim her zaman vitrin görselidir
		Gallery:     gallery,
	}

	embedText := fmt.Sprintf("Kategori: %s, Başlık: %s, Açıklama: %s", product.Category, product.Title, product.Description)
	embedding, err := u.aiService.CreateEmbedding(ctx, embedText)
	if err == nil {
		product.Embedding = embedding
	} else {
		fmt.Println("UYARI: RAG Vektör güncellenemedi:", err)
	}

	err = u.repo.UpdateFull(ctx, &product)
	if err != nil {
		return nil, fmt.Errorf("ürün tamamen güncellenemedi: %w", err)
	}

	res := mapProductToResponse(product)
	return &res, nil
}

// Domain dosyasındaki ProductResponse struct'ı ile eşleştirildi
func mapProductToResponse(p domain.Product) domain.ProductResponse {

	gallery := p.Gallery
	if gallery == nil {
		gallery = []string{}
	}

	return domain.ProductResponse{
		ID:          p.ID,
		SellerID:    p.SellerID,
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

func (u *productUsecase) GenerateUploadURL(ctx context.Context, filename string) (string, string, error) {
	return u.storage.GeneratePresignedURL(ctx, "products", filename)
}
