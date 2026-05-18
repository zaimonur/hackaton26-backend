package http

import (
	"drewisy/internal/infrastructure/websocket"
	"errors"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	gorillaws "github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS bypass
	},
}

type WSHandler struct {
	hub *websocket.Hub
}

func NewWSHandler(e *echo.Group, hub *websocket.Hub) {
	handler := &WSHandler{hub: hub}
	// Endpoint artık query param bekliyor: /ws?token=<JWT>
	e.GET("/ws", handler.HandleWS)
}

func (h *WSHandler) HandleWS(c echo.Context) error {
	// 1. PRE-UPGRADE AUTHENTICATION (Bağlantı yükseltilmeden önce kontrol)
	tokenString := c.QueryParam("token")
	if tokenString == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Token eksik"})
	}

	claims, err := parseWSToken(tokenString, os.Getenv("JWT_SECRET"))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Geçersiz veya süresi dolmuş token"})
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Geçersiz kimlik bilgisi"})
	}

	// 2. KİMLİK DOĞRULANDI -> Bağlantıyı HTTP'den WebSocket'e güvenle yükselt
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err // Yükseltme hatası
	}

	// 3. İstemciyi doğrudan Hub'a kaydet (Zaman aşımı / bekleme süresi riskleri yok edildi)
	client := &websocket.Client{
		Hub:    h.hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		UserID: userID,
	}

	client.Hub.Register(client)

	// Dinleme ve yazma döngülerini asenkron başlat
	go client.WritePump()
	go client.ReadPump()

	return nil
}

// WebSocket bağlantısı için JWT token doğrulama fonksiyonu
func parseWSToken(tokenString string, secret string) (jwt.MapClaims, error) {
	if secret == "" {
		return nil, errors.New("sunucu yapılandırma hatası")
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("geçersiz veya süresi dolmuş token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("token payload okunamadı")
	}

	return claims, nil
}
