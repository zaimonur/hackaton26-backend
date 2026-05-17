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

type InboxItemResponse struct {
	TargetID    string    `json:"target_id" db:"target_id"`
	TargetName  string    `json:"target_name" db:"target_name"`
	TargetRole  string    `json:"target_role" db:"target_role"`
	LastMessage string    `json:"last_message" db:"last_message"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Interfaces
type MessageRepository interface {
	Create(ctx context.Context, msg *Message) error
	GetChatHistory(ctx context.Context, user1ID, user2ID string) ([]Message, error)
	GetInbox(ctx context.Context, userID string) ([]InboxItemResponse, error)
}

type MessageUsecase interface {
	SendMessage(ctx context.Context, senderID string, req *SendMessageRequest) (*MessageResponse, error)
	GetChatHistory(ctx context.Context, currentUserID, targetUserID string) ([]MessageResponse, error)
	GetInbox(ctx context.Context, userID string) ([]InboxItemResponse, error)
}
