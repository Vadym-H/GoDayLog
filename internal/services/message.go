package services

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/domain"
)

const (
	messageStatusDone   = "done"
	messageStatusFailed = "failed"
)

type MessageRepository interface {
	SaveMessage(ctx context.Context, id domain.Identity, externalMessageID, text string) (string, error)
	UpdateMessageStatus(ctx context.Context, messageID, status, statusError string) error
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
	const op = "services.MessageService.SaveMessage"

	messageID, err := s.repo.SaveMessage(ctx, id, externalMessageID, text)
	if err != nil {
		s.log.Warn("failed to save message",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
		return "", err
	}

	return messageID, nil
}

func (s *MessageService) MarkDone(ctx context.Context, messageID string) error {
	const op = "services.MessageService.MarkDone"

	if err := s.repo.UpdateMessageStatus(ctx, messageID, messageStatusDone, ""); err != nil {
		s.log.Warn("failed to mark message done",
			slog.String("op", op),
			slog.String("message_id", messageID),
			slog.String("error", err.Error()),
		)
		return err
	}

	return nil
}

func (s *MessageService) MarkFailed(ctx context.Context, messageID, reason string) error {
	const op = "services.MessageService.MarkFailed"

	if err := s.repo.UpdateMessageStatus(ctx, messageID, messageStatusFailed, reason); err != nil {
		s.log.Warn("failed to mark message failed",
			slog.String("op", op),
			slog.String("message_id", messageID),
			slog.String("error", err.Error()),
		)
		return err
	}

	return nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, messageID string) error {
	const op = "services.MessageService.DeleteMessage"

	if err := s.repo.DeleteMessage(ctx, messageID); err != nil {
		s.log.Warn("failed to delete message",
			slog.String("op", op),
			slog.String("message_id", messageID),
			slog.String("error", err.Error()),
		)
		return err
	}

	return nil
}
