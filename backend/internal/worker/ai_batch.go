package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"drewisy/internal/domain"

	"golang.org/x/time/rate"
)

// StartAIBatchProcessor: Günde 1 kez (veya belirlenen aralıklarla) çalışarak AI özetlerini ve RAG vektörlerini topluca günceller.
func StartAIBatchProcessor(ctx context.Context, aiUsecase domain.AIUsecase, aiService domain.AIService, productRepo domain.ProductRepository) {
	ticker := time.NewTicker(24 * time.Hour)
	log.Println("🤖 AI Batch Processor (Gece Vardiyası) Başlatıldı...")

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("AI Batch Processor durduruluyor...")
				return
			case <-ticker.C:
				log.Println("⏳ Günlük AI Özetleme ve RAG İndeksleme görevi başladı...")

				products, err := productRepo.GetPendingAIUpdates(ctx, 500)
				if err != nil {
					log.Printf("Ürünler çekilemedi: %v", err)
					continue
				}

				if len(products) == 0 {
					log.Println("✅ Güncellenecek yeni ürün bulunamadı. AI senkronizasyonu tamam.")
					continue
				}

				limiter := rate.NewLimiter(rate.Limit(1), 1) // Saniyede 1 istek

				productCh := make(chan domain.Product, len(products))
				var wg sync.WaitGroup

				// Consumer (Workers): 5 Goroutine
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						for p := range productCh {
							if err := limiter.Wait(ctx); err != nil {
								return
							}

							// Ürünün güncel yorumlarını özetle
							summary, err := aiUsecase.SummarizeProductReviews(ctx, p.ID)
							if err != nil {
								log.Printf("[Worker-%d] Ürün %s özet üretilemedi: %v", workerID, p.ID, err)
								continue
							}

							richText := fmt.Sprintf("Kategori: %s, Başlık: %s, Açıklama: %s, Müşteri Deneyimi ve AI Özeti: %s",
								p.Category, p.Title, p.Description, summary)

							// Vektörü hesapla
							newEmbedding, err := aiService.CreateEmbedding(ctx, richText)
							if err != nil {
								log.Printf("[Worker-%d] Ürün %s için embedding üretilemedi: %v", workerID, p.ID, err)
								continue
							}

							//  DB'ye yaz
							err = productRepo.UpdateAIInsights(ctx, p.ID, summary, "Yapay Zeka Analizi", newEmbedding)
							if err != nil {
								log.Printf("[Worker-%d] Ürün %s DB'ye yazılamadı: %v", workerID, p.ID, err)
							} else {
								log.Printf("[Worker-%d] ✅ Ürün %s başarıyla RAG için indekslendi.", workerID, p.ID)
							}
						}
					}(i)
				}

				// Producer: Ürünleri kanala gönder
				for _, p := range products {
					productCh <- p
				}
				close(productCh)

				wg.Wait()
				log.Println("✅ Günlük AI Özetleme ve RAG İndeksleme görevi tamamlandı.")
			}
		}
	}()
}
