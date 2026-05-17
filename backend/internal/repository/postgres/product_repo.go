package postgres

import (
	"context"
	"drewisy/internal/domain"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type productRepository struct {
	db *sqlx.DB
}

func NewProductRepository(db *sqlx.DB) domain.ProductRepository {
	return &productRepository{db}
}

func (r *productRepository) Fetch(ctx context.Context, category, searchQuery string) ([]domain.Product, error) {
	query := `SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, p.stock, p.category, p.image_path, p.created_at, p.updated_at 
			  FROM products p 
			  JOIN stores s ON p.store_id = s.id 
			  WHERE 1=1`

	var args []interface{}
	argId := 1

	if category != "" {
		query += fmt.Sprintf(` AND p.category = $%d`, argId)
		args = append(args, category)
		argId++
	}

	if searchQuery != "" {
		query += fmt.Sprintf(` AND (p.title ILIKE '%%' || $%d || '%%' OR p.category ILIKE '%%' || $%d || '%%')`, argId, argId)
		args = append(args, searchQuery)
		argId++
	}

	query += ` ORDER BY p.created_at DESC`

	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, args...); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) FetchByStoreId(ctx context.Context, storeID string) ([]domain.Product, error) {
	query := `SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, p.stock, p.category, p.image_path, p.created_at, p.updated_at 
			  FROM products p 
			  JOIN stores s ON p.store_id = s.id 
			  WHERE p.store_id = $1 
			  ORDER BY p.created_at DESC`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, storeID); err != nil {
		return nil, err
	}

	imgQuery := `SELECT image_path FROM product_images WHERE product_id = $1 ORDER BY created_at ASC`
	for i := range products {
		var images []string
		err := r.db.SelectContext(ctx, &images, imgQuery, products[i].ID)
		if err != nil {
			return nil, err
		}
		if images == nil {
			images = []string{}
		}
		products[i].Gallery = images
	}

	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) Store(ctx context.Context, p *domain.Product) (err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	query := `INSERT INTO products (store_id, title, description, price, stock, category, image_path, created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW()) 
			  RETURNING id, created_at, updated_at`

	err = tx.QueryRowxContext(ctx, query, p.StoreID, p.Title, p.Description, p.Price, p.Stock, p.Category, p.ImagePath).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return errors.New("ürün kaydedilemedi")
	}

	if len(p.Gallery) > 0 {
		imageQuery := `INSERT INTO product_images (product_id, image_path, created_at) VALUES ($1, $2, NOW())`
		for _, imgPath := range p.Gallery {
			_, err = tx.ExecContext(ctx, imageQuery, p.ID, imgPath)
			if err != nil {
				return errors.New("ürün görselleri kaydedilemedi")
			}
		}
	}

	return nil
}

func (r *productRepository) UpdatePriceAndStock(ctx context.Context, productID string, storeID string, price float64, stock int) error {
	query := `
		UPDATE products
		SET price = $1, stock = $2, updated_at = NOW()
		WHERE id = $3 AND store_id = $4
	`

	result, err := r.db.ExecContext(ctx, query, price, stock, productID, storeID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("ürün bulunamadı veya bu işlem için yetkiniz yok")
	}

	return nil
}

func (r *productRepository) Delete(ctx context.Context, id string, storeID string) error {
	query := `DELETE FROM products WHERE id = $1 AND store_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, storeID)
	if err != nil {
		return err
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

func (r *productRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	var p domain.Product
	query := `
    SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, p.stock, 
           p.category, p.image_path, p.ai_summary, p.ai_sentiment_score, p.ai_last_updated_at, 
           p.created_at, p.updated_at 
    FROM products p
    JOIN stores s ON p.store_id = s.id
    WHERE p.id = $1
	`
	err := r.db.GetContext(ctx, &p, query, id)
	if err != nil {
		return nil, err
	}

	var images []string
	imgQuery := `SELECT image_path FROM product_images WHERE product_id = $1 ORDER BY created_at ASC`
	err = r.db.SelectContext(ctx, &images, imgQuery, id)
	if err != nil {
		return nil, err
	}

	if images == nil {
		images = []string{}
	}
	p.Gallery = images

	return &p, nil
}

func (r *productRepository) UpdateAIInsights(ctx context.Context, productID string, summary string, sentiment string) error {
	query := `
		UPDATE products 
		SET ai_summary = $1, ai_sentiment_score = $2, ai_last_updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, summary, sentiment, productID)
	return err
}

func (r *productRepository) GetLowStockProducts(ctx context.Context, storeID string, limit int) ([]domain.Product, error) {
	query := `
		SELECT id, store_id, title, description, price, stock, category, image_path, created_at, updated_at 
		FROM products 
		WHERE store_id = $1 AND stock <= 5 
		ORDER BY stock ASC 
		LIMIT $2
	`
	var products []domain.Product
	err := r.db.SelectContext(ctx, &products, query, storeID, limit)
	if err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) GetBestsellers(ctx context.Context, limit int) ([]domain.Product, error) {
	query := `
		SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, 
		       p.stock, p.category, p.image_path, p.created_at, p.updated_at 
		FROM products p
		JOIN stores s ON p.store_id = s.id
		JOIN order_items oi ON p.id = oi.product_id
		GROUP BY p.id, p.store_id, s.seller_id, s.name, p.title, p.description, p.price, p.stock, p.category, p.image_path, p.created_at, p.updated_at
		ORDER BY SUM(oi.quantity) DESC
		LIMIT $1
	`
	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, limit); err != nil {
		return nil, err
	}
	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) GetCategories(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT category FROM products WHERE category IS NOT NULL AND category != '' ORDER BY category ASC`
	var categories []string
	if err := r.db.SelectContext(ctx, &categories, query); err != nil {
		return nil, err
	}
	if categories == nil {
		categories = []string{}
	}
	return categories, nil
}

func (r *productRepository) GetAllForAI(ctx context.Context) ([]domain.ProductLightweight, error) {
	query := `SELECT id, title, category, LEFT(description, 100) AS short_description FROM products`
	var items []domain.ProductLightweight

	if err := r.db.SelectContext(ctx, &items, query); err != nil {
		return nil, err
	}

	if items == nil {
		items = []domain.ProductLightweight{}
	}
	return items, nil
}

func (r *productRepository) GetByIDs(ctx context.Context, ids []string) ([]domain.Product, error) {
	if len(ids) == 0 {
		return []domain.Product{}, nil
	}

	query, args, err := sqlx.In(`
		SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, p.stock, p.category, p.image_path, p.created_at, p.updated_at 
		FROM products p 
		JOIN stores s ON p.store_id = s.id 
		WHERE p.id IN (?)
	`, ids)
	if err != nil {
		return nil, err
	}

	query = r.db.Rebind(query)

	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, args...); err != nil {
		return nil, err
	}

	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}

func (r *productRepository) UpdateFull(ctx context.Context, p *domain.Product) (err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if rec := recover(); rec != nil {
			tx.Rollback()
			panic(rec)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	updateQuery := `
		UPDATE products 
		SET title = $1, description = $2, price = $3, stock = $4, category = $5, image_path = $6, updated_at = NOW()
		WHERE id = $7 AND store_id = $8
	`
	result, err := tx.ExecContext(ctx, updateQuery, p.Title, p.Description, p.Price, p.Stock, p.Category, p.ImagePath, p.ID, p.StoreID)
	if err != nil {
		return errors.New("ürün bilgileri güncellenirken veritabanı hatası oluştu")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("ürün bulunamadı veya bu işlem için yetkiniz yok")
	}

	deleteImagesQuery := `DELETE FROM product_images WHERE product_id = $1`
	if _, err = tx.ExecContext(ctx, deleteImagesQuery, p.ID); err != nil {
		return errors.New("eski ürün görselleri silinemedi")
	}

	if len(p.Gallery) > 0 {
		insertImageQuery := `INSERT INTO product_images (product_id, image_path, created_at) VALUES ($1, $2, NOW())`
		for _, imgPath := range p.Gallery {
			if _, err = tx.ExecContext(ctx, insertImageQuery, p.ID, imgPath); err != nil {
				return errors.New("yeni ürün görselleri kaydedilemedi")
			}
		}
	}

	return nil
}
