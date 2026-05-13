package usecase

import (
	"context"
	"drewisy/internal/domain"
	"drewisy/internal/infrastructure/storage"
	"errors"
	"strings"
)

type productUsecase struct {
	repo      domain.ProductRepository
	storeRepo domain.StoreRepository // YENİ: Dependency Injection
	storage   storage.FileStorage
}

func NewProductUsecase(r domain.ProductRepository, sr domain.StoreRepository, s storage.FileStorage) domain.ProductUsecase {
	return &productUsecase{repo: r, storeRepo: sr, storage: s}
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

	res := make([]domain.ProductResponse, 0)
	for _, p := range products {
		res = append(res, mapProductToResponse(p))
	}
	return res, nil
}

// Kategori parametresine göre repo'yu dinamik seçer
func (u *productUsecase) Fetch(ctx context.Context, category, searchQuery string) ([]domain.ProductResponse, error) {
	products, err := u.repo.Fetch(ctx, category, searchQuery)
	if err != nil {
		return nil, err
	}

	res := make([]domain.ProductResponse, 0)
	for _, p := range products {
		res = append(res, mapProductToResponse(p))
	}
	return res, nil
}

func (u *productUsecase) Store(ctx context.Context, sellerID string, req *domain.CreateProductRequest) (*domain.ProductResponse, error) {
	// Mağazayı Bul
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, errors.New("ürün eklemek için önce bir mağaza oluşturmalısınız")
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)

	if req.Title == "" || req.Category == "" || req.Price <= 0 {
		return nil, errors.New("eksik veya hatalı ürün bilgisi")
	}
	if req.Image == nil {
		return nil, errors.New("ürün görseli zorunludur")
	}

	imagePath, err := u.storage.UploadImage(req.Image, "products")
	if err != nil {
		return nil, err
	}

	product := domain.Product{
		StoreID:     store.ID,   // Veritabanına store_id gidiyor
		StoreName:   store.Name, // Response için kullanacağız
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImagePath:   imagePath,
	}

	if err := u.repo.Store(ctx, &product); err != nil {
		return nil, errors.New("ürün veritabanına kaydedilemedi")
	}

	res := mapProductToResponse(product)
	return &res, nil
}

func mapProductToResponse(p domain.Product) domain.ProductResponse {
	return domain.ProductResponse{
		ID:          p.ID,
		StoreID:     p.StoreID,
		StoreName:   p.StoreName, // Eklendi
		Title:       p.Title,
		Description: p.Description,
		Price:       p.Price,
		Category:    p.Category,
		ImagePath:   p.ImagePath,
	}
}

func (u *productUsecase) Delete(ctx context.Context, sellerID string, productID string) error {
	// 1. Satıcının mağazasını bul
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return errors.New("işlem yapılamadı: mağaza bulunamadı")
	}

	// 2. Repo'ya ürün ID'si ve Mağaza ID'sini gönder
	return u.repo.Delete(ctx, productID, store.ID)
}
