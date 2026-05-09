package usecase

import (
	"context"
	"errors"
	"seledec/internal/domain"
	"strings"
)

type productUsecase struct {
	repo domain.ProductRepository
}

func NewProductUsecase(r domain.ProductRepository) domain.ProductUsecase {
	return &productUsecase{repo: r}
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

	if req.Title == "" {
		return nil, errors.New("ürün başlığı boş olamaz")
	}
	if req.Category == "" {
		return nil, errors.New("kategori boş olamaz")
	}
	if req.Price <= 0 {
		return nil, errors.New("fiyat 0'dan büyük olmalıdır")
	}

	product := domain.Product{
		Title:       req.Title,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImagePath:   req.ImagePath,
	}

	if err := u.repo.Store(ctx, &product); err != nil {
		return nil, err
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
