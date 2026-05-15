package domain

import (
	"context"
	"time"
)

// SalesDashboardQuery Dashboard filtreleme parametrelerini tutar
type SalesDashboardQuery struct {
	Period    string `query:"period"`
	StartDate string `query:"start_date"`
	EndDate   string `query:"end_date"`
}

// CategoryStat Kategori bazlı satış istatistiklerini tutar
type CategoryStat struct {
	Category   string  `json:"category" db:"category"`
	SalesCount int     `json:"sales_count" db:"sales_count"`
	Revenue    float64 `json:"revenue" db:"revenue"`
}

// ProductSalesStat Ürün bazlı satış istatistiklerini tutar
type ProductSalesStat struct {
	ProductID  string  `json:"product_id" db:"product_id"`
	Title      string  `json:"title" db:"title"`
	ImagePath  string  `json:"image_path" db:"image_path"`
	Category   string  `json:"category" db:"category"`
	SalesCount int     `json:"sales_count" db:"sales_count"`
	Revenue    float64 `json:"revenue" db:"revenue"`
}

// SalesDashboardResponse İstemciye dönülecek ana aggregate DTO'su
type SalesDashboardResponse struct {
	TotalRevenue      float64            `json:"total_revenue"`
	SuccessfulOrders  int                `json:"successful_orders"`
	AverageOrderValue float64            `json:"average_order_value"`
	CancelledOrders   int                `json:"cancelled_orders"`
	CancelledRevenue  float64            `json:"cancelled_revenue"`
	CategorySales     []CategoryStat     `json:"category_sales"`
	ProductSales      []ProductSalesStat `json:"product_sales"`
}

// AIDashboardSummaryResponse AI Özeti DTO'su
type AIDashboardSummaryResponse struct {
	Summary string `json:"summary"`
}

// DashboardRepository Veritabanı (Data Access) sözleşmesi
type DashboardRepository interface {
	GetSalesStats(ctx context.Context, sellerID string, startDate time.Time, endDate time.Time) (*SalesDashboardResponse, error)
}

// DashboardUsecase İş mantığı (Business Logic) sözleşmesi
type DashboardUsecase interface {
	GetDashboardData(ctx context.Context, sellerID string, query *SalesDashboardQuery) (*SalesDashboardResponse, error)
	GetAISummary(ctx context.Context, sellerID string) (*AIDashboardSummaryResponse, error)
}
