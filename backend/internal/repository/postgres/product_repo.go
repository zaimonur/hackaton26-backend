package postgres

import (
	"context"
	"drewisy/internal/domain"
	"errors"

	"github.com/jmoiron/sqlx"
)

type productRepository struct {
	db *sqlx.DB
}

func NewProductRepository(db *sqlx.DB) domain.ProductRepository {
	return &productRepository{db}
}

func (r *productRepository) Fetch(ctx context.Context) ([]domain.Product, error) {
	// JOIN ile store_name eklendi
	query := `SELECT p.id, p.store_id, s.name AS store_name, p.title, p.description, p.price, p.category, p.image_path, p.created_at, p.updated_at 
			  FROM products p 
			  JOIN stores s ON p.store_id = s.id 
			  ORDER BY p.created_at DESC`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) FetchByCategory(ctx context.Context, category string) ([]domain.Product, error) {
	query := `SELECT p.id, p.store_id, s.name AS store_name, p.title, p.description, p.price, p.category, p.image_path, p.created_at, p.updated_at 
			  FROM products p 
			  JOIN stores s ON p.store_id = s.id 
			  WHERE p.category = $1 
			  ORDER BY p.created_at DESC`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, category); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) FetchByStoreId(ctx context.Context, storeID string) ([]domain.Product, error) {
	query := `SELECT p.id, p.store_id, s.name AS store_name, p.title, p.description, p.price, p.category, p.image_path, p.created_at, p.updated_at 
			  FROM products p 
			  JOIN stores s ON p.store_id = s.id 
			  WHERE p.store_id = $1 
			  ORDER BY p.created_at DESC`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, storeID); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) Store(ctx context.Context, p *domain.Product) error {
	query := `INSERT INTO products (store_id, title, description, price, category, image_path, created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW()) 
			  RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, query, p.StoreID, p.Title, p.Description, p.Price, p.Category, p.ImagePath).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *productRepository) Delete(ctx context.Context, id string, storeID string) error {
	query := `DELETE FROM products WHERE id = $1 AND store_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, storeID)
	if err != nil {
		return err // Tip uyuşmazlığı (UUID hatası) buraya düşer
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errors.New("ürün bulunamadı veya bu ürünü silme yetkiniz yok")
	}

	return nil
}
