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
	"drewisy/internal/infrastructure/websocket"
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
	defer db.Close()

	log.Println("PostgreSQL bağlantısı başarılı!")

	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.CORS())
	e.Static("/static", "uploads")

	// WebSocket Hub
	hub := websocket.NewHub()

	// Repolar
	fileStorage := storage.NewLocalStorage("uploads")
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

	//  ProductUsecase bağımlılık zinciri tamamlandı
	productUsecase := usecase.NewProductUsecase(productRepo, storeRepo, fileStorage, reviewRepo, aiService)

	orderUsecase := usecase.NewOrderUsecase(orderRepo, productRepo, notificationRepo, hub)
	reviewUsecase := usecase.NewReviewUsecase(reviewRepo, aiService, productRepo)

	// AI Usecase
	aiUsecase := usecase.NewAIUsecase(aiService, productRepo, reviewRepo, historyRepo)

	// Dashboard Usecase
	dashboardUsecase := usecase.NewDashboardUsecase(dashboardRepo, storeRepo, productRepo, reviewRepo, aiUsecase)

	//Mesaj ve Bildirim Usecaseleri
	messageUsecase := usecase.NewMessageUsecase(messageRepo, hub)
	notificationUsecase := usecase.NewNotificationUsecase(notificationRepo)

	// Routing
	v1 := e.Group("/api/v1")
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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
