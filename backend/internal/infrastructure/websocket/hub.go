package websocket

import (
	"drewisy/internal/domain"
	"encoding/json"
	"log"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client // userID -> *Client
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Multi-device session override: Aynı UserID ile yeni bağlantı gelirse eskini temizle.
	if oldClient, exists := h.clients[client.UserID]; exists {
		close(oldClient.Send)
	}

	h.clients[client.UserID] = client
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Race condition koruması: Sadece mevcut aktif bağlantı bu client ise sil.
	if currentClient, ok := h.clients[client.UserID]; ok && currentClient == client {
		delete(h.clients, client.UserID)
		close(client.Send)
	}
}

func (h *Hub) SendToUser(userID string, event domain.WSEvent) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		// Kullanıcı offline. (Mesaj DB'de var, online olduğunda REST ile çeker)
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("WS Event Marshal Error: %v", err)
		return
	}

	select {
	case client.Send <- payload:
	default:
		// Kanal doluysa veya bloke olduysa dead-lock önlemi olarak bağlantıyı kopar
		h.Unregister(client)
	}
}
