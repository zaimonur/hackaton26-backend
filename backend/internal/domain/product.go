package domain

import (
	"context"
	"time"
)

type Product struct {
	ID          string    `db:"id"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	Price       float64   `db:"price"`
	Category    string    `db:"category"`
	ImagePath   string    `db:"image_path"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type CreateProductRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	ImagePath   string  `json:"image_path"`
}

type ProductResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	ImagePath   string  `json:"image_path"`
}

type ProductRepository interface {
	Fetch(ctx context.Context) ([]Product, error)
	FetchByCategory(ctx context.Context, category string) ([]Product, error)
	Store(ctx context.Context, p *Product) error
}

type ProductUsecase interface {
	Fetch(ctx context.Context, category string) ([]ProductResponse, error)
	Store(ctx context.Context, req *CreateProductRequest) (*ProductResponse, error)
}
