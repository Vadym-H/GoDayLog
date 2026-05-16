package services

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

type MessageRepository interface {
	SaveMessage(ctx context.Context, id domain.Identity, externalMessageID, text string) (string, error)
	UpdateMessageStatus(ctx context.Context, messageID, userID, status, statusError string) error
	DeleteMessage(ctx context.Context, messageID string) error
}

type MessageService struct {
	log  *slog.Logger
	repo MessageRepository
}

func NewMessageService(log *slog.Logger, repo MessageRepository) *MessageService {
	return &MessageService{log: log, repo: repo}
}

func (s *MessageService) SaveMessage(ctx context.Context, id domain.Identity, externalMessageID, text string) (string, error) {
	log := logger.From(ctx, s.log)

	messageID, err := s.repo.SaveMessage(ctx, id, externalMessageID, text)
	if err != nil {
		log.Error("failed to save message", slog.Any("error", err))
		return "", err
	}

	log.Info("message saved", slog.String("message_id", messageID))

	return messageID, nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, messageID string) error {
	log := logger.From(ctx, s.log)

	if err := s.repo.DeleteMessage(ctx, messageID); err != nil {
		log.Error("failed to delete message",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}
