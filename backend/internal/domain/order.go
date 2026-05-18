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

// SellerOrderItem: Siparişin içindeki ürünlerin detayı (DTO)
type SellerOrderItem struct {
	ProductTitle string  `json:"product_title"`
	ProductImage string  `json:"product_image"`
	Quantity     int     `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
	TotalPrice   float64 `json:"total_price"`
}

// SellerOrderResponse: Satıcı panelinde listelenecek ana sipariş (DTO)
type SellerOrderResponse struct {
	OrderID       string            `json:"order_id"`
	CustomerID    string            `json:"customer_id" db:"customer_id"`
	CustomerEmail string            `json:"customer_email"`
	TotalAmount   float64           `json:"total_amount"`
	Status        string            `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	Items         []SellerOrderItem `json:"items"`
}

// CustomerOrderItem: Müşteri sipariş detayında gösterilecek ürün DTO'su
type CustomerOrderItem struct {
	ProductTitle string  `json:"product_title"`
	ProductImage string  `json:"product_image"`
	Quantity     int     `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
}

// CustomerOrderResponse: Müşterinin kendi sipariş panelinde listelenecek ana DTO
type CustomerOrderResponse struct {
	OrderID     string              `json:"order_id"`
	TotalAmount float64             `json:"total_amount"`
	Status      string              `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	Items       []CustomerOrderItem `json:"items"`
}

// UpdateOrderStatusRequest: Statü güncelleme payload'ı
type UpdateOrderStatusRequest struct {
	Status string `json:"status" validate:"required"`
}

// OrderRepository
type OrderRepository interface {
	CreateOrderTx(ctx context.Context, order *Order, items []OrderItem) error
	FetchBySellerId(ctx context.Context, sellerID string) ([]SellerOrderResponse, error)
	FetchByCustomerId(ctx context.Context, customerID string) ([]CustomerOrderResponse, error)
	UpdateStatus(ctx context.Context, orderID string, status string, sellerID string) (string, error)
}

// OrderUsecase
type OrderUsecase interface {
	CreateOrder(ctx context.Context, customerID string, req *CreateOrderRequest) (*OrderResponse, error)

	FetchSellerOrders(ctx context.Context, sellerID string) ([]SellerOrderResponse, error)
	FetchCustomerOrders(ctx context.Context, customerID string) ([]CustomerOrderResponse, error)
	UpdateOrderStatus(ctx context.Context, sellerID string, orderID string, req *UpdateOrderStatusRequest) error
}
