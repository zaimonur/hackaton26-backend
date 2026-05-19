package websocket

import (
	"context"
	"drewisy/internal/domain"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	UserID    string
	Send      chan []byte
	AIUsecase domain.AIUsecase // DI ile main.go'dan enjekte edilecek
}

type WSEventIn struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

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

// safeSend: Kanal kapalı olsa dahi panic fırlatıp sunucuyu çökertmez.
func (c *Client) safeSend(payload []byte) bool {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WS SafeSend] Recovered from panic (UserID: %s): %v", c.UserID, r)
		}
	}()
	c.Send <- payload
	return true
}

// ReadPump: Client'tan gelen WebSocket isteklerini dinler
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var incomingEvent WSEventIn
		if err := json.Unmarshal(messageBytes, &incomingEvent); err == nil {
			if incomingEvent.Event == "chat_message" {
				go c.handleAIChatMessage(incomingEvent.Payload)
			}
		}
	}
}

// WritePump: Hub veya istemci içinden Send kanalına basılan verileri tarayıcıya/iOS'a fırlatır
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleAIChatMessage: Kullanıcı promptunu yakalayıp AI akış hattını çalıştırır
func (c *Client) handleAIChatMessage(payload []byte) {
	var msgPayload WSMessagePayload
	if err := json.Unmarshal(payload, &msgPayload); err != nil {
		return
	}

	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	ctx := context.Background()

	if c.AIUsecase == nil {
		log.Printf("[WS Error] AIUsecase enjekte edilmemiş!")
		return
	}

	// AI Usecase Katmanından Go Channel stream'ini alıyoruz
	chunkChan, err := c.AIUsecase.StreamShoppingAssistant(ctx, c.UserID, msgPayload.Content)
	if err != nil {
		log.Printf("[AI Usecase Error] Stream başlatılamadı: %v", err)
		return
	}

	// Gelen kelimeleri parça parça iOS istemcisine fırlatıyoruz
	for chunk := range chunkChan {
		outEvent := WSEventOut{
			Event: "ai_stream_chunk",
			Payload: WSStreamChunkPayload{
				MessageID: msgID,
				Chunk:     chunk,
				IsFinal:   false,
			},
		}
		outBytes, _ := json.Marshal(outEvent)
		c.safeSend(outBytes)
	}

	// Akış başarıyla sonlandığında final paketini gönderiyoruz
	finalEvent := WSEventOut{
		Event: "ai_stream_chunk",
		Payload: WSStreamChunkPayload{
			MessageID: msgID,
			Chunk:     "",
			IsFinal:   true,
		},
	}
	finalBytes, _ := json.Marshal(finalEvent)
	c.safeSend(finalBytes)
}
