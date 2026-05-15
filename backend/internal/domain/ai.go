package domain

import "context"

// DTO: Yapay Zeka'dan açıklama istemek için kullanılacak request
type GenerateDescriptionRequest struct {
	Title    string `json:"title" validate:"required"`
	Category string `json:"category" validate:"required"`
	Keywords string `json:"keywords"`
}

// DTO: Yapay Zeka'nın döndüreceği response
type GenerateDescriptionResponse struct {
	GeneratedDescription string `json:"generated_description"`
}

type SmartSearchRequest struct {
	Query string `json:"query" validate:"required"`
}

type SmartSearchResponse struct {
	Products []ProductResponse `json:"products"`
}

// Infrastructure (Altyapı) Katmanı İçin Arayüz
type AIService interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
	SmartSearch(ctx context.Context, catalogJSON string, userQuery string) ([]string, error)
}

// Usecase (İş Mantığı) Katmanı İçin Arayüz
type AIUsecase interface {
	GenerateDescription(ctx context.Context, req *GenerateDescriptionRequest) (*GenerateDescriptionResponse, error)
	SmartSearch(ctx context.Context, req *SmartSearchRequest) (*SmartSearchResponse, error)
	SummarizeProductReviews(ctx context.Context, productID string) (string, error)
	GenerateDashboardSummary(ctx context.Context, salesData *SalesDashboardResponse, lowStock []Product, recentReviews []ReviewResponse) (string, error)
}
