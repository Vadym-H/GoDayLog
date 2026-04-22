package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func NewUserRepo(s *Storage) *UserRepo {
	return &UserRepo{db: s.db, log: s.log}
}

// CreateUser creates a user and links the external provider identity.
// It returns created=false when identity already exists.
func (r *UserRepo) CreateUser(ctx context.Context, id domain.Identity) (string, bool, error) {
	const op = "storage.postgres.CreateUser"
	log := r.log.With(
		slog.String("op", op),
		slog.String("provider", id.Provider),
	)

	tx, err := r.db.Begin(ctx)
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
	`, userID, id.Provider, id.ExternalID)

	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			_ = tx.Rollback(ctx)
			err = r.db.QueryRow(ctx, `
        SELECT user_id
        FROM user_identities
        WHERE provider = $1 AND external_id = $2
    `, id.Provider, id.ExternalID).Scan(&userID)

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

func (r *UserRepo) UpdateUserContext(ctx context.Context, id domain.Identity, llmContext string) error {
	const op = "storage.postgres.UpdateUserContext"

	// Update context by external identity so Telegram-specific IDs stay outside domain tables.
	cmdTag, err := r.db.Exec(ctx, `
		UPDATE users u
		SET llm_context = $1
		FROM user_identities ui
		WHERE u.id = ui.user_id
		  AND ui.provider = $2
		  AND ui.external_id = $3
		  AND u.deleted_at IS NULL
	`, llmContext, id.Provider, id.ExternalID)
	if err != nil {
		return fmt.Errorf("%s: update context: %w", op, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%s: user not found for provider/external_id", op)
	}

	r.log.Info("user context updated",
		slog.String("op", op),
		slog.String("provider", id.Provider),
		slog.String("external_id", id.ExternalID),
	)

	return nil
}

func (r *UserRepo) GetUserContext(ctx context.Context, id domain.Identity) (string, error) {
	const op = "storage.postgres.GetUserContext"

	var llmContext string
	err := r.db.QueryRow(ctx, `
        SELECT u.llm_context
        FROM users u
        JOIN user_identities ui ON ui.user_id = u.id
        WHERE ui.provider = $1 AND ui.external_id = $2 AND u.deleted_at IS NULL
    `, id.Provider, id.ExternalID).Scan(&llmContext)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}
		return "", fmt.Errorf("%s: query user context: %w", op, err)
	}

	return llmContext, nil
}
