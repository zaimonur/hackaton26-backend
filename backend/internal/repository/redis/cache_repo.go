package redis

import (
	"context"
	"encoding/json"
	"time"

	"drewisy/internal/domain"

	"github.com/redis/go-redis/v9"
)

type redisCacheRepo struct {
	client *redis.Client
}

func NewRedisCacheRepository(addr string) domain.CacheRepository {
	client := redis.NewClient(&redis.Options{
		Addr: addr, // Örn: "localhost:6379"
	})
	return &redisCacheRepo{client: client}
}

func (r *redisCacheRepo) Set(ctx context.Context, key string, value interface{}, exp time.Duration) error {
	// Gelen struct'ı JSON'a çevirip Redis'e yazıyoruz
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, bytes, exp).Err()
}

func (r *redisCacheRepo) Get(ctx context.Context, key string, dest interface{}) error {
	// Redis'ten JSON'u okuyup ilgili Struct'a mapliyoruz
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err // redis.Nil dönerse key yok demektir
	}
	return json.Unmarshal([]byte(val), dest)
}

func (r *redisCacheRepo) DecrementStock(ctx context.Context, productID string, quantity int) (int, error) {
	script := redis.NewScript(`
		local current_stock = redis.call('GET', KEYS[1])
		if current_stock == false then
			return -1 -- Stok Redis'te yok (Cache Miss)
		end
		if tonumber(current_stock) >= tonumber(ARGV[1]) then
			redis.call('DECRBY', KEYS[1], ARGV[1])
			return 1 -- Başarılı (Stok ayrıldı)
		else
			return 0 -- Yetersiz Stok
		end
	`)

	key := "product:stock:" + productID
	result, err := script.Run(ctx, r.client, []string{key}, quantity).Result()
	if err != nil {
		return 0, err
	}

	return int(result.(int64)), nil
}

// Kuyruğa Mesaj Bırakma (Producer)
func (r *redisCacheRepo) PublishToQueue(ctx context.Context, queueName string, payload interface{}) error {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// LPUSH: Kuyruğun solundan ekle
	return r.client.LPush(ctx, queueName, bytes).Err()
}

// Kuyruktan Mesaj Okuma (Consumer - Blocking Pop)
func (r *redisCacheRepo) ConsumeFromQueue(ctx context.Context, queueName string) (string, error) {
	// BRPOP: Kuyruğun sağından oku, boşsa yeni mesaj gelene kadar bekle (0 = sonsuz bekle)
	result, err := r.client.BRPop(ctx, 0, queueName).Result()
	if err != nil {
		return "", err
	}
	// BRPop iki değer döner: [kuyruk_adi, mesaj]
	return result[1], nil
}

// PublishToDLQ: İşlenemeyen sorunlu mesajları (zehirli mesajlar) Dead Letter Queue'ya atar
func (r *redisCacheRepo) PublishToDLQ(ctx context.Context, queueName string, payload interface{}) error {
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// LPUSH ile mesajı DLQ kuyruğuna ekliyoruz.
	// (Mantık olarak PublishToQueue ile aynıdır ancak mimari olarak ayrıştırmak Clean Architecture için önemlidir)
	return r.client.LPush(ctx, queueName, bytes).Err()
}

// 1. Güvenli Okuma (Taşıyarak oku)
func (r *redisCacheRepo) ConsumeReliably(ctx context.Context, sourceQueue, processingQueue string) (string, error) {
	// BRPOPLPUSH veya BLMOVE (Redis 6.2+) kullanır. Sağdan alır, sola ekler.
	result, err := r.client.BLMove(ctx, sourceQueue, processingQueue, "RIGHT", "LEFT", 0).Result()
	if err != nil {
		return "", err
	}
	return result, nil
}

// 2. Onaylama (İşlem bitince sil)
func (r *redisCacheRepo) AckMessage(ctx context.Context, processingQueue string, msg string) error {
	// İşlenen mesajı processing kuyruğundan sil
	return r.client.LRem(ctx, processingQueue, 1, msg).Err()
}
