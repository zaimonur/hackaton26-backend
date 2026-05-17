package http

import (
	"drewisy/internal/infrastructure/websocket"
	"net/http"

	gorillaws "github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Hackathon ortamı testleri kolaylaştırmak için CORS bypass.
	},
}

type WSHandler struct {
	hub *websocket.Hub
}

func NewWSHandler(e *echo.Group, hub *websocket.Hub) {
	handler := &WSHandler{hub: hub}
	e.GET("/ws", handler.HandleWS) // Public gibi görünüp içeride query token validasyonu yapar
}

func (h *WSHandler) HandleWS(c echo.Context) error {
	tokenStr := c.QueryParam("token")
	if tokenStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Erişim reddedildi: Token bulunamadı"})
	}

	claims, err := parseToken(tokenStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Geçersiz kimlik bilgisi"})
	}

	// gorillaws.Upgrader kullanılıyor
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	client := &websocket.Client{
		Hub:    h.hub,
		Conn:   conn,
		UserID: userID,
		Send:   make(chan []byte, 256),
	}

	h.hub.Register(client)

	// Goroutine Pump motorları devreye alınıyor
	go client.WritePump()
	go client.ReadPump()

	return nil
}
