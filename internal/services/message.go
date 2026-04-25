package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

const (
	messageStatusDone   = "done"
	messageStatusFailed = "failed"
)

type AIClient interface {
	ExtractActivity(ctx context.Context, today, userContext, userMessage string) (ai.ActivityResponse, error)
}

type UserContextReader interface {
	GetUserContext(ctx context.Context, id domain.Identity) (string, error)
	GetUserTimezone(ctx context.Context, id domain.Identity) (string, error)
}

type ActivityLogRepository interface {
	CreateActivityLogs(ctx context.Context, messageID string, items []domain.Activity) error
}

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
	log := logger.From(ctx, s.log)

	messageID, err := s.repo.SaveMessage(ctx, id, externalMessageID, text)
	if err != nil {
		log.Error("failed to save message", slog.Any("error", err))
		return "", err
	}

	log.Info("message saved", slog.String("message_id", messageID))

	return messageID, nil
}

type MessageAIProcessor struct {
	log           *slog.Logger
	ai            AIClient
	messageRepo   MessageRepository
	userCtxReader UserContextReader
	activityRepo  ActivityLogRepository
}

func NewMessageAIProcessor(log *slog.Logger, ai AIClient, messageRepo MessageRepository, userCtxReader UserContextReader, activityRepo ActivityLogRepository) *MessageAIProcessor {
	return &MessageAIProcessor{
		log:           log,
		ai:            ai,
		messageRepo:   messageRepo,
		userCtxReader: userCtxReader,
		activityRepo:  activityRepo,
	}
}

// ExtractActivities calls AI and returns items for user review.
// On AI failure or invalid input it marks the message as failed and returns an error.
func (p *MessageAIProcessor) ExtractActivities(ctx context.Context, id domain.Identity, messageID, text string) ([]domain.Activity, error) {
	log := logger.From(ctx, p.log)

	userContext, err := p.userCtxReader.GetUserContext(ctx, id)
	if err != nil {
		log.Warn("could not fetch user context, proceeding without it",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		userContext = ""
	}

	tz, err := p.userCtxReader.GetUserTimezone(ctx, id)
	if err != nil {
		log.Warn("could not fetch user timezone, falling back to UTC",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Warn("invalid timezone in DB, falling back to UTC",
			slog.String("timezone", tz),
			slog.Any("error", err),
		)
		loc = time.UTC
	}
	today := time.Now().In(loc).Format("2006-01-02")

	response, err := p.ai.ExtractActivity(ctx, today, userContext, text)
	if err != nil {
		log.Error("failed to process message with ai",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		_ = p.markFailed(ctx, messageID, err.Error())
		return nil, err
	}

	if !response.Valid {
		log.Info("ai marked message as invalid", slog.String("message_id", messageID))
		_ = p.markFailed(ctx, messageID, "ai: input not a valid day activity")
		return nil, nil
	}

	activities := make([]domain.Activity, len(response.Activities))
	for i, a := range response.Activities {
		activities[i] = domain.Activity{
			Description:     a.Description,
			Tag:             a.Tag,
			ActivityType:    a.ActivityType,
			DurationMinutes: a.DurationMinutes,
			StartedAt:       a.StartedAt,
			CompletedAt:     a.CompletedAt,
		}
	}

	return activities, nil
}

// SaveActivities inserts accepted activities in a single transaction and marks the message done.
func (p *MessageAIProcessor) SaveActivities(ctx context.Context, messageID string, activities []domain.Activity) error {
	log := logger.From(ctx, p.log)

	if err := p.activityRepo.CreateActivityLogs(ctx, messageID, activities); err != nil {
		log.Error("failed to create activity logs",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		_ = p.markFailed(ctx, messageID, err.Error())
		return err
	}

	if err := p.messageRepo.UpdateMessageStatus(ctx, messageID, messageStatusDone, ""); err != nil {
		log.Error("failed to mark message done",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// CancelReview marks the message as failed because the user cancelled the review.
func (p *MessageAIProcessor) CancelReview(ctx context.Context, messageID string) error {
	return p.markFailed(ctx, messageID, "user cancelled review")
}

func (p *MessageAIProcessor) markFailed(ctx context.Context, messageID, reason string) error {
	log := logger.From(ctx, p.log)

	if err := p.messageRepo.UpdateMessageStatus(ctx, messageID, messageStatusFailed, reason); err != nil {
		log.Error("failed to mark message failed",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
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
