package usecase

import (
	"context"
	"drewisy/internal/domain"
	"errors"
)

type orderUsecase struct {
	orderRepo   domain.OrderRepository
	productRepo domain.ProductRepository
}

func NewOrderUsecase(or domain.OrderRepository, pr domain.ProductRepository) domain.OrderUsecase {
	return &orderUsecase{orderRepo: or, productRepo: pr}
}

func (u *orderUsecase) CreateOrder(ctx context.Context, customerID string, req *domain.CreateOrderRequest) (*domain.OrderResponse, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("sipariş oluşturmak için en az bir ürün eklenmelidir")
	}

	var totalAmount float64
	var orderItems []domain.OrderItem

	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, errors.New("ürün adedi 0'dan büyük olmalıdır")
		}

		// Güvenlik: Fiyatı DB'den çekiyoruz
		product, err := u.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, errors.New("ürün bulunamadı veya geçersiz")
		}

		totalAmount += product.Price * float64(item.Quantity)

		orderItems = append(orderItems, domain.OrderItem{
			ProductID: product.ID,
			Quantity:  item.Quantity,
			UnitPrice: product.Price, // Güvenli fiyat kaydı
		})
	}

	order := &domain.Order{
		CustomerID:  customerID,
		TotalAmount: totalAmount,
		Status:      "pending",
	}

	if err := u.orderRepo.Create(ctx, order, orderItems); err != nil {
		return nil, err
	}

	return &domain.OrderResponse{
		OrderID:     order.ID,
		TotalAmount: order.TotalAmount,
		Status:      order.Status,
	}, nil
}

// FetchSellerOrders: Satıcının siparişlerini getirir (Business Logic)
func (u *orderUsecase) FetchSellerOrders(ctx context.Context, sellerID string) ([]domain.SellerOrderResponse, error) {
	return u.orderRepo.FetchBySellerId(ctx, sellerID)
}

// UpdateOrderStatus: Satıcının sipariş statüsünü günceller (Business Logic & Validation)
func (u *orderUsecase) UpdateOrderStatus(ctx context.Context, sellerID string, orderID string, req *domain.UpdateOrderStatusRequest) error {
	// 1. İş Kuralları (Business Rule) Validasyonu: Statü sadece belirli değerler alabilir
	validStatuses := map[string]bool{
		"pending":   true,
		"shipped":   true,
		"delivered": true,
		"cancelled": true,
	}

	if !validStatuses[req.Status] {
		return errors.New("geçersiz sipariş statüsü")
	}

	// 2. Repository'e yönlendir (Güvenlik IDOR kontrolü SQL tarafında yapılacak)
	return u.orderRepo.UpdateStatus(ctx, orderID, req.Status, sellerID)
}
