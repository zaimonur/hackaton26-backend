package http

import (
	"drewisy/internal/infrastructure/websocket"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

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
	e.GET("/ws", handler.HandleWS)
}

func (h *WSHandler) HandleWS(c echo.Context) error {
	// 1. URL'de token kontrolü YOK. Doğrudan bağlantıyı HTTP'den WebSocket'e yükseltiyoruz.
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	// 2. Güvenlik: İstemciye kimliğini kanıtlaması (Auth Frame) için 5 saniye süre veriyoruz.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 3. İlk mesajı oku
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil
	}

	// 4. Gelen ilk mesajın "auth" tipinde ve token içerip içermediğini kontrol et
	var authReq struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(msg, &authReq); err != nil || authReq.Type != "auth" || authReq.Token == "" {
		conn.WriteMessage(gorillaws.TextMessage, []byte(`{"error": "unauthorized: auth frame bekleniyor"}`))
		conn.Close()
		return nil
	}

	// 5. Token'ı doğrula
	claims, err := parseWSToken(authReq.Token, os.Getenv("JWT_SECRET"))
	if err != nil {
		conn.WriteMessage(gorillaws.TextMessage, []byte(`{"error": "unauthorized: geçersiz token"}`))
		conn.Close()
		return nil
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		conn.WriteMessage(gorillaws.TextMessage, []byte(`{"error": "unauthorized: geçersiz kimlik"}`))
		conn.Close()
		return nil
	}

	// 6. Kimlik doğrulama başarılı! Zaman aşımını (timeout) kaldır ve Hub'a kaydet
	conn.SetReadDeadline(time.Time{})
	client := &websocket.Client{
		Hub:    h.hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		UserID: userID,
	}

	client.Hub.Register(client)

	// Dinleme ve yazma döngülerini başlat
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
