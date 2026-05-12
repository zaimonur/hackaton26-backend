package postgres

import (
	"context"
	"drewisy/internal/domain"

	"github.com/jmoiron/sqlx"
)

type storeRepository struct {
	db *sqlx.DB
}

func NewStoreRepository(db *sqlx.DB) domain.StoreRepository {
	return &storeRepository{db}
}

func (r *storeRepository) Create(ctx context.Context, s *domain.Store) error {
	query := `INSERT INTO stores (seller_id, name, description, created_at, updated_at) 
              VALUES ($1, $2, $3, NOW(), NOW()) RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, query, s.SellerID, s.Name, s.Description).
		Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *storeRepository) GetBySellerId(ctx context.Context, sellerID string) (*domain.Store, error) {
	var s domain.Store
	query := `SELECT id, seller_id, name, description, created_at, updated_at FROM stores WHERE seller_id = $1`
	err := r.db.GetContext(ctx, &s, query, sellerID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
