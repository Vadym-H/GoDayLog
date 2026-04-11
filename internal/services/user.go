package services

import (
	"context"
	"log/slog"
)

type UserCreator interface {
	CreateUser(ctx context.Context, provider, externalID string) error
}

type UserService struct {
	log  *slog.Logger
	repo UserCreator
}

func NewUserService(log *slog.Logger, repo UserCreator) *UserService {
	return &UserService{log: log, repo: repo}
}

// RegisterUser registers an external identity and backing user.
func (s *UserService) RegisterUser(ctx context.Context, provider, externalID string) error {
	if err := s.repo.CreateUser(ctx, provider, externalID); err != nil {
		s.log.Warn("failed to create user", slog.String("error", err.Error()))
		return err
	}

	s.log.Info("user registered",
		slog.String("provider", provider),
		slog.String("external_id", externalID),
	)

	return nil
}
