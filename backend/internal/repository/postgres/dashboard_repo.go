package postgres

import (
	"context"
	"time"

	"drewisy/internal/domain"

	"github.com/jmoiron/sqlx"
)

type dashboardRepository struct {
	db *sqlx.DB
}

func NewDashboardRepository(db *sqlx.DB) domain.DashboardRepository {
	return &dashboardRepository{db}
}

func (r *dashboardRepository) GetSalesStats(ctx context.Context, sellerID string, startDate time.Time, endDate time.Time) (*domain.SalesDashboardResponse, error) {
	response := &domain.SalesDashboardResponse{
		CategorySales: []domain.CategoryStat{},
		ProductSales:  []domain.ProductSalesStat{},
	}

	// Sorgu A: Ana KPI metrikleri (Satıcının ürünlerinden elde ettiği gerçek ciro üzerinden hesaplanır)
	kpiQuery := `
		SELECT 
			COUNT(DISTINCT CASE WHEN o.status IN ('shipped', 'delivered') THEN o.id END) AS successful_orders,
			COALESCE(SUM(CASE WHEN o.status IN ('shipped', 'delivered') THEN oi.quantity * oi.unit_price ELSE 0 END), 0) AS total_revenue,
			COUNT(DISTINCT CASE WHEN o.status = 'cancelled' THEN o.id END) AS cancelled_orders,
			COALESCE(SUM(CASE WHEN o.status = 'cancelled' THEN oi.quantity * oi.unit_price ELSE 0 END), 0) AS cancelled_revenue
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		JOIN products p ON oi.product_id = p.id
		JOIN stores s ON p.store_id = s.id
		WHERE s.seller_id = $1 AND o.created_at >= $2 AND o.created_at <= $3
	`

	var kpi struct {
		SuccessfulOrders int     `db:"successful_orders"`
		TotalRevenue     float64 `db:"total_revenue"`
		CancelledOrders  int     `db:"cancelled_orders"`
		CancelledRevenue float64 `db:"cancelled_revenue"`
	}

	if err := r.db.GetContext(ctx, &kpi, kpiQuery, sellerID, startDate, endDate); err != nil {
		return nil, err
	}

	response.SuccessfulOrders = kpi.SuccessfulOrders
	response.TotalRevenue = kpi.TotalRevenue
	response.CancelledOrders = kpi.CancelledOrders
	response.CancelledRevenue = kpi.CancelledRevenue

	// Sorgu B: Kategori Satışları
	categoryQuery := `
		SELECT 
			p.category, 
			COALESCE(SUM(oi.quantity), 0) AS sales_count, 
			COALESCE(SUM(oi.quantity * oi.unit_price), 0) AS revenue
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		JOIN products p ON oi.product_id = p.id
		JOIN stores s ON p.store_id = s.id
		WHERE s.seller_id = $1 
		  AND o.created_at >= $2 
		  AND o.created_at <= $3 
		  AND o.status IN ('shipped', 'delivered')
		GROUP BY p.category
		ORDER BY revenue DESC
	`

	if err := r.db.SelectContext(ctx, &response.CategorySales, categoryQuery, sellerID, startDate, endDate); err != nil {
		return nil, err
	}

	// Sorgu C: Ürün Satışları
	productQuery := `
		SELECT 
			p.id AS product_id, 
			p.title, 
			p.image_path, 
			p.category, 
			COALESCE(SUM(oi.quantity), 0) AS sales_count, 
			COALESCE(SUM(oi.quantity * oi.unit_price), 0) AS revenue
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		JOIN products p ON oi.product_id = p.id
		JOIN stores s ON p.store_id = s.id
		WHERE s.seller_id = $1 
		  AND o.created_at >= $2 
		  AND o.created_at <= $3 
		  AND o.status IN ('shipped', 'delivered')
		GROUP BY p.id, p.title, p.image_path, p.category
		ORDER BY revenue DESC
	`

	if err := r.db.SelectContext(ctx, &response.ProductSales, productQuery, sellerID, startDate, endDate); err != nil {
		return nil, err
	}

	return response, nil
}
