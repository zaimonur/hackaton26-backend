package postgres

import (
	"context"
	"drewisy/internal/domain"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type orderRepository struct {
	db *sqlx.DB
}

// sellerOrderRow: SQL JOIN sonucunu maplemek için flat yapıdaki internal struct. Dışarı sızdırılmaz.
type sellerOrderRow struct {
	OrderID       string    `db:"order_id"`
	CustomerEmail string    `db:"customer_email"`
	TotalAmount   float64   `db:"total_amount"`
	Status        string    `db:"status"`
	CreatedAt     time.Time `db:"created_at"`
	ProductTitle  string    `db:"product_title"`
	ProductImage  string    `db:"product_image"`
	Quantity      int       `db:"quantity"`
	UnitPrice     float64   `db:"unit_price"`
	TotalPrice    float64   `db:"total_price"`
}

func NewOrderRepository(db *sqlx.DB) domain.OrderRepository {
	return &orderRepository{db}
}

func (r *orderRepository) Create(ctx context.Context, order *domain.Order, items []domain.OrderItem) (err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	// Panic durumunda ve hata yönetiminde defer ile güvenli rollback/commit
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	orderQuery := `INSERT INTO orders (customer_id, total_amount, status, created_at, updated_at) 
				   VALUES ($1, $2, $3, NOW(), NOW()) RETURNING id, created_at, updated_at`

	err = tx.QueryRowxContext(ctx, orderQuery, order.CustomerID, order.TotalAmount, order.Status).
		Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return errors.New("sipariş oluşturulamadı")
	}

	itemQuery := `INSERT INTO order_items (order_id, product_id, quantity, unit_price, created_at, updated_at) 
				  VALUES ($1, $2, $3, $4, NOW(), NOW())`

	for _, item := range items {
		_, err = tx.ExecContext(ctx, itemQuery, order.ID, item.ProductID, item.Quantity, item.UnitPrice)
		if err != nil {
			return errors.New("sipariş kalemleri eklenemedi")
		}
	}

	return nil
}

// FetchBySellerId: Flat SQL verisini çeker, Go tarafında OrderID bazlı (Nested) gruplar.
func (r *orderRepository) FetchBySellerId(ctx context.Context, sellerID string) ([]domain.SellerOrderResponse, error) {
	query := `
		SELECT o.id AS order_id, u.email AS customer_email, o.total_amount, o.status, o.created_at,
		       p.title AS product_title, p.image_path AS product_image, 
		       oi.quantity, oi.unit_price, (oi.quantity * oi.unit_price) AS total_price
		FROM orders o
		JOIN users u ON o.customer_id = u.id
		JOIN order_items oi ON o.id = oi.order_id
		JOIN products p ON oi.product_id = p.id
		JOIN stores s ON p.store_id = s.id
		WHERE s.seller_id = $1
		ORDER BY o.created_at DESC
	`

	var rows []sellerOrderRow
	if err := r.db.SelectContext(ctx, &rows, query, sellerID); err != nil {
		return nil, err
	}

	// Pointer map ile O(n) zaman karmaşıklığında gruplama yapıyoruz.
	orderMap := make(map[string]*domain.SellerOrderResponse)
	var orderedIDs []string // Veritabanından gelen DESC sıralamasını korumak için index takipçisi

	for _, row := range rows {
		if _, exists := orderMap[row.OrderID]; !exists {
			orderMap[row.OrderID] = &domain.SellerOrderResponse{
				OrderID:       row.OrderID,
				CustomerEmail: row.CustomerEmail,
				TotalAmount:   row.TotalAmount,
				Status:        row.Status,
				CreatedAt:     row.CreatedAt,
				Items:         []domain.SellerOrderItem{}, // Null pointer hatasını önlemek için initialize
			}
			orderedIDs = append(orderedIDs, row.OrderID)
		}

		item := domain.SellerOrderItem{
			ProductTitle: row.ProductTitle,
			ProductImage: row.ProductImage,
			Quantity:     row.Quantity,
			UnitPrice:    row.UnitPrice,
			TotalPrice:   row.TotalPrice,
		}

		orderMap[row.OrderID].Items = append(orderMap[row.OrderID].Items, item)
	}

	// Map'i sıralı array'e dönüştürme
	var result []domain.SellerOrderResponse
	for _, id := range orderedIDs {
		result = append(result, *orderMap[id])
	}

	if result == nil {
		result = []domain.SellerOrderResponse{} // Frontend'e null yerine [] dönmesi için
	}

	return result, nil
}

// UpdateStatus: IDOR Korumalı Update işlemi. Sadece satıcının kendine ait mağazasıyla ilişkili siparişleri güncelleyebilir.
func (r *orderRepository) UpdateStatus(ctx context.Context, orderID string, status string, sellerID string) error {
	query := `
		UPDATE orders 
		SET status = $1, updated_at = NOW()
		WHERE id = $2 
		AND EXISTS (
			SELECT 1 
			FROM order_items oi
			JOIN products p ON oi.product_id = p.id
			JOIN stores s ON p.store_id = s.id
			WHERE oi.order_id = orders.id AND s.seller_id = $3
		)
	`

	result, err := r.db.ExecContext(ctx, query, status, orderID, sellerID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// Eğer 0 satır güncellendiyse ya sipariş yoktur ya da satıcının yetkisi (IDOR) yoktur.
	if rowsAffected == 0 {
		return errors.New("sipariş bulunamadı veya bu işlemi yapmak için yetkiniz yok")
	}

	return nil
}
