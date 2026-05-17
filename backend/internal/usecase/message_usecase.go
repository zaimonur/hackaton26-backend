package usecase

import (
	"context"
	"drewisy/internal/domain"
	"drewisy/internal/infrastructure/websocket"
)

type messageUsecase struct {
	messageRepo domain.MessageRepository
	hub         *websocket.Hub
}

func NewMessageUsecase(mr domain.MessageRepository, hub *websocket.Hub) domain.MessageUsecase {
	return &messageUsecase{
		messageRepo: mr,
		hub:         hub,
	}
}

func (u *messageUsecase) SendMessage(ctx context.Context, senderID string, req *domain.SendMessageRequest) (*domain.MessageResponse, error) {
	msg := &domain.Message{
		SenderID:   senderID,
		ReceiverID: req.ReceiverID,
		Content:    req.Content,
	}

	if err := u.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	resp := &domain.MessageResponse{
		ID:         msg.ID,
		SenderID:   msg.SenderID,
		ReceiverID: msg.ReceiverID,
		Content:    msg.Content,
		CreatedAt:  msg.CreatedAt,
	}

	// Anlık bildirim için WS Tetikleyici fırlatılıyor
	event := domain.WSEvent{
		Type:    "NEW_MESSAGE",
		Payload: resp,
	}
	u.hub.SendToUser(req.ReceiverID, event)

	return resp, nil
}

func (u *messageUsecase) GetChatHistory(ctx context.Context, currentUserID, targetUserID string) ([]domain.MessageResponse, error) {
	messages, err := u.messageRepo.GetChatHistory(ctx, currentUserID, targetUserID)
	if err != nil {
		return nil, err
	}

	resp := make([]domain.MessageResponse, 0, len(messages))
	for _, m := range messages {
		resp = append(resp, domain.MessageResponse{
			ID:         m.ID,
			SenderID:   m.SenderID,
			ReceiverID: m.ReceiverID,
			Content:    m.Content,
			CreatedAt:  m.CreatedAt,
		})
	}
	return resp, nil
}
