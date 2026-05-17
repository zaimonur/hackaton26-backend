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

func (r *messageRepository) GetInbox(ctx context.Context, userID string) ([]domain.InboxItemResponse, error) {
	// CTE ve Window Function (ROW_NUMBER) kullanımı:
	// DISTINCT ON kullanıldığında sıralama (ORDER BY) ilk olarak distinct kolona göre yapılmak zorundadır,
	// bu da gelen kutusundaki "en son mesajlaşan en üstte" kuralını bozar.
	// ROW_NUMBER() ile Partition By atarak bu sorunu eziyoruz ve global tarihe göre sıralayabiliyoruz.
	query := `
		WITH RankedMessages AS (
			SELECT 
				CASE WHEN sender_id = $1 THEN receiver_id ELSE sender_id END AS target_id,
				content AS last_message,
				created_at,
				ROW_NUMBER() OVER(
					PARTITION BY CASE WHEN sender_id = $1 THEN receiver_id ELSE sender_id END 
					ORDER BY created_at DESC
				) as rn
			FROM messages
			WHERE sender_id = $1 OR receiver_id = $1
		)
		SELECT 
			rm.target_id, 
			u.email AS target_name, 
			u.role AS target_role, 
			rm.last_message, 
			rm.created_at
		FROM RankedMessages rm
		JOIN users u ON rm.target_id = u.id
		WHERE rm.rn = 1
		ORDER BY rm.created_at DESC
	`

	var inbox []domain.InboxItemResponse
	if err := r.db.SelectContext(ctx, &inbox, query, userID); err != nil {
		return nil, err
	}

	if inbox == nil {
		inbox = []domain.InboxItemResponse{}
	}

	return inbox, nil
}
