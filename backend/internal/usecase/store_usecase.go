package usecase

import (
	"context"
	"drewisy/internal/domain"
	"errors"
	"strings"
)

type storeUsecase struct {
	repo domain.StoreRepository
}

func NewStoreUsecase(r domain.StoreRepository) domain.StoreUsecase {
	return &storeUsecase{repo: r}
}

func (u *storeUsecase) Create(ctx context.Context, sellerID string, req *domain.CreateStoreRequest) (*domain.StoreResponse, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return nil, errors.New("mağaza adı boş olamaz")
	}

	// Aynı seller_id'ye ait mağaza var mı kontrolü
	existingStore, _ := u.repo.GetBySellerId(ctx, sellerID)
	if existingStore != nil {
		return nil, errors.New("bu satıcının zaten bir mağazası bulunuyor")
	}

	store := &domain.Store{
		SellerID:    sellerID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := u.repo.Create(ctx, store); err != nil {
		return nil, errors.New("mağaza oluşturulamadı")
	}

	return &domain.StoreResponse{
		ID:          store.ID,
		Name:        store.Name,
		Description: store.Description,
	}, nil
}
