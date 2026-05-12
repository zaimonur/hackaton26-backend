package domain

import (
	"context"
	"time"
)

type Store struct {
	ID          string    `db:"id"`
	SellerID    string    `db:"seller_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type CreateStoreRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type StoreResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type StoreRepository interface {
	Create(ctx context.Context, s *Store) error
	GetBySellerId(ctx context.Context, sellerID string) (*Store, error)
}

type StoreUsecase interface {
	Create(ctx context.Context, sellerID string, req *CreateStoreRequest) (*StoreResponse, error)
}
