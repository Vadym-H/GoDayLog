package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MessageStatusPending    = "pending"
	MessageStatusProcessing = "processing"
	MessageStatusDone       = "done"
	MessageStatusFailed     = "failed"
)

var allowedMessageStatuses = map[string]struct{}{
	MessageStatusPending:    {},
	MessageStatusProcessing: {},
	MessageStatusDone:       {},
	MessageStatusFailed:     {},
}

type MessageRepo struct {
	db  *pgxpool.Pool
	log *slog.Logger
}

func NewMessageRepo(s *Storage) *MessageRepo {
	return &MessageRepo{db: s.db, log: s.log}
}

func (r *MessageRepo) SaveMessage(ctx context.Context, id domain.Identity, externalMessageID, text string) (string, error) {
	const op = "storage.postgres.SaveMessage"

	status := MessageStatusPending
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return "", fmt.Errorf("%s: message text is empty", op)
	}

	var messageID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO messages (user_id, external_message_id, text, status)
		SELECT u.id, NULLIF($3, ''), $4, $5
		FROM users u
		JOIN user_identities ui ON ui.user_id = u.id
		WHERE ui.provider = $1
		  AND ui.external_id = $2
		  AND u.deleted_at IS NULL
		RETURNING id
	`, id.Provider, id.ExternalID, strings.TrimSpace(externalMessageID), trimmedText, status).Scan(&messageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}
		return "", fmt.Errorf("%s: insert message: %w", op, err)
	}

	return messageID, nil
}

func (r *MessageRepo) UpdateMessageStatus(ctx context.Context, messageID, status, statusError string) error {
	const op = "storage.postgres.UpdateMessageStatus"

	if _, ok := allowedMessageStatuses[status]; !ok {
		return fmt.Errorf("%s: %w", op, storage.ErrInvalidMessageStatus)
	}

	var dbErr any
	if status == MessageStatusFailed {
		trimmedErr := strings.TrimSpace(statusError)
		if trimmedErr == "" {
			trimmedErr = "unknown error"
		}
		dbErr = trimmedErr
	}

	cmdTag, err := r.db.Exec(ctx, `
		UPDATE messages
		SET status = $2,
		    error = $3
		WHERE id = $1
	`, messageID, status, dbErr)
	if err != nil {
		return fmt.Errorf("%s: update status: %w", op, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrMessageNotFound)
	}

	return nil
}

func (r *MessageRepo) DeleteMessage(ctx context.Context, messageID string) error {
	const op = "storage.postgres.DeleteMessage"

	cmdTag, err := r.db.Exec(ctx, `DELETE FROM messages WHERE id = $1`, messageID)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23503" {
			return fmt.Errorf("%s: %w", op, storage.ErrMessageInUse)
		}
		return fmt.Errorf("%s: delete message: %w", op, err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrMessageNotFound)
	}

	return nil
}
