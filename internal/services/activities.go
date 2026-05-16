package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/storage"
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
	GetUserID(ctx context.Context, id domain.Identity) (string, error)
	GetUserPlan(ctx context.Context, userID string) (string, error)
}

type ActivityLogRepository interface {
	CreateActivityLogs(ctx context.Context, messageID, userID string, items []domain.Activity) error
}

type AIRequestLogRepository interface {
	CreateAIRequestLog(ctx context.Context, messageID, model string, usage ai.TokenUsage) error
	SumTokensSince(ctx context.Context, userID string, since time.Time) (int, error)
}

type messageAIProcessor struct {
	log           *slog.Logger
	ai            AIClient
	messageRepo   MessageRepository
	userCtxReader UserContextReader
	activityRepo  ActivityLogRepository
	aiLogRepo     AIRequestLogRepository
	limits        config.LLMUsageLimits
	proLimits     config.LLMUsageLimits
}

func NewMessageAIProcessor(log *slog.Logger, ai AIClient, messageRepo MessageRepository, userCtxReader UserContextReader, activityRepo ActivityLogRepository, aiLogRepo AIRequestLogRepository, limits config.LLMUsageLimits, proLimits config.LLMUsageLimits) *messageAIProcessor {
	return &messageAIProcessor{
		log:           log,
		ai:            ai,
		messageRepo:   messageRepo,
		userCtxReader: userCtxReader,
		activityRepo:  activityRepo,
		aiLogRepo:     aiLogRepo,
		limits:        limits,
		proLimits:     proLimits,
	}
}

// ExtractActivities calls AI and returns items for user review.
// On AI failure or invalid input it marks the message as failed and returns an error.
func (p *messageAIProcessor) ExtractActivities(ctx context.Context, id domain.Identity, messageID, text string) ([]domain.Activity, error) {
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
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")

	// userID is needed both for plan/budget checks and for marking the message
	// failed under the owning user. We always look it up; if it fails, markFailed
	// is skipped (the user can resubmit and the stale-pending sweeper will catch it).
	userID, idErr := p.userCtxReader.GetUserID(ctx, id)
	if idErr != nil {
		log.Warn("could not fetch user id",
			slog.String("message_id", messageID),
			slog.Any("error", idErr),
		)
	}

	limits := p.limits
	if p.limits.Enabled && userID != "" {
		plan, planErr := p.userCtxReader.GetUserPlan(ctx, userID)
		if planErr != nil {
			log.Warn("could not fetch user plan, defaulting to free",
				slog.String("message_id", messageID),
				slog.Any("error", planErr),
			)
		} else if plan == "pro" {
			limits = p.proLimits
			limits.Enabled = p.limits.Enabled
		}

		if limits.DailyBudgetTokens > 0 {
			dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
			used, sumErr := p.aiLogRepo.SumTokensSince(ctx, userID, dayStart)
			if sumErr != nil {
				log.Warn("could not sum daily tokens, skipping budget check",
					slog.String("message_id", messageID),
					slog.Any("error", sumErr),
				)
			} else if used >= limits.DailyBudgetTokens {
				_ = p.markFailed(ctx, messageID, userID, "daily budget exceeded")
				return nil, ai.ErrDailyBudgetExceeded
			}
		}
	}

	response, err := p.ai.ExtractActivity(ctx, today, userContext, text)
	if response.Usage.TotalTokens > 0 {
		if logErr := p.aiLogRepo.CreateAIRequestLog(ctx, messageID, response.Model, response.Usage); logErr != nil {
			log.Warn("failed to log ai request usage",
				slog.String("message_id", messageID),
				slog.Any("error", logErr),
			)
		}
	}
	if err != nil {
		log.Info("failed to process message with ai",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		_ = p.markFailed(ctx, messageID, userID, err.Error())
		return nil, err
	}

	if !response.Valid {
		log.Info("ai marked message as invalid", slog.String("message_id", messageID))
		_ = p.markFailed(ctx, messageID, userID, "ai: input not a valid day activity")
		return nil, nil
	}

	if len(response.Activities) == 0 {
		log.Info("ai returned no activities", slog.String("message_id", messageID))
		_ = p.markFailed(ctx, messageID, userID, "ai: no activities extracted")
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
// All storage writes are scoped to the caller's user_id, so a foreign message ID
// returns ErrMessageNotFound instead of mutating another user's data.
func (p *messageAIProcessor) SaveActivities(ctx context.Context, id domain.Identity, messageID string, activities []domain.Activity) error {
	log := logger.From(ctx, p.log)

	// Guard: a zero identity must never produce an empty user_id that would
	// silently match nothing in the scoped UPDATE/INSERT WHERE clauses.
	if id.Provider == "" || id.ExternalID == "" {
		return fmt.Errorf("save activities: %w", storage.ErrUserNotFound)
	}

	userID, err := p.userCtxReader.GetUserID(ctx, id)
	if err != nil {
		log.Error("failed to resolve user id",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		return err
	}

	if err := p.activityRepo.CreateActivityLogs(ctx, messageID, userID, activities); err != nil {
		log.Error("failed to create activity logs",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		_ = p.markFailed(ctx, messageID, userID, err.Error())
		return err
	}

	if err := p.messageRepo.UpdateMessageStatus(ctx, messageID, userID, messageStatusDone, ""); err != nil {
		log.Error("failed to mark message done",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}

// CancelReview marks the message as failed because the user cancelled the review.
func (p *messageAIProcessor) CancelReview(ctx context.Context, id domain.Identity, messageID string) error {
	log := logger.From(ctx, p.log)

	if id.Provider == "" || id.ExternalID == "" {
		return fmt.Errorf("cancel review: %w", storage.ErrUserNotFound)
	}

	userID, err := p.userCtxReader.GetUserID(ctx, id)
	if err != nil {
		log.Error("failed to resolve user id",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		return err
	}
	return p.markFailed(ctx, messageID, userID, "user cancelled review")
}

// markFailed is a no-op when userID is empty so that internal flows
// which couldn't resolve the user can call it without error.
func (p *messageAIProcessor) markFailed(ctx context.Context, messageID, userID, reason string) error {
	log := logger.From(ctx, p.log)
	if userID == "" {
		return nil
	}

	if err := p.messageRepo.UpdateMessageStatus(ctx, messageID, userID, messageStatusFailed, reason); err != nil {
		log.Error("failed to mark message failed",
			slog.String("message_id", messageID),
			slog.Any("error", err),
		)
		return err
	}

	return nil
}
