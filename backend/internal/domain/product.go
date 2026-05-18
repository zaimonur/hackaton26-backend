package domain

import (
	"context"
	"time"
)

// Entity (Model)
type Product struct {
	ID               string     `db:"id"`
	SellerID         string     `db:"seller_id"`
	StoreID          string     `db:"store_id"`
	StoreName        string     `db:"store_name"`
	Title            string     `db:"title"`
	Description      string     `db:"description"`
	Price            float64    `db:"price"`
	Stock            int        `db:"stock"`
	Category         string     `db:"category"`
	ImagePath        string     `db:"image_path"`
	Gallery          []string   `db:"-"`
	AISummary        *string    `db:"ai_summary"`
	AISentimentScore *string    `db:"ai_sentiment_score"`
	AILastUpdatedAt  *time.Time `db:"ai_last_updated_at"`
	Embedding        []float32  `db:"embedding"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// --- DTOs ---

type CreateProductRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Stock       int      `json:"stock"`
	Category    string   `json:"category"`
	Images      []string `json:"images"`
}

type UpdateProductRequest struct {
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

type ProductResponse struct {
	ID          string   `json:"id"`
	SellerID    string   `json:"seller_id"`
	StoreID     string   `json:"store_id"`
	StoreName   string   `json:"store_name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Stock       int      `json:"stock"`
	Category    string   `json:"category"`
	ImagePath   string   `json:"image_path"`
	Gallery     []string `json:"gallery"`
}

type ProductDetailResponse struct {
	ID               string           `json:"id"`
	SellerID         string           `json:"seller_id"`
	StoreID          string           `json:"store_id"`
	StoreName        string           `json:"store_name"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	Price            float64          `json:"price"`
	Stock            int              `json:"stock"`
	Category         string           `json:"category"`
	ImagePath        string           `json:"image_path"`
	Gallery          []string         `json:"gallery"`
	AISummary        string           `json:"ai_summary"`
	AISentimentBadge string           `json:"ai_sentiment_badge"`
	RecentReviews    []ReviewResponse `json:"recent_reviews"`
}

type ProductAskRequest struct {
	Question string `json:"question" validate:"required"`
}

type ProductAskResponse struct {
	Answer string `json:"answer"`
}

type ProductLightweight struct {
	ID               string `json:"id" db:"id"`
	Title            string `json:"title" db:"title"`
	Category         string `json:"category" db:"category"`
	ShortDescription string `json:"short_description" db:"short_description"`
}

type UpdateProductFullRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Stock       int      `json:"stock"`
	Category    string   `json:"category"`
	Images      []string `json:"images"`
	KeptImages  []string `json:"kept_images"`
}

// --- Interfaces ---

type ProductRepository interface {
	Fetch(ctx context.Context, category, searchQuery string, limit int, cursorTime string) ([]Product, error)
	FetchByStoreId(ctx context.Context, storeID string) ([]Product, error)
	GetByID(ctx context.Context, id string) (*Product, error)
	Store(ctx context.Context, p *Product) error
	UpdatePriceAndStock(ctx context.Context, productID string, storeID string, price float64, stock int) error
	Delete(ctx context.Context, id string, storeID string) error
	GetLowStockProducts(ctx context.Context, storeID string, limit int) ([]Product, error)
	UpdateAIInsights(ctx context.Context, productID string, summary string, badge string, embedding []float32) error
	GetBestsellers(ctx context.Context, limit int) ([]Product, error)
	GetCategories(ctx context.Context) ([]string, error)
	GetPendingAIUpdates(ctx context.Context, limit int) ([]Product, error)
	GetByIDs(ctx context.Context, ids []string) ([]Product, error)
	UpdateFull(ctx context.Context, p *Product) error
	SearchBySimilarity(ctx context.Context, embedding []float32, limit int, maxPrice float64, inStock bool) ([]Product, error)
}

type ProductUsecase interface {
	Fetch(ctx context.Context, category, searchQuery string, limit int, cursorTime string) ([]ProductResponse, error)
	FetchBySeller(ctx context.Context, sellerID string) ([]ProductResponse, error)
	Store(ctx context.Context, sellerID string, req *CreateProductRequest) (*ProductResponse, error)
	UpdatePriceAndStock(ctx context.Context, sellerID string, productID string, req *UpdateProductRequest) (*ProductResponse, error)
	Delete(ctx context.Context, sellerID string, productID string) error
	GetProductDetail(ctx context.Context, id string) (*ProductDetailResponse, error)
	AskQuestion(ctx context.Context, productID string, req *ProductAskRequest) (*ProductAskResponse, error)
	GetBestsellers(ctx context.Context) ([]ProductResponse, error)
	GetCategories(ctx context.Context) ([]string, error)
	UpdateFull(ctx context.Context, sellerID string, productID string, req *UpdateProductFullRequest) (*ProductResponse, error)
	GenerateUploadURL(ctx context.Context, filename string) (string, string, error)
}
