package domain

import (
	"context"
	"time"
)

// Entity (DB Model)
type Message struct {
	ID         string    `db:"id"`
	SenderID   string    `db:"sender_id"`
	ReceiverID string    `db:"receiver_id"`
	Content    string    `db:"content"`
	CreatedAt  time.Time `db:"created_at"`
}

// DTOs
type SendMessageRequest struct {
	ReceiverID string `json:"receiver_id" validate:"required"`
	Content    string `json:"content" validate:"required"`
}

type MessageResponse struct {
	ID         string    `json:"id"`
	SenderID   string    `json:"sender_id"`
	ReceiverID string    `json:"receiver_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// Interfaces
type MessageRepository interface {
	Create(ctx context.Context, msg *Message) error
	GetChatHistory(ctx context.Context, user1ID, user2ID string) ([]Message, error)
}

type MessageUsecase interface {
	SendMessage(ctx context.Context, senderID string, req *SendMessageRequest) (*MessageResponse, error)
	GetChatHistory(ctx context.Context, currentUserID, targetUserID string) ([]MessageResponse, error)
}
