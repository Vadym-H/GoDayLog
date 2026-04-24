package storage

import (
	"context"
	"fmt"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ActivityLogRepo struct {
	db *pgxpool.Pool
}

func NewActivityLogRepo(s *Storage) *ActivityLogRepo {
	return &ActivityLogRepo{db: s.db}
}

func (r *ActivityLogRepo) CreateActivityLog(ctx context.Context, messageID string, a ai.ActivityExtraction) error {
	const op = "storage.postgres.CreateActivityLog"

	_, err := r.db.Exec(ctx, `
		INSERT INTO activity_logs (message_id, user_id, description, tag, is_useful, duration_minutes, started_at)
		SELECT $1, m.user_id, $2, $3, $4, $5, $6
		FROM messages m
		WHERE m.id = $1
	`, messageID, a.Description, a.Tag, a.IsUseful, a.DurationMinutes, a.StartedAt)
	if err != nil {
		return fmt.Errorf("%s: insert: %w", op, err)
	}

	return nil
}
