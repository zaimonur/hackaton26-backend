package usecase

import (
	"context"
	"errors"
	"time"

	"drewisy/internal/domain"
)

type dashboardUsecase struct {
	repo        domain.DashboardRepository
	storeRepo   domain.StoreRepository
	productRepo domain.ProductRepository
	reviewRepo  domain.ReviewRepository
	aiUsecase   domain.AIUsecase
}

func NewDashboardUsecase(r domain.DashboardRepository, sr domain.StoreRepository, pr domain.ProductRepository, rr domain.ReviewRepository, aiU domain.AIUsecase) domain.DashboardUsecase {
	return &dashboardUsecase{
		repo:        r,
		storeRepo:   sr,
		productRepo: pr,
		reviewRepo:  rr,
		aiUsecase:   aiU,
	}
}

func (u *dashboardUsecase) GetDashboardData(ctx context.Context, sellerID string, query *domain.SalesDashboardQuery) (*domain.SalesDashboardResponse, error) {
	now := time.Now()
	var startDate, endDate time.Time

	switch query.Period {
	case "daily":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endDate = startDate.Add(24 * time.Hour).Add(-time.Nanosecond)
	case "weekly":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -6)
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	case "monthly":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -29)
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	case "custom":
		var err error
		startDate, err = time.Parse("2006-01-02", query.StartDate)
		if err != nil {
			return nil, errors.New("geçersiz başlangıç tarihi formatı (YYYY-MM-DD bekleniyor)")
		}
		endParsed, err := time.Parse("2006-01-02", query.EndDate)
		if err != nil {
			return nil, errors.New("geçersiz bitiş tarihi formatı (YYYY-MM-DD bekleniyor)")
		}
		endDate = time.Date(endParsed.Year(), endParsed.Month(), endParsed.Day(), 23, 59, 59, 0, endParsed.Location())
	default:
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -29)
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	}

	res, err := u.repo.GetSalesStats(ctx, sellerID, startDate, endDate)
	if err != nil {
		return nil, errors.New("dashboard verileri alınırken bir hata oluştu")
	}

	if res.SuccessfulOrders > 0 {
		res.AverageOrderValue = res.TotalRevenue / float64(res.SuccessfulOrders)
	} else {
		res.AverageOrderValue = 0
	}

	if res.CategorySales == nil {
		res.CategorySales = []domain.CategoryStat{}
	}
	if res.ProductSales == nil {
		res.ProductSales = []domain.ProductSalesStat{}
	}

	return res, nil
}

func (u *dashboardUsecase) GetAISummary(ctx context.Context, sellerID string) (*domain.AIDashboardSummaryResponse, error) {
	// a) StoreID'yi bul
	store, err := u.storeRepo.GetBySellerId(ctx, sellerID)
	if err != nil {
		return nil, errors.New("analiz edilecek mağaza bulunamadı")
	}

	// b) Dashboard Data (Son 30 Gün)
	salesData, err := u.GetDashboardData(ctx, sellerID, &domain.SalesDashboardQuery{Period: "monthly"})
	if err != nil {
		return nil, errors.New("satış verileri alınamadı")
	}

	// c) Low Stock Products (Limit 10)
	lowStock, err := u.productRepo.GetLowStockProducts(ctx, store.ID, 10)
	if err != nil {
		return nil, errors.New("kritik stok bilgileri alınamadı")
	}

	// d) Recent Reviews (Limit 5)
	recentReviews, err := u.reviewRepo.GetRecentReviewsByStore(ctx, store.ID, 5)
	if err != nil {
		return nil, errors.New("son yorumlar alınamadı")
	}

	// e) AI Usecase çağırma
	summaryText, err := u.aiUsecase.GenerateDashboardSummary(ctx, salesData, lowStock, recentReviews)
	if err != nil {
		return nil, err
	}

	return &domain.AIDashboardSummaryResponse{
		Summary: summaryText,
	}, nil
}
