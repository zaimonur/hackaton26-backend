package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	handler "drewisy/internal/delivery/http"
	"drewisy/internal/infrastructure/ai"
	"drewisy/internal/infrastructure/storage"
	"drewisy/internal/repository/postgres"
	"drewisy/internal/usecase"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
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

	// 1. Ortam Değişkenleri
	if err := godotenv.Load(); err != nil {
		log.Println("Uyarı: .env dosyası okunamadı, sistem environment veya default değerler kullanılacak.")
	}

	// 2. Veritabanı DSN Hazırlığı
	host := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5433")
	user := getEnv("DB_USER", "postgres")
	pass := getEnv("DB_PASSWORD", "secret")
	dbName := getEnv("DB_NAME", "drewisy")
	ssl := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, dbPort, user, pass, dbName, ssl,
	)

	// 3. Veritabanı Bağlantısı
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("Fatal: Veritabanı bağlantısı reddedildi. DSN: %s | Hata: %v", dsn, err)
	}
	defer db.Close()

	log.Println("PostgreSQL bağlantısı başarılı!")

	// 4. Echo Sunucusu ve Genel Middleware'ler
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.CORS())
	e.Static("/static", "uploads")

	// 5. Dependency Injection (Bağımlılık Enjeksiyonları)
	fileStorage := storage.NewLocalStorage("uploads")

	userRepo := postgres.NewUserRepository(db)
	userUsecase := usecase.NewUserUsecase(userRepo)

	// YENİ: Store Dependency Injection
	storeRepo := postgres.NewStoreRepository(db)
	storeUsecase := usecase.NewStoreUsecase(storeRepo)

	// GÜNCELLEME: Product Dependency Injection (storeRepo içeri enjekte edildi)
	productRepo := postgres.NewProductRepository(db)
	productUsecase := usecase.NewProductUsecase(productRepo, storeRepo, fileStorage)

	//Gemini AI DI
	geminiApiKey := getEnv("GEMINI_API_KEY", "")
	if geminiApiKey == "" {
		log.Println("UYARI: GEMINI_API_KEY bulunamadı! AI özellikleri çalışmayacaktır.")
	}
	aiService := ai.NewGeminiService(geminiApiKey)
	aiUsecase := usecase.NewAIUsecase(aiService)

	// 6. API Yönlendirmeleri (Routing)
	v1 := e.Group("/api/v1")

	// Kullanıcı işlemleri
	handler.NewUserHandler(v1, userUsecase)

	// YENİ: Mağaza işlemleri
	handler.NewStoreHandler(v1, storeUsecase)

	// Ürün işlemleri
	handler.NewProductHandler(v1, productUsecase)

	//AI Handler Yönlendirmesi
	handler.NewAIHandler(v1, aiUsecase)

	// 7. Sunucuyu Başlatma (Graceful Shutdown ile)
	appPort := getEnv("PORT", "8080")
	go func() {
		if err := e.Start(":" + appPort); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Sunucu başlatılamadı: %v", err)
		}
	}()

	// 8. Güvenli Kapatma Sinyalleri
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
