package usecase

import (
	"context"
	"drewisy/internal/domain"
	"drewisy/internal/infrastructure/websocket"
	"errors"
)

type orderUsecase struct {
	orderRepo        domain.OrderRepository
	productRepo      domain.ProductRepository
	notificationRepo domain.NotificationRepository
	cacheRepo        domain.CacheRepository
	hub              *websocket.Hub
}

func NewOrderUsecase(or domain.OrderRepository, pr domain.ProductRepository, nr domain.NotificationRepository, cr domain.CacheRepository, hub *websocket.Hub) domain.OrderUsecase {
	return &orderUsecase{
		orderRepo:        or,
		productRepo:      pr,
		notificationRepo: nr,
		cacheRepo:        cr,
		hub:              hub,
	}
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

		// 1. STOK REZERVASYONU (REDIS)
		status, err := u.cacheRepo.DecrementStock(ctx, item.ProductID, item.Quantity)
		if err != nil {
			return nil, errors.New("stok kontrolü sırasında bir hata oluştu")
		}

		// Cache Miss: Redis'te stok verisi henüz yoksa DB'den çekip Redis'e yükle ve tekrar dene
		if status == -1 {
			product, err := u.productRepo.GetByID(ctx, item.ProductID)
			if err != nil {
				return nil, errors.New("ürün bulunamadı")
			}
			// DB'deki stoğu Redis'e yaz (Lazy Loading)
			u.cacheRepo.Set(ctx, "product:stock:"+product.ID, product.Stock, 0) // 0 = Kalıcı

			// Tekrar düşmeyi dene
			status, _ = u.cacheRepo.DecrementStock(ctx, item.ProductID, item.Quantity)
		}

		if status == 0 {
			return nil, errors.New("üzgünüz, bu ürünün stoğu tükendi veya yetersiz")
		}

		// Stok ayrıldı, fiyatı DB'den güvenli şekilde çekelim (Fiyat bilgisini de ileride Redis'e alabilirsin)
		product, _ := u.productRepo.GetByID(ctx, item.ProductID)
		totalAmount += product.Price * float64(item.Quantity)

		orderItems = append(orderItems, domain.OrderItem{
			ProductID: product.ID,
			Quantity:  item.Quantity,
			UnitPrice: product.Price,
		})
	}

	order := &domain.Order{
		CustomerID:  customerID,
		TotalAmount: totalAmount,
		Status:      "pending", // Önce pending, DB'ye yazılınca onaylanacak
	}

	// 2. SİPARİŞİ KUYRUĞA GÖNDER (Asenkron İşlem Paketi)
	type OrderPayload struct {
		Order domain.Order       `json:"order"`
		Items []domain.OrderItem `json:"items"`
	}

	payload := OrderPayload{Order: *order, Items: orderItems}

	err := u.cacheRepo.PublishToQueue(ctx, "order_queue", payload)
	if err != nil {
		// Kuyruğa yazılamazsa, alınan stokları iade eden bir telafi (compensation) mekanizması eklenmeli.
		// Hackathon için şimdilik doğrudan hata fırlatıyoruz.
		return nil, errors.New("sipariş alınamadı, lütfen tekrar deneyin")
	}

	// 3. MÜŞTERİYE ANINDA YANIT DÖN
	return &domain.OrderResponse{
		OrderID:     "İşleniyor...", // DB ID'si henüz oluşmadı
		TotalAmount: totalAmount,
		Status:      "processing", // Müşteriye işleniyor diyoruz
	}, nil
}

// FetchSellerOrders: Satıcının siparişlerini getirir (Business Logic)
func (u *orderUsecase) FetchSellerOrders(ctx context.Context, sellerID string) ([]domain.SellerOrderResponse, error) {
	return u.orderRepo.FetchBySellerId(ctx, sellerID)
}

// UpdateOrderStatus: Satıcının sipariş statüsünü günceller (Business Logic & Validation)
func (u *orderUsecase) UpdateOrderStatus(ctx context.Context, sellerID string, orderID string, req *domain.UpdateOrderStatusRequest) error {
	validStatuses := map[string]bool{
		"pending":   true,
		"shipped":   true,
		"delivered": true,
		"cancelled": true,
	}

	if !validStatuses[req.Status] {
		return errors.New("geçersiz sipariş statüsü")
	}

	// Repo üzerinden güncellenen satırdan customer_id anında yakalanır
	customerID, err := u.orderRepo.UpdateStatus(ctx, orderID, req.Status, sellerID)
	if err != nil {
		return err
	}

	// Müşteri için kalıcı bildirim kaydı oluşturulur
	notification := &domain.Notification{
		UserID:      customerID,
		Type:        "ORDER_UPDATE",
		ReferenceID: &orderID,
		Title:       "Sipariş Durumunuz Güncellendi",
		Body:        "Siparişinizin yeni durumu: " + req.Status,
	}

	if err := u.notificationRepo.Create(ctx, notification); err != nil {
		return err
	}

	// Canlı akış için anlık WebSocket Event tetiklenir
	wsResp := domain.NotificationResponse{
		ID:          notification.ID,
		Type:        notification.Type,
		ReferenceID: notification.ReferenceID,
		Title:       notification.Title,
		Body:        notification.Body,
		IsRead:      notification.IsRead,
		CreatedAt:   notification.CreatedAt,
	}

	u.hub.SendToUser(customerID, domain.WSEvent{
		Type:    "ORDER_UPDATE",
		Payload: wsResp,
	})

	return nil
}

func (u *orderUsecase) FetchCustomerOrders(ctx context.Context, customerID string) ([]domain.CustomerOrderResponse, error) {
	return u.orderRepo.FetchByCustomerId(ctx, customerID)
}
