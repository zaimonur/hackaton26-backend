package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	handler "drewisy/internal/delivery/http"
	"drewisy/internal/infrastructure/ai"
	"drewisy/internal/infrastructure/storage"
	"drewisy/internal/infrastructure/websocket"
	"drewisy/internal/repository/postgres"
	"drewisy/internal/usecase"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	if err := os.MkdirAll("uploads/products", os.ModePerm); err != nil {
		log.Fatalf("Klasör oluşturulamadı: %v", err)
	}

	if err := godotenv.Load(); err != nil {
		log.Println("Uyarı: .env dosyası okunamadı, sistem environment veya default değerler kullanılacak.")
	}

	host := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5433")
	user := getEnv("DB_USER", "postgres")
	pass := getEnv("DB_PASSWORD", "secret")
	dbName := getEnv("DB_NAME", "drewisy")
	ssl := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, dbPort, user, pass, dbName, ssl,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Fatal: Veritabanı bağlantısı reddedildi. DSN: %s | Hata: %v", dsn, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	defer db.Close()

	log.Println("PostgreSQL bağlantısı başarılı!")

	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.CORS())
	e.Static("/static", "uploads")

	// WebSocket Hub
	hub := websocket.NewHub()

	// Cloud-Native Storage Seçimi
	var fileStorage storage.FileStorage
	useS3 := getEnv("USE_S3", "false")

	if useS3 == "true" {
		s3Region := getEnv("AWS_REGION", "eu-central-1")
		s3Bucket := getEnv("AWS_BUCKET", "drewisy-assets")
		fs, err := storage.NewS3Storage(s3Region, s3Bucket)
		if err != nil {
			log.Fatalf("S3 bağlantısı kurulamadı: %v", err)
		}
		fileStorage = fs
		log.Println("☁️  Depolama Katmanı: AWS S3 / MinIO entegrasyonu aktif.")
	} else {
		fileStorage = storage.NewLocalStorage("uploads")
		log.Println("📁 Depolama Katmanı: Yerel Disk (LocalStorage) aktif.")
	}

	// Arka plan AI işlemlerini izleyecek sistem bekçisi (WaitGroup)
	var wg sync.WaitGroup

	// Repolar
	userRepo := postgres.NewUserRepository(db)
	storeRepo := postgres.NewStoreRepository(db)
	productRepo := postgres.NewProductRepository(db)
	orderRepo := postgres.NewOrderRepository(db)
	reviewRepo := postgres.NewReviewRepository(db)
	dashboardRepo := postgres.NewDashboardRepository(db)
	historyRepo := postgres.NewHistoryRepository(db)
	historyUsecase := usecase.NewHistoryUsecase(historyRepo)
	messageRepo := postgres.NewMessageRepository(db)
	notificationRepo := postgres.NewNotificationRepository(db)

	// AI Servisi
	geminiApiKey := getEnv("GEMINI_API_KEY", "")
	if geminiApiKey == "" {
		log.Println("UYARI: GEMINI_API_KEY bulunamadı! AI özellikleri çalışmayacaktır.")
	}
	aiService := ai.NewGeminiService(geminiApiKey)

	// Usecase'ler
	userUsecase := usecase.NewUserUsecase(userRepo)
	storeUsecase := usecase.NewStoreUsecase(storeRepo)

	productUsecase := usecase.NewProductUsecase(productRepo, storeRepo, fileStorage, reviewRepo, aiService)

	orderUsecase := usecase.NewOrderUsecase(orderRepo, productRepo, notificationRepo, hub)

	// wg bekçisini reviewUsecase'e enjekte ettik
	reviewUsecase := usecase.NewReviewUsecase(reviewRepo, aiService, productRepo, &wg)

	aiUsecase := usecase.NewAIUsecase(aiService, productRepo, reviewRepo, historyRepo)

	dashboardUsecase := usecase.NewDashboardUsecase(dashboardRepo, storeRepo, productRepo, reviewRepo, aiUsecase)

	messageUsecase := usecase.NewMessageUsecase(messageRepo, notificationRepo, hub)
	notificationUsecase := usecase.NewNotificationUsecase(notificationRepo)

	// Routing
	v1 := e.Group("/api/v1")
	//  Saniyede 10 istek limiti (Burst: 30)
	v1.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(10))))

	handler.NewUserHandler(v1, userUsecase)
	handler.NewStoreHandler(v1, storeUsecase)
	handler.NewProductHandler(v1, productUsecase)
	handler.NewAIHandler(v1, aiUsecase)
	handler.NewOrderHandler(v1, orderUsecase)
	handler.NewReviewHandler(v1, reviewUsecase)
	handler.NewDashboardHandler(v1, dashboardUsecase)
	handler.NewHistoryHandler(v1, historyUsecase)
	handler.NewMessageHandler(v1, messageUsecase)
	handler.NewNotificationHandler(v1, notificationUsecase)
	handler.NewWSHandler(v1, hub)

	appPort := getEnv("PORT", "8080")
	go func() {
		if err := e.Start(":" + appPort); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Sunucu başlatılamadı: %v", err)
		}
	}()

	// Graceful Shutdown Yakalama
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Kapanma sinyali alındı. Sunucu trafiğe kapatılıyor...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatal("Sunucu kapatılırken hata:", err)
	}

	// Arka plan işlemlerinin bitmesini bekle
	log.Println("Arka plandaki AI işlemlerinin (goroutines) bitmesi bekleniyor...")
	waitCh := make(chan struct{})
	go func() {
		wg.Wait() // Tüm sayaçlar 0 olana kadar engeller
		close(waitCh)
	}()

	select {
	case <-waitCh:
		log.Println("✅ Tüm arka plan işlemleri başarıyla tamamlandı. Sistem güvenle kapatılıyor.")
	case <-ctx.Done():
		log.Println("⚠️ Zaman aşımı: Bazı arka plan işlemleri tamamlanamadı. Sistem zorla kapatılıyor.")
	}
}
