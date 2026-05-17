package postgres

import (
	"context"
	"drewisy/internal/domain"

	"github.com/jmoiron/sqlx"
)

type messageRepository struct {
	db *sqlx.DB
}

func NewMessageRepository(db *sqlx.DB) domain.MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) Create(ctx context.Context, msg *domain.Message) error {
	query := `
		INSERT INTO messages (sender_id, receiver_id, content, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, created_at
	`
	return r.db.QueryRowxContext(ctx, query, msg.SenderID, msg.ReceiverID, msg.Content).
		Scan(&msg.ID, &msg.CreatedAt)
}

func (r *messageRepository) GetChatHistory(ctx context.Context, user1ID, user2ID string) ([]domain.Message, error) {
	query := `
		SELECT id, sender_id, receiver_id, content, created_at
		FROM messages
		WHERE (sender_id = $1 AND receiver_id = $2)
		   OR (sender_id = $2 AND receiver_id = $1)
		ORDER BY created_at ASC
	`
	var messages []domain.Message
	if err := r.db.SelectContext(ctx, &messages, query, user1ID, user2ID); err != nil {
		return nil, err
	}

	if messages == nil {
		messages = []domain.Message{}
	}

	return messages, nil
}
