package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
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

// CreateUser creates a user and links the external provider identity.
func (s *Storage) CreateUser(ctx context.Context, provider, externalID string) error {
	const op = "storage.CreateUser"

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var userID string
	err = tx.QueryRow(ctx, `INSERT INTO users DEFAULT VALUES RETURNING id`).Scan(&userID)
	if err != nil {
		return fmt.Errorf("%s: create user: %w", op, err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_identities (user_id, provider, external_id)
		VALUES ($1, $2, $3)
	`, userID, provider, externalID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%s: %w", op, storage.UserExists)
		}

		return fmt.Errorf("%s: create user identity: %w", op, err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit tx: %w", op, err)
	}

	s.log.Info("user created",
		slog.String("provider", provider),
		slog.String("external_id", externalID),
	)

	return nil
}
