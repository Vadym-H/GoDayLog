package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(s *Storage) *UserRepo {
	return &UserRepo{db: s.db}
}

// CreateUser creates a user and links the external provider identity.
// Returns created=false when the identity already exists.
func (r *UserRepo) CreateUser(ctx context.Context, id domain.Identity) (string, bool, error) {
	const op = "storage.postgres.CreateUser"

	tx, err := r.db.Begin(ctx)
	if err != nil {
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
				SELECT user_id FROM user_identities
				WHERE provider = $1 AND external_id = $2
			`, id.Provider, id.ExternalID).Scan(&userID)
			if err != nil {
				return "", false, fmt.Errorf("%s: get existing user: %w", op, err)
			}
			return userID, false, nil
		}
		return "", false, fmt.Errorf("%s: create identity: %w", op, err)
	}

	if err = tx.Commit(ctx); err != nil {
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
		return fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
	}

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

func (r *UserRepo) GetUserID(ctx context.Context, id domain.Identity) (string, error) {
	const op = "storage.postgres.GetUserID"

	var userID string
	err := r.db.QueryRow(ctx, `
		SELECT user_id FROM user_identities
		WHERE provider = $1 AND external_id = $2
	`, id.Provider, id.ExternalID).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return userID, nil
}

func (r *UserRepo) GetUserTimezone(ctx context.Context, id domain.Identity) (string, error) {
	const op = "storage.postgres.GetUserTimezone"

	var tz string
	err := r.db.QueryRow(ctx, `
		SELECT u.timezone
		FROM users u
		JOIN user_identities ui ON ui.user_id = u.id
		WHERE ui.provider = $1 AND ui.external_id = $2 AND u.deleted_at IS NULL
	`, id.Provider, id.ExternalID).Scan(&tz)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}
		return "", fmt.Errorf("%s: query timezone: %w", op, err)
	}

	return tz, nil
}
