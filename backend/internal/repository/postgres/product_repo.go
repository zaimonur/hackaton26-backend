package postgres

import (
	"context"
	"drewisy/internal/domain"

	"github.com/jmoiron/sqlx"
)

type productRepository struct {
	db *sqlx.DB
}

func NewProductRepository(db *sqlx.DB) domain.ProductRepository {
	return &productRepository{db}
}

func (r *productRepository) Fetch(ctx context.Context) ([]domain.Product, error) {
	query := `SELECT id, title, description, price, category, image_path, created_at, updated_at FROM products ORDER BY created_at DESC`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

// Indexlenmiş kategori alanını kullanan yeni sorgu
func (r *productRepository) FetchByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	query := `SELECT id, title, description, price, category, image_path, created_at, updated_at FROM products WHERE category = $1 ORDER BY created_at DESC`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, category); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) Store(ctx context.Context, p *domain.Product) error {
	query := `
		INSERT INTO products (title, description, price, category, image_path, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) 
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRowContext(ctx, query, p.Title, p.Description, p.Price, p.Category, p.ImagePath).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}
