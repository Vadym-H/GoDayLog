package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/storage"
)

type ErrContextTooLong struct {
	Len   int
	Limit int
}

func (e *ErrContextTooLong) Error() string {
	return fmt.Sprintf("user context too long: %d chars, limit %d", e.Len, e.Limit)
}

type UserCreator interface {
	CreateUser(ctx context.Context, id domain.Identity) (string, bool, error)
}
type UserContext interface {
	GetUserContext(ctx context.Context, id domain.Identity) (string, error)
	UpdateUserContext(ctx context.Context, id domain.Identity, llmContext string) error
	UpdateUserTimezone(ctx context.Context, id domain.Identity, timezone string) error
	GetUserID(ctx context.Context, id domain.Identity) (string, error)
	GetUserTimezone(ctx context.Context, id domain.Identity) (string, error)
	GetUserPlanByIdentity(ctx context.Context, id domain.Identity) (string, error)
}

type UserService struct {
	log       *slog.Logger
	repo      UserCreator
	userctx   UserContext
	limits    config.LLMUsageLimits
	proLimits config.LLMUsageLimits
}

func NewUserService(log *slog.Logger, repo UserCreator, userctx UserContext, limits config.LLMUsageLimits, proLimits config.LLMUsageLimits) *UserService {
	return &UserService{log: log, repo: repo, userctx: userctx, limits: limits, proLimits: proLimits}
}

// RegisterUser registers an external identity and backing user.
func (s *UserService) RegisterUser(ctx context.Context, id domain.Identity) (string, bool, error) {
	log := logger.From(ctx, s.log)

	userID, created, err := s.repo.CreateUser(ctx, id)
	if err != nil {
		log.Error("failed to create user", slog.Any("error", err))
		return "", false, err
	}

	if created {
		log.Info("user registered",
			slog.String("provider", id.Provider),
			slog.String("external_id", id.ExternalID),
			slog.String("user_id", userID),
		)
	}

	return userID, created, nil
}

func (s *UserService) UpdateUserContext(ctx context.Context, id domain.Identity, llmContext string) error {
	log := logger.From(ctx, s.log)

	limits := s.limits
	if s.limits.Enabled {
		plan, planErr := s.userctx.GetUserPlanByIdentity(ctx, id)
		if planErr != nil {
			log.Warn("could not fetch user plan, defaulting to free", slog.Any("error", planErr))
		} else if plan == "pro" {
			limits = s.proLimits
			limits.Enabled = s.limits.Enabled
		}
	}

	if limits.Enabled && limits.MaxContextChars > 0 && len(llmContext) > limits.MaxContextChars {
		return &ErrContextTooLong{Len: len(llmContext), Limit: limits.MaxContextChars}
	}

	if err := s.userctx.UpdateUserContext(ctx, id, llmContext); err != nil {
		log.Error("failed to update user context", slog.Any("error", err))
		return err
	}

	log.Info("user context updated",
		slog.String("provider", id.Provider),
		slog.String("external_id", id.ExternalID),
	)

	return nil
}

func (s *UserService) GetUserID(ctx context.Context, id domain.Identity) (string, error) {
	return s.userctx.GetUserID(ctx, id)
}

func (s *UserService) GetUserTimezone(ctx context.Context, id domain.Identity) (string, error) {
	return s.userctx.GetUserTimezone(ctx, id)
}

func (s *UserService) UpdateUserTimezone(ctx context.Context, id domain.Identity, timezone string) error {
	if err := s.userctx.UpdateUserTimezone(ctx, id, timezone); err != nil {
		logger.From(ctx, s.log).Error("failed to update user timezone", slog.Any("error", err))
		return err
	}
	return nil
}

func (s *UserService) GetUserContext(ctx context.Context, id domain.Identity) (string, error) {
	log := logger.From(ctx, s.log)

	llmContext, err := s.userctx.GetUserContext(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Debug("user not found when getting context",
				slog.String("provider", id.Provider),
				slog.String("external_id", id.ExternalID),
			)
			return "", err
		}
		log.Error("failed to get user context", slog.Any("error", err))
		return "", err
	}
	return llmContext, nil
}
