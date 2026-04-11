package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
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

// CreateUser creates a user and links the external provider identity.
// It returns created=false when identity already exists.
func (s *Storage) CreateUser(ctx context.Context, provider, externalID string) (string, bool, error) {
	const op = "storage.postgres.CreateUser"
	log := s.log.With(
		slog.String("op", op),
		slog.String("provider", provider),
	)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Warn("failed to begin transaction", slog.String("error", err.Error()))
		return "", false, fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var userID string

	err = tx.QueryRow(ctx,
		`INSERT INTO users DEFAULT VALUES RETURNING id`,
	).Scan(&userID)
	if err != nil {
		log.Warn("failed to create user", slog.String("error", err.Error()))
		return "", false, fmt.Errorf("%s: create user: %w", op, err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_identities (user_id, provider, external_id)
		VALUES ($1, $2, $3)
	`, userID, provider, externalID)

	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			_ = tx.Rollback(ctx)
			err = s.db.QueryRow(ctx, `
				SELECT user_id
				FROM user_identities
				WHERE provider = $1 AND external_id = $2
			`, provider, externalID).Scan(&userID)

			if err != nil {
				log.Warn("failed to load existing user after duplicate identity", slog.String("error", err.Error()))
				return "", false, fmt.Errorf("%s: get existing user: %w", op, err)
			}

			log.Debug("identity already exists, returned existing user", slog.String("user_id", userID))

			return userID, false, nil
		}
		log.Warn("failed to create identity", slog.String("error", err.Error()))
		return "", false, fmt.Errorf("%s: create identity: %w", op, err)
	}
	if err = tx.Commit(ctx); err != nil {
		log.Warn("failed to commit transaction", slog.String("error", err.Error()))
		return "", false, fmt.Errorf("%s: commit tx: %w", op, err)
	}
	return userID, true, nil
}

func (s *Storage) UpdateUserContext(ctx context.Context, provider, externalID, llmContext string) error {
	const op = "storage.postgres.UpdateUserContext"

	// Update context by external identity so Telegram-specific IDs stay outside domain tables.
	cmdTag, err := s.db.Exec(ctx, `
		UPDATE users u
		SET llm_context = $1
		FROM user_identities ui
		WHERE u.id = ui.user_id
		  AND ui.provider = $2
		  AND ui.external_id = $3
		  AND u.deleted_at IS NULL
	`, llmContext, provider, externalID)
	if err != nil {
		return fmt.Errorf("%s: update context: %w", op, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%s: user not found for provider/external_id", op)
	}

	s.log.Info("user context updated",
		slog.String("op", op),
		slog.String("provider", provider),
		slog.String("external_id", externalID),
	)

	return nil
}
