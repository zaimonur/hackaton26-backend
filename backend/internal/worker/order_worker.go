package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"drewisy/internal/domain"
)

func StartOrderProcessor(ctx context.Context, cacheRepo domain.CacheRepository, orderRepo domain.OrderRepository) {
	log.Println("Order Worker Başlatıldı. Güvenilir Kuyruk (Reliable Queue) dinleniyor...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Order Worker durduruluyor...")
				return
			default:
				// 1. GÜVENLİ OKUMA: Mesajı ana kuyruktan alır almaz processing (işleniyor) kuyruğuna taşır.
				// Worker aniden çökerse mesaj processing kuyruğunda güvende kalır.
				msg, err := cacheRepo.ConsumeReliably(ctx, "order_queue", "order_processing_queue")
				if err != nil {
					log.Printf("Kuyruk okuma hatası: %v", err)
					continue
				}

				type OrderPayload struct {
					Order domain.Order       `json:"order"`
					Items []domain.OrderItem `json:"items"`
				}

				var payload OrderPayload
				if err := json.Unmarshal([]byte(msg), &payload); err != nil {
					log.Printf("Geçersiz sipariş payload'ı (Çöpe atılıyor): %v", err)
					// Parse edilemeyen hatalı formatlı veriyi sonsuza dek tutmamak için ACK ile siliyoruz
					_ = cacheRepo.AckMessage(ctx, "order_processing_queue", msg)
					continue
				}

				// 3. DB YAZMA VE EXPONENTIAL BACKOFF RETRY
				maxRetries := 3
				backoff := 1 * time.Second
				success := false

				for attempt := 1; attempt <= maxRetries; attempt++ {
					err = orderRepo.CreateOrderTx(ctx, &payload.Order, payload.Items)
					if err == nil {
						success = true
						break
					}

					log.Printf("⚠️ DB yazma başarısız (Deneme %d/%d). Hata: %v. %v sonra yeniden denenecek...", attempt, maxRetries, err, backoff)

					select {
					case <-ctx.Done():
						log.Println("Worker shutdown sinyali aldı, retry mekanizması iptal ediliyor.")
						return
					case <-time.After(backoff):
					}

					backoff *= 2 // Üstel artış (1s -> 2s -> 4s)
				}

				// 4. SONUÇ VE ACKNOWLEDGE (ONAYLAMA) İŞLEMLERİ
				if success {
					log.Printf("✅ Sipariş DB'ye başarıyla kaydedildi: Tutar: %.2f", payload.Order.TotalAmount)
					// DB'ye yazıldığı için mesajı güvenle processing kuyruğundan silebiliriz
					_ = cacheRepo.AckMessage(ctx, "order_processing_queue", msg)
				} else {
					log.Printf("❌ Kritik: Sipariş %d deneme sonrası kurtarılamadı. DLQ kuyruğuna aktarılıyor.", maxRetries)

					// Mesajı Dead Letter Queue'ya taşı
					if dlqErr := cacheRepo.PublishToDLQ(ctx, "order_queue_dlq", payload); dlqErr != nil {
						log.Printf("🚨 FATAL ERROR: Mesaj DLQ'ya da yazılamadı! Olası veri kaybı: %v", dlqErr)
					} else {
						// DLQ'ya başarıyla aktarıldıysa, processing kuyruğundan sil
						_ = cacheRepo.AckMessage(ctx, "order_processing_queue", msg)
					}
				}
			}
		}
	}()
}
