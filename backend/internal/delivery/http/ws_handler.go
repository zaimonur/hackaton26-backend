package http

import (
	"drewisy/internal/domain"
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
	hub       *websocket.Hub
	aiUsecase domain.AIUsecase // DI ile eklendi
}

// Faz 1, 2 ve 3 için Event DTO'ları
type WSMessagePayload struct {
	Content string `json:"content"`
}

type WSStreamChunkPayload struct {
	MessageID string `json:"message_id"`
	Chunk     string `json:"chunk"`
	IsFinal   bool   `json:"is_final"`
}

type WSEventOut struct {
	Event   string      `json:"event"`
	Payload interface{} `json:"payload"`
}

// NewWSHandler artık 3 parametre alıyor (main.go'daki çağrı ile eşleşti)
func NewWSHandler(e *echo.Group, hub *websocket.Hub, aiUsecase domain.AIUsecase) {
	handler := &WSHandler{hub: hub, aiUsecase: aiUsecase}
	e.GET("/ws", handler.HandleWS)
}

func (h *WSHandler) HandleWS(c echo.Context) error {
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

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	client := &websocket.Client{
		Hub:       h.hub,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		UserID:    userID,
		AIUsecase: h.aiUsecase, // Client'a AI yeteneğini doğrudan veriyoruz
	}

	client.Hub.Register(client)

	go client.WritePump()
	go client.ReadPump()

	return nil
}

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
