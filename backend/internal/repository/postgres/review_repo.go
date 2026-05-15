package postgres

import (
	"context"
	"drewisy/internal/domain"

	"github.com/jmoiron/sqlx"
)

type reviewRepository struct {
	db *sqlx.DB
}

func NewReviewRepository(db *sqlx.DB) domain.ReviewRepository {
	return &reviewRepository{db: db}
}

func (r *reviewRepository) CheckEligibility(ctx context.Context, customerID string, productID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM orders o
			JOIN order_items oi ON o.id = oi.order_id
			WHERE o.customer_id = $1 AND oi.product_id = $2 AND o.status = 'delivered'
		)
	`
	var isEligible bool
	err := r.db.GetContext(ctx, &isEligible, query, customerID, productID)
	if err != nil {
		return false, err
	}
	return isEligible, nil
}

func (r *reviewRepository) Create(ctx context.Context, review *domain.Review) error {
	query := `
		INSERT INTO reviews (product_id, user_id, rating, comment, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at
	`
	return r.db.QueryRowxContext(ctx, query, review.ProductID, review.UserID, review.Rating, review.Comment).
		Scan(&review.ID, &review.CreatedAt)
}

func (r *reviewRepository) GetByProductID(ctx context.Context, productID string) ([]domain.ReviewResponse, error) {
	query := `
		SELECT r.id, r.rating, r.comment, r.created_at, u.email as user_email
		FROM reviews r
		JOIN users u ON r.user_id = u.id
		WHERE r.product_id = $1
		ORDER BY r.created_at DESC
	`
	var reviews []domain.ReviewResponse
	if err := r.db.SelectContext(ctx, &reviews, query, productID); err != nil {
		return nil, err
	}

	if reviews == nil {
		reviews = []domain.ReviewResponse{}
	}

	return reviews, nil
}

func (r *reviewRepository) GetAverageRating(ctx context.Context, productID string) (float64, int, error) {
	query := `
		SELECT COALESCE(AVG(rating), 0) as avg_rating, COUNT(*) as total_reviews
		FROM reviews
		WHERE product_id = $1
	`
	var result struct {
		AvgRating    float64 `db:"avg_rating"`
		TotalReviews int     `db:"total_reviews"`
	}

	if err := r.db.GetContext(ctx, &result, query, productID); err != nil {
		return 0, 0, err
	}

	return result.AvgRating, result.TotalReviews, nil
}

func (r *reviewRepository) GetRecentReviewsByStore(ctx context.Context, storeID string, limit int) ([]domain.ReviewResponse, error) {
	query := `
		SELECT r.id, r.rating, r.comment, r.created_at, u.email as user_email
		FROM reviews r
		JOIN products p ON r.product_id = p.id
		JOIN users u ON r.user_id = u.id
		WHERE p.store_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2
	`
	var reviews []domain.ReviewResponse
	if err := r.db.SelectContext(ctx, &reviews, query, storeID, limit); err != nil {
		return nil, err
	}
	if reviews == nil {
		reviews = []domain.ReviewResponse{}
	}
	return reviews, nil
}
