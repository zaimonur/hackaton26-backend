package usecase

import (
	"context"
	"drewisy/internal/domain"
	"errors"
)

type reviewUsecase struct {
	repo domain.ReviewRepository
}

func NewReviewUsecase(r domain.ReviewRepository) domain.ReviewUsecase {
	return &reviewUsecase{repo: r}
}

func (u *reviewUsecase) CreateReview(ctx context.Context, customerID string, productID string, req *domain.CreateReviewRequest) error {
	// 1. İş Kuralı: Uygunluk Kontrolü
	isEligible, err := u.repo.CheckEligibility(ctx, customerID, productID)
	if err != nil {
		return err // DB bağlantı/sorgu hatası
	}

	if !isEligible {
		return errors.New("Yorum yapabilmek için ürünü satın almış ve teslim almış olmanız gerekmektedir.")
	}

	// 2. DTO -> Entity Maplemesi
	review := &domain.Review{
		ProductID: productID,
		UserID:    customerID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	}

	// 3. Veritabanına Kayıt
	return u.repo.Create(ctx, review)
}

func (u *reviewUsecase) GetProductReviews(ctx context.Context, productID string) (*domain.ProductReviewsSummary, error) {

	avgRating, totalReviews, err := u.repo.GetAverageRating(ctx, productID)
	if err != nil {
		return nil, err
	}

	reviews, err := u.repo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	return &domain.ProductReviewsSummary{
		AverageRating: avgRating,
		TotalReviews:  totalReviews,
		Reviews:       reviews,
	}, nil
}
