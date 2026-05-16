package usecase

import (
	"context"
	"drewisy/internal/domain"
	"errors"
)

type historyUsecase struct {
	historyRepo domain.HistoryRepository
}

func NewHistoryUsecase(hr domain.HistoryRepository) domain.HistoryUsecase {
	return &historyUsecase{historyRepo: hr}
}

func (u *historyUsecase) LogHistory(ctx context.Context, userID string, req *domain.HistoryLogRequest) error {
	if req.ProductID == "" {
		return errors.New("geçersiz ürün ID")
	}
	return u.historyRepo.Upsert(ctx, userID, req.ProductID)
}

func (u *historyUsecase) GetHistory(ctx context.Context, userID string) ([]domain.ProductResponse, error) {
	products, err := u.historyRepo.GetByUserID(ctx, userID, 20) // LIMIT 20
	if err != nil {
		return nil, err
	}

	res := make([]domain.ProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, domain.ProductResponse{
			ID:          p.ID,
			StoreID:     p.StoreID,
			StoreName:   p.StoreName,
			Title:       p.Title,
			Description: p.Description,
			Price:       p.Price,
			Stock:       p.Stock,
			Category:    p.Category,
			ImagePath:   p.ImagePath,
			Gallery:     []string{}, // Performans için history'de galeriye girmedik
		})
	}
	return res, nil
}
