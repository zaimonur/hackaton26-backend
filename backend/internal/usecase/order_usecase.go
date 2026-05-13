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
