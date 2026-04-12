package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

// New initializes a new Storage instance by connecting to the PostgreSQL database using the provided connection string.
func New(ctx context.Context, log *slog.Logger, connStr string) (*Storage, error) {
	const op = "storage.postgres.New"

	db, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Info("storage initialized", slog.String("op", op))

	return &Storage{db: db, log: log}, nil
}

func (s *Storage) Close() {
	const op = "storage.postgres.Close"
	s.log.Info("closing storage", slog.String("op", op))
	s.db.Close()
}
