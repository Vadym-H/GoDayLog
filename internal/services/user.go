package services

import (
	"context"
	"log/slog"
)

type UserCreator interface {
	CreateUser(ctx context.Context, provider, externalID string) (string, bool, error)
	UpdateUserContext(ctx context.Context, provider, externalID, llmContext string) error
}

type UserService struct {
	log  *slog.Logger
	repo UserCreator
}

func NewUserService(log *slog.Logger, repo UserCreator) *UserService {
	return &UserService{log: log, repo: repo}
}

// RegisterUser registers an external identity and backing user.
func (s *UserService) RegisterUser(ctx context.Context, provider, externalID string) (string, bool, error) {
	const op = "services.UserService.RegisterUser"

	userID, created, err := s.repo.CreateUser(ctx, provider, externalID)
	if err != nil {
		s.log.Warn("failed to create user",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
		return "", false, err
	}

	if created {
		s.log.Info("user registered",
			slog.String("op", op),
			slog.String("provider", provider),
			slog.String("external_id", externalID),
			slog.String("user_id", userID),
		)
	}

	return userID, created, nil
}

func (s *UserService) UpdateUserContext(ctx context.Context, provider, externalID, llmContext string) error {
	const op = "services.UserService.UpdateUserContext"

	if err := s.repo.UpdateUserContext(ctx, provider, externalID, llmContext); err != nil {
		s.log.Warn("failed to update user context",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
		return err
	}

	s.log.Info("user context updated",
		slog.String("op", op),
		slog.String("provider", provider),
		slog.String("external_id", externalID),
	)

	return nil
}
