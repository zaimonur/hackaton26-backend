package postgres

import (
	"context"
	"drewisy/internal/domain"
	"errors"

	"github.com/jmoiron/sqlx"
)

type orderRepository struct {
	db *sqlx.DB
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
