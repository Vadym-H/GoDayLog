package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

func (r *ActivityLogRepo) CreateActivityLogs(ctx context.Context, messageID, userID string, items []domain.Activity) error {
	const op = "storage.postgres.CreateActivityLogs"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var msgStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM messages WHERE id = $1 AND user_id = $2`, messageID, userID).Scan(&msgStatus)
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
		_, err := tx.Exec(ctx, `
			INSERT INTO activity_logs (message_id, user_id, description, tag, activity_type, duration_minutes, started_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, messageID, userID, a.Description, a.Tag, a.ActivityType, a.DurationMinutes, a.StartedAt, a.CompletedAt)
		if err != nil {
			return fmt.Errorf("%s: insert: %w", op, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *ActivityLogRepo) GetActivityLogs(ctx context.Context, userID string, limit, offset int) ([]domain.ActivityLog, error) {
	const op = "storage.postgres.GetActivityLogs"

	rows, err := r.db.Query(ctx, `
		SELECT id, message_id, description, tag, activity_type, duration_minutes, started_at, completed_at, created_at
		FROM activity_logs
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	var logs []domain.ActivityLog
	for rows.Next() {
		var a domain.ActivityLog
		if err := rows.Scan(&a.ID, &a.MessageID, &a.Description, &a.Tag, &a.ActivityType,
			&a.DurationMinutes, &a.StartedAt, &a.CompletedAt, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		logs = append(logs, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	return logs, nil
}

func (r *ActivityLogRepo) GetActivityLog(ctx context.Context, activityID, userID string) (domain.ActivityLog, error) {
	const op = "storage.postgres.GetActivityLog"

	var a domain.ActivityLog
	err := r.db.QueryRow(ctx, `
		SELECT id, message_id, description, tag, activity_type, duration_minutes, started_at, completed_at, created_at
		FROM activity_logs
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, activityID, userID).Scan(&a.ID, &a.MessageID, &a.Description, &a.Tag, &a.ActivityType,
		&a.DurationMinutes, &a.StartedAt, &a.CompletedAt, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ActivityLog{}, fmt.Errorf("%s: %w", op, storage.ErrActivityNotFound)
		}
		return domain.ActivityLog{}, fmt.Errorf("%s: %w", op, err)
	}

	return a, nil
}

func (r *ActivityLogRepo) GetActivityLogsByMessage(ctx context.Context, messageID, userID string) ([]domain.ActivityLog, error) {
	const op = "storage.postgres.GetActivityLogsByMessage"

	rows, err := r.db.Query(ctx, `
		SELECT id, message_id, description, tag, activity_type, duration_minutes, started_at, completed_at, created_at
		FROM activity_logs
		WHERE message_id = $1 AND user_id = $2 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`, messageID, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	var logs []domain.ActivityLog
	for rows.Next() {
		var a domain.ActivityLog
		if err := rows.Scan(&a.ID, &a.MessageID, &a.Description, &a.Tag, &a.ActivityType,
			&a.DurationMinutes, &a.StartedAt, &a.CompletedAt, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		logs = append(logs, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	return logs, nil
}

func (r *ActivityLogRepo) SoftDeleteActivityLog(ctx context.Context, activityID, userID string) error {
	const op = "storage.postgres.SoftDeleteActivityLog"

	tag, err := r.db.Exec(ctx, `
		UPDATE activity_logs
		SET deleted_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, activityID, userID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrActivityNotFound)
	}

	return nil
}

func (r *ActivityLogRepo) UpdateActivityLog(ctx context.Context, activityID, userID string, fields domain.UpdateActivityFields) error {
	const op = "storage.postgres.UpdateActivityLog"

	setClauses := make([]string, 0, 5)
	args := []any{activityID, userID}
	n := 3

	if fields.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", n))
		args = append(args, *fields.Description)
		n++
	}
	if fields.Tag != nil {
		setClauses = append(setClauses, fmt.Sprintf("tag = $%d", n))
		args = append(args, *fields.Tag)
		n++
	}
	if fields.ActivityType != nil {
		setClauses = append(setClauses, fmt.Sprintf("activity_type = $%d", n))
		args = append(args, *fields.ActivityType)
		n++
	}
	if fields.DurationMinutes != nil {
		setClauses = append(setClauses, fmt.Sprintf("duration_minutes = $%d", n))
		args = append(args, *fields.DurationMinutes)
		n++
	}
	if fields.StartedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("started_at = $%d", n))
		args = append(args, *fields.StartedAt)
		n++
	}

	if len(setClauses) == 0 {
		return nil
	}

	query := fmt.Sprintf(`
		UPDATE activity_logs
		SET %s
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, strings.Join(setClauses, ", "))

	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrActivityNotFound)
	}

	return nil
}
