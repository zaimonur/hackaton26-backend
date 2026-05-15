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
	storeRepo domain.StoreRepository
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
			coverPath = path // İlk resim ana fotoğraf (kapak)
		}
		gallery = append(gallery, path) // Tüm resimler galeri tablosuna kaydolmak üzere diziye eklenir
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
