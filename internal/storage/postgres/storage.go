package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

// New initializes a new Storage instance by connecting to the PostgreSQL database using the provided connection string.
func New(ctx context.Context, log *slog.Logger, connStr string) (*Storage, error) {
	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &Storage{db: db, log: log}, nil
}

func (s *Storage) Close() {
	s.db.Close()
}

// CreateUser creates a user in the database. It is called by the service layer when registering a user.
func (s *Storage) CreateUser(ctx context.Context, userID int64, username, firstName string) error {
	const op = "storage.CreateUser"

	query := `INSERT INTO users (id, username, first_name)
	VALUES ($1, $2, $3)
	ON CONFLICT (id) DO NOTHING`

	_, err := s.db.Exec(ctx, query, userID, username, firstName)
	if err != nil {
		return fmt.Errorf("%s: %w", op, storage.UserExists)
	}

	s.log.Info("user created",
		slog.Int64("user_id", userID),
		slog.String("username", username),
		slog.String("first_name", firstName),
	)

	return nil
}
