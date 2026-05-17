package domain

import (
	"context"
	"time"
)

// Entity (DB Model)
type Notification struct {
	ID          string    `db:"id"`
	UserID      string    `db:"user_id"`
	Type        string    `db:"type"`         // 'NEW_MESSAGE' veya 'ORDER_UPDATE'
	ReferenceID *string   `db:"reference_id"` // Nullable UUID
	Title       string    `db:"title"`
	Body        string    `db:"body"`
	IsRead      bool      `db:"is_read"`
	CreatedAt   time.Time `db:"created_at"`
}

// DTOs
type NotificationResponse struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	ReferenceID *string   `json:"reference_id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	IsRead      bool      `json:"is_read"`
	CreatedAt   time.Time `json:"created_at"`
}

// WebSocket Event DTO
type WSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Interfaces
type NotificationRepository interface {
	Create(ctx context.Context, n *Notification) error
	GetByUserID(ctx context.Context, userID string) ([]Notification, error)
	MarkAsRead(ctx context.Context, id, userID string) error
}

type NotificationUsecase interface {
	GetMyNotifications(ctx context.Context, userID string) ([]NotificationResponse, error)
	MarkAsRead(ctx context.Context, id, userID string) error
}
