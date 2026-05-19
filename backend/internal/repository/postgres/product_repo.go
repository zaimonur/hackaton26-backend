package postgres

import (
	"context"
	"drewisy/internal/domain"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/pgvector/pgvector-go"
)

type productRepository struct {
	db *sqlx.DB
}

func NewProductRepository(db *sqlx.DB) domain.ProductRepository {
	return &productRepository{db}
}

func (r *productRepository) Fetch(ctx context.Context, category, searchQuery string, limit int, cursorTime string) ([]domain.Product, error) {
	// Temel sorgumuz (OFFSET YOK)
	query := `SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, p.stock, p.category, p.image_path, p.created_at, p.updated_at 
			  FROM products p 
			  JOIN stores s ON p.store_id = s.id 
			  WHERE 1=1`

	var args []interface{}
	argId := 1

	// 1. Keyset Pagination (Cursor) Kontrolü
	// Müşterinin gördüğü en son ürünün tarihinden daha "eski" olanları getir
	if cursorTime != "" {
		query += fmt.Sprintf(` AND p.created_at < $%d`, argId)
		args = append(args, cursorTime)
		argId++
	}

	// 2. Diğer Filtreler
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

	// 3. Sıralama ve Limit
	query += fmt.Sprintf(` ORDER BY p.created_at DESC LIMIT $%d`, argId)
	args = append(args, limit)

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
	// 1. Tüm ürünleri tek sorguda çek
	query := `SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, p.stock, p.category, p.image_path, p.created_at, p.updated_at 
			  FROM products p 
			  JOIN stores s ON p.store_id = s.id 
			  WHERE p.store_id = $1 
			  ORDER BY p.created_at DESC`

	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, storeID); err != nil {
		return nil, err
	}

	if len(products) == 0 {
		return []domain.Product{}, nil
	}

	// 2. Ürün ID'lerini topla
	productIDs := make([]string, 0, len(products))
	for _, p := range products {
		productIDs = append(productIDs, p.ID)
	}

	// 3. Tüm görselleri TEK BİR SORGUDA (IN clause) çek
	imgQuery, args, err := sqlx.In(`SELECT product_id, image_path FROM product_images WHERE product_id IN (?) ORDER BY created_at ASC`, productIDs)
	if err != nil {
		return nil, err
	}
	imgQuery = r.db.Rebind(imgQuery)

	type ProductImage struct {
		ProductID string `db:"product_id"`
		ImagePath string `db:"image_path"`
	}
	var allImages []ProductImage
	if err := r.db.SelectContext(ctx, &allImages, imgQuery, args...); err != nil {
		return nil, err
	}

	// 4. Go tarafında bellekte (In-Memory) Map'le eşleştir
	imageMap := make(map[string][]string)
	for _, img := range allImages {
		imageMap[img.ProductID] = append(imageMap[img.ProductID], img.ImagePath)
	}

	for i := range products {
		if imgs, ok := imageMap[products[i].ID]; ok {
			products[i].Gallery = imgs
		} else {
			products[i].Gallery = []string{}
		}
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

	query := `INSERT INTO products (store_id, title, description, price, stock, category, image_path, embedding, created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()) 
			  RETURNING id, created_at, updated_at`

	err = tx.QueryRowxContext(ctx, query, p.StoreID, p.Title, p.Description, p.Price, p.Stock, p.Category, p.ImagePath, pgvector.NewVector(p.Embedding)).
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

func (r *productRepository) UpdateAIInsights(ctx context.Context, productID string, summary string, badge string, embedding []float32) error {
	query := `
		UPDATE products 
		SET ai_summary = $1, 
		    ai_sentiment_score = $2, 
		    embedding = $3, 
		    ai_last_updated_at = NOW() 
		WHERE id = $4
	`

	_, err := r.db.ExecContext(ctx, query, summary, badge, pgvector.NewVector(embedding), productID)
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

func (r *productRepository) GetPendingAIUpdates(ctx context.Context, limit int) ([]domain.Product, error) {
	// ai_last_updated_at NULL ise (yeni ürün) veya güncellenme tarihi AI'ın son baktığı tarihten büyükse getir.
	query := `
		SELECT id, title, category, description, updated_at, ai_last_updated_at 
		FROM products 
		WHERE ai_last_updated_at IS NULL OR updated_at > ai_last_updated_at
		ORDER BY updated_at ASC
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
		SET title = $1, description = $2, price = $3, stock = $4, category = $5, image_path = $6, embedding = $7, updated_at = NOW()
		WHERE id = $8 AND store_id = $9
	`
	result, err := tx.ExecContext(ctx, updateQuery, p.Title, p.Description, p.Price, p.Stock, p.Category, p.ImagePath, pgvector.NewVector(p.Embedding), p.ID, p.StoreID)
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

func (r *productRepository) SearchBySimilarity(ctx context.Context, embedding []float32, limit int, maxPrice float64, inStock bool) ([]domain.Product, error) {
	query := `
		SELECT p.id, p.store_id, s.seller_id, s.name AS store_name, p.title, p.description, p.price, p.stock, p.category, p.image_path, p.created_at, p.updated_at 
		FROM products p 
		JOIN stores s ON p.store_id = s.id 
		WHERE 1=1
	`

	var args []interface{}
	argIdx := 1

	if inStock {
		query += ` AND p.stock > 0`
	}

	if maxPrice > 0 {
		query += fmt.Sprintf(` AND p.price <= $%d`, argIdx)
		args = append(args, maxPrice)
		argIdx++
	}

	query += fmt.Sprintf(` AND (p.embedding <=> $%d) < 0.55`, argIdx)
	args = append(args, pgvector.NewVector(embedding))
	argIdx++

	// 2. VECTOR RANKING
	query += fmt.Sprintf(` ORDER BY p.embedding <=> $%d LIMIT $%d`, argIdx-1, argIdx)
	args = append(args, limit)

	var products []domain.Product
	if err := r.db.SelectContext(ctx, &products, query, args...); err != nil {
		return nil, err
	}

	if products == nil {
		products = []domain.Product{}
	}
	return products, nil
}
