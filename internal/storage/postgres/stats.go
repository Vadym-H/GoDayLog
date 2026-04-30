package storage

import (
	"context"
	"fmt"

	"github.com/Vadym-H/GoDayLog/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatsRepo struct {
	db *pgxpool.Pool
}

func NewStatsRepo(s *Storage) *StatsRepo {
	return &StatsRepo{db: s.db}
}

func (r *StatsRepo) GetStats(ctx context.Context, q services.StatsQuery) (services.StatsReport, error) {
	const op = "storage.postgres.GetStats"

	rows, err := r.db.Query(ctx, `
		SELECT
			activity_type,
			COUNT(*)                                         AS count,
			COALESCE(SUM(duration_minutes), 0)               AS total_minutes,
			COUNT(*) FILTER (WHERE duration_minutes IS NULL) AS untracked_count
		FROM activity_logs
		WHERE user_id    = $1
		  AND deleted_at IS NULL
		  AND COALESCE(started_at, created_at) >= $2
		  AND COALESCE(started_at, created_at) <  $3
		GROUP BY activity_type
	`, q.UserID, q.From, q.To)
	if err != nil {
		return services.StatsReport{}, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	typeMap := make(map[string]services.TypeSummary, 4)
	var totalCount, totalMinutes, totalUntracked int

	for rows.Next() {
		var ts services.TypeSummary
		if err := rows.Scan(&ts.Type, &ts.Count, &ts.TotalMinutes, &ts.UntrackedCount); err != nil {
			return services.StatsReport{}, fmt.Errorf("%s: scan: %w", op, err)
		}
		typeMap[ts.Type] = ts
		totalCount += ts.Count
		totalMinutes += ts.TotalMinutes
		totalUntracked += ts.UntrackedCount
	}
	if err := rows.Err(); err != nil {
		return services.StatsReport{}, fmt.Errorf("%s: rows: %w", op, err)
	}

	// preserve canonical display order
	order := []string{"growth", "routine", "rest", "drain"}
	byType := make([]services.TypeSummary, 0, 4)
	for _, t := range order {
		if ts, ok := typeMap[t]; ok {
			byType = append(byType, ts)
		}
	}

	return services.StatsReport{
		From:           q.From,
		To:             q.To,
		TotalCount:     totalCount,
		TotalMinutes:   totalMinutes,
		UntrackedCount: totalUntracked,
		ByType:         byType,
	}, nil
}
