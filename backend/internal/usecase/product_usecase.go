package usecase

import (
	"context"
	"drewisy/internal/domain"
	"drewisy/internal/infrastructure/storage"
	"errors"
	"strings"
)

type productUsecase struct {
	repo    domain.ProductRepository
	storage storage.FileStorage
}

func NewProductUsecase(r domain.ProductRepository, s storage.FileStorage) domain.ProductUsecase {
	return &productUsecase{repo: r, storage: s}
}

// Kategori parametresine göre repo'yu dinamik seçer
func (u *productUsecase) Fetch(ctx context.Context, category string) ([]domain.ProductResponse, error) {
	var products []domain.Product
	var err error

	if category != "" {
		products, err = u.repo.FetchByCategory(ctx, category)
	} else {
		products, err = u.repo.Fetch(ctx)
	}

	if err != nil {
		return nil, err
	}

	res := make([]domain.ProductResponse, 0)
	for _, p := range products {
		res = append(res, mapProductToResponse(p))
	}
	return res, nil
}

func (u *productUsecase) Store(ctx context.Context, req *domain.CreateProductRequest) (*domain.ProductResponse, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)

	if req.Title == "" || req.Category == "" || req.Price <= 0 {
		return nil, errors.New("eksik veya hatalı ürün bilgisi")
	}

	if req.Image == nil {
		return nil, errors.New("ürün görseli zorunludur")
	}

	// Dosyayı Altyapı (Storage) katmanına gönder, URL'yi al
	imagePath, err := u.storage.UploadImage(req.Image, "products")
	if err != nil {
		return nil, err
	}

	product := domain.Product{
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImagePath:   imagePath, // Üretilen güvenli yol DB'ye gidiyor
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
		Title:       p.Title,
		Description: p.Description,
		Price:       p.Price,
		Category:    p.Category,
		ImagePath:   p.ImagePath,
	}
}
