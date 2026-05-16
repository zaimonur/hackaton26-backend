package postgres

import (
	"context"
	"drewisy/internal/domain"

	"github.com/jmoiron/sqlx"
)

type historyRepository struct {
	db *sqlx.DB
}

func NewHistoryRepository(db *sqlx.DB) domain.HistoryRepository {
	return &historyRepository{db}
}

func (r *historyRepository) Upsert(ctx context.Context, userID, productID string) error {
	query := `
		INSERT INTO user_view_history (user_id, product_id, viewed_at) 
		VALUES ($1, $2, NOW()) 
		ON CONFLICT (user_id, product_id) 
		DO UPDATE SET viewed_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, query, userID, productID)
	return err
}

func (r *historyRepository) GetByUserID(ctx context.Context, userID string, limit int) ([]domain.Product, error) {
	query := `
		SELECT p.id, p.store_id, s.name AS store_name, p.title, p.description, p.price, 
		       p.stock, p.category, p.image_path, p.created_at, p.updated_at 
		FROM products p 
		JOIN stores s ON p.store_id = s.id 
		JOIN user_view_history h ON p.id = h.product_id 
		WHERE h.user_id = $1 
		ORDER BY h.viewed_at DESC 
		LIMIT $2
	`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, userID, limit); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}
