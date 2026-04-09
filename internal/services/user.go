package services

import (
	"context"
	"log/slog"
)

type UserCreator interface {
	CreateUser(ctx context.Context, userID int64, username, firstName string) error
}

type UserService struct {
	log  *slog.Logger
	repo UserCreator
}

func NewUserService(log *slog.Logger, repo UserCreator) *UserService {
	return &UserService{log: log, repo: repo}
}

// RegisterUser is a service layer method that registers a user in the system.
// It delegates user creation to the repository and logs the result.
func (s *UserService) RegisterUser(ctx context.Context, userID int64, username, firstName string) error {
	if err := s.repo.CreateUser(ctx, userID, username, firstName); err != nil {
		s.log.Error("failed to create user", slog.String("error", err.Error()))
		return err
	}

	s.log.Info("user registered",
		slog.Int64("user_id", userID),
		slog.String("username", username),
		slog.String("first_name", firstName),
	)

	return nil
}
