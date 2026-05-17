package usecase

import (
	"context"
	"drewisy/internal/domain"
)

type notificationUsecase struct {
	notificationRepo domain.NotificationRepository
}

func NewNotificationUsecase(nr domain.NotificationRepository) domain.NotificationUsecase {
	return &notificationUsecase{notificationRepo: nr}
}

func (u *notificationUsecase) GetMyNotifications(ctx context.Context, userID string) ([]domain.NotificationResponse, error) {
	notifications, err := u.notificationRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := make([]domain.NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		resp = append(resp, domain.NotificationResponse{
			ID:          n.ID,
			Type:        n.Type,
			ReferenceID: n.ReferenceID,
			Title:       n.Title,
			Body:        n.Body,
			IsRead:      n.IsRead,
			CreatedAt:   n.CreatedAt,
		})
	}
	return resp, nil
}

func (u *notificationUsecase) MarkAsRead(ctx context.Context, id, userID string) error {
	return u.notificationRepo.MarkAsRead(ctx, id, userID)
}
