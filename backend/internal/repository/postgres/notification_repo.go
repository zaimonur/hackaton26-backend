package postgres

import (
	"context"
	"drewisy/internal/domain"
	"errors"

	"github.com/jmoiron/sqlx"
)

type notificationRepository struct {
	db *sqlx.DB
}

func NewNotificationRepository(db *sqlx.DB) domain.NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	query := `
		INSERT INTO notifications (user_id, type, reference_id, title, body, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, NOW())
		RETURNING id, created_at, is_read
	`
	return r.db.QueryRowxContext(ctx, query, n.UserID, n.Type, n.ReferenceID, n.Title, n.Body).
		Scan(&n.ID, &n.CreatedAt, &n.IsRead)
}

func (r *notificationRepository) GetByUserID(ctx context.Context, userID string) ([]domain.Notification, error) {
	query := `
		SELECT id, user_id, type, reference_id, title, body, is_read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	var notifications []domain.Notification
	if err := r.db.SelectContext(ctx, &notifications, query, userID); err != nil {
		return nil, err
	}

	if notifications == nil {
		notifications = []domain.Notification{}
	}

	return notifications, nil
}

func (r *notificationRepository) MarkAsRead(ctx context.Context, id, userID string) error {
	// IDOR Koruması: Sadece ilgili kullanıcının bildirimi güncellenebilir
	query := `
		UPDATE notifications
		SET is_read = TRUE
		WHERE id = $1 AND user_id = $2
	`
	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("bildirim bulunamadı veya bu işlem için yetkiniz yok")
	}

	return nil
}
