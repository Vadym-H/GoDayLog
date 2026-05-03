package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ActivityLogRepo struct {
	db *pgxpool.Pool
}

func NewActivityLogRepo(s *Storage) *ActivityLogRepo {
	return &ActivityLogRepo{db: s.db}
}

func (r *ActivityLogRepo) CreateActivityLogs(ctx context.Context, messageID string, items []domain.Activity) error {
	const op = "storage.postgres.CreateActivityLogs"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var msgStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM messages WHERE id = $1`, messageID).Scan(&msgStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, storage.ErrMessageNotFound)
		}
		return fmt.Errorf("%s: check message status: %w", op, err)
	}
	if msgStatus == MessageStatusFailed {
		return fmt.Errorf("%s: %w", op, storage.ErrMessageFailed)
	}

	for _, a := range items {
		result, err := tx.Exec(ctx, `
			INSERT INTO activity_logs (message_id, user_id, description, tag, activity_type, duration_minutes, started_at, completed_at)
			SELECT $1, m.user_id, $2, $3, $4, $5, $6, $7
			FROM messages m
			WHERE m.id = $1
		`, messageID, a.Description, a.Tag, a.ActivityType, a.DurationMinutes, a.StartedAt, a.CompletedAt)
		if err != nil {
			return fmt.Errorf("%s: insert: %w", op, err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("%s: insert: %w", op, storage.ErrMessageNotFound)
		}
	}

	return tx.Commit(ctx)
}
