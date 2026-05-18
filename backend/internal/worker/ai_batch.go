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

type SummaryJob struct {
	ProductID string
	Content   string
}

// StartAIBatchProcessor: Günde 1 kez çalışarak AI özetlerini ve RAG vektörlerini topluca günceller.
// DİKKAT: Vektör üretebilmek için parametrelere 'aiService domain.AIService' eklendi!
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

				products, err := productRepo.GetAllForAI(ctx)
				if err != nil {
					log.Printf("Ürünler çekilemedi: %v", err)
					continue
				}

				// Rate Limiter: Saniyede 1 istek, Burst 1 (Strict Token Bucket)
				limiter := rate.NewLimiter(rate.Limit(1), 1)
				productCh := make(chan domain.ProductLightweight, len(products))
				var wg sync.WaitGroup

				// 1. Consumer (Workers): 5 Goroutine başlat
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						for p := range productCh {
							// Rate limit dolana kadar güvenli bloklama
							if err := limiter.Wait(ctx); err != nil {
								return
							}

							// 1. Ürünün güncel yorumlarını özetle
							summary, err := aiUsecase.SummarizeProductReviews(ctx, p.ID)
							if err != nil {
								log.Printf("[Worker-%d] Ürün %s özet üretilemedi: %v", workerID, p.ID, err)
								continue
							}

							// 2. Ürünün detaylı verilerini al (Kategori, Başlık, Açıklama için)
							fullProduct, err := productRepo.GetByID(ctx, p.ID)
							if err != nil {
								log.Printf("[Worker-%d] Ürün %s detayları alınamadı: %v", workerID, p.ID, err)
								continue
							}

							// 3. GERÇEK RAG MİMARİSİ: Zengin Metin (Rich Text) Oluştur
							richText := fmt.Sprintf("Kategori: %s, Başlık: %s, Açıklama: %s, Müşteri Deneyimi ve AI Özeti: %s",
								fullProduct.Category, fullProduct.Title, fullProduct.Description, summary)

							// 4. Bu zengin metnin vektörünü hesapla
							newEmbedding, err := aiService.CreateEmbedding(ctx, richText)
							if err != nil {
								log.Printf("[Worker-%d] Ürün %s için embedding üretilemedi: %v", workerID, p.ID, err)
								continue
							}

							// 5. DB'ye hem AI Özetini hem de YENİ Vektörü yaz
							err = productRepo.UpdateAIInsights(ctx, p.ID, summary, "Yapay Zeka Analizi", newEmbedding)
							if err != nil {
								log.Printf("[Worker-%d] Ürün %s DB'ye yazılamadı: %v", workerID, p.ID, err)
							} else {
								log.Printf("[Worker-%d] ✅ Ürün %s başarıyla RAG için indekslendi.", workerID, p.ID)
							}
						}
					}(i)
				}

				// 2. Producer: Ürünleri kanala gönder
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
