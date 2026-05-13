package domain

import (
	"context"
	"time"
)

type Order struct {
	ID          string    `db:"id"`
	CustomerID  string    `db:"customer_id"`
	TotalAmount float64   `db:"total_amount"`
	Status      string    `db:"status"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type OrderItem struct {
	ID        string    `db:"id"`
	OrderID   string    `db:"order_id"`
	ProductID string    `db:"product_id"`
	Quantity  int       `db:"quantity"`
	UnitPrice float64   `db:"unit_price"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type OrderItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type CreateOrderRequest struct {
	Items []OrderItemRequest `json:"items"`
}

type OrderResponse struct {
	OrderID     string  `json:"order_id"`
	TotalAmount float64 `json:"total_amount"`
	Status      string  `json:"status"`
}

type OrderRepository interface {
	Create(ctx context.Context, order *Order, items []OrderItem) error
}

type OrderUsecase interface {
	CreateOrder(ctx context.Context, customerID string, req *CreateOrderRequest) (*OrderResponse, error)
}
