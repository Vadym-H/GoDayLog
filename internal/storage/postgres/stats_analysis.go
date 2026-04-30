package storage

import (
	"context"
	"fmt"

	"github.com/Vadym-H/GoDayLog/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatsAnalysisRepo struct {
	db *pgxpool.Pool
}

func NewStatsAnalysisRepo(s *Storage) *StatsAnalysisRepo {
	return &StatsAnalysisRepo{db: s.db}
}

func (r *StatsAnalysisRepo) GetActivityEntries(ctx context.Context, q services.StatsQuery) ([]services.ActivityEntry, error) {
	const op = "storage.postgres.GetActivityEntries"

	rows, err := r.db.Query(ctx, `
		SELECT description, tag, activity_type, duration_minutes, started_at
		FROM activity_logs
		WHERE user_id    = $1
		  AND deleted_at IS NULL
		  AND COALESCE(started_at, created_at) >= $2
		  AND COALESCE(started_at, created_at) <  $3
		ORDER BY COALESCE(started_at, created_at)
	`, q.UserID, q.From, q.To)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	var entries []services.ActivityEntry
	for rows.Next() {
		var e services.ActivityEntry
		if err := rows.Scan(&e.Description, &e.Tag, &e.ActivityType, &e.DurationMinutes, &e.StartedAt); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	return entries, nil
}
