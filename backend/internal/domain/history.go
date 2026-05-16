package domain

import (
	"context"
	"time"
)

type UserViewHistory struct {
	UserID    string    `db:"user_id"`
	ProductID string    `db:"product_id"`
	ViewedAt  time.Time `db:"viewed_at"`
}

type HistoryLogRequest struct {
	ProductID string `json:"product_id" validate:"required"`
}

type HistoryRepository interface {
	Upsert(ctx context.Context, userID, productID string) error
	GetByUserID(ctx context.Context, userID string, limit int) ([]Product, error)
}

type HistoryUsecase interface {
	LogHistory(ctx context.Context, userID string, req *HistoryLogRequest) error
	GetHistory(ctx context.Context, userID string) ([]ProductResponse, error)
}
