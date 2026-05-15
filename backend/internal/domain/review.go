package domain

import (
	"context"
	"time"
)

// Review: Veritabanı modeli (Entity)
type Review struct {
	ID        string    `db:"id"`
	ProductID string    `db:"product_id"`
	UserID    string    `db:"user_id"`
	Rating    int       `db:"rating"`
	Comment   string    `db:"comment"`
	CreatedAt time.Time `db:"created_at"`
}

// CreateReviewRequest: İstemciden (Client) gelecek değerlendirme DTO'su
type CreateReviewRequest struct {
	Rating  int    `json:"rating" validate:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

// ReviewResponse: Dışa sunulacak (Frontend) tekil değerlendirme modeli DTO'su
type ReviewResponse struct {
	ID        string    `json:"id" db:"id"`
	Rating    int       `json:"rating" db:"rating"`
	Comment   string    `json:"comment" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UserEmail string    `json:"user_email" db:"user_email"`
}

// ProductReviewsSummary: Ürün detaylarında kullanılacak olan özet DTO'su
type ProductReviewsSummary struct {
	AverageRating float64          `json:"average_rating"`
	TotalReviews  int              `json:"total_reviews"`
	Reviews       []ReviewResponse `json:"reviews"`
}

// ReviewRepository: Veri erişim katmanı arayüzü (Port)
type ReviewRepository interface {
	CheckEligibility(ctx context.Context, customerID string, productID string) (bool, error)
	Create(ctx context.Context, review *Review) error
	GetByProductID(ctx context.Context, productID string) ([]ReviewResponse, error)
	GetAverageRating(ctx context.Context, productID string) (float64, int, error)
	GetRecentReviewsByStore(ctx context.Context, storeID string, limit int) ([]ReviewResponse, error)
}

// ReviewUsecase: İş mantığı katmanı arayüzü
type ReviewUsecase interface {
	CreateReview(ctx context.Context, customerID string, productID string, req *CreateReviewRequest) error
	GetProductReviews(ctx context.Context, productID string) (*ProductReviewsSummary, error)
}
