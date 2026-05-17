package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StatsRepo struct {
	db *pgxpool.Pool
}

func NewStatsRepo(s *Storage) *StatsRepo {
	return &StatsRepo{db: s.db}
}

func (r *StatsRepo) GetStats(ctx context.Context, q domain.StatsQuery) (domain.StatsReport, error) {
	const op = "storage.postgres.GetStats"

	rows, err := r.db.Query(ctx, `
		SELECT
			activity_type,
			COUNT(*)                                         AS count,
			COALESCE(SUM(duration_minutes), 0)               AS total_minutes,
			COUNT(*) FILTER (WHERE duration_minutes IS NULL) AS untracked_count,
			MIN(MIN(COALESCE(started_at, created_at)))           OVER () AS first_activity,
			MAX(MAX(COALESCE(completed_at, created_at)))         OVER () AS last_activity
		FROM activity_logs
		WHERE user_id    = $1
		  AND deleted_at IS NULL
		  AND COALESCE(started_at, created_at) >= $2
		  AND COALESCE(started_at, created_at) <  $3
		GROUP BY activity_type
	`, q.UserID, q.From, q.To)
	if err != nil {
		return domain.StatsReport{}, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	typeMap := make(map[string]domain.TypeSummary, 4)
	var totalCount, totalMinutes, totalUntracked int
	var firstActivity, lastActivity *time.Time

	for rows.Next() {
		var ts domain.TypeSummary
		var rowFirst, rowLast *time.Time
		if err := rows.Scan(&ts.Type, &ts.Count, &ts.TotalMinutes, &ts.UntrackedCount, &rowFirst, &rowLast); err != nil {
			return domain.StatsReport{}, fmt.Errorf("%s: scan: %w", op, err)
		}
		if firstActivity == nil {
			firstActivity = rowFirst
			lastActivity = rowLast
		}
		typeMap[ts.Type] = ts
		totalCount += ts.Count
		totalMinutes += ts.TotalMinutes
		totalUntracked += ts.UntrackedCount
	}
	if err := rows.Err(); err != nil {
		return domain.StatsReport{}, fmt.Errorf("%s: rows: %w", op, err)
	}

	// preserve canonical display order
	order := []string{"growth", "routine", "rest", "drain"}
	byType := make([]domain.TypeSummary, 0, 4)
	for _, t := range order {
		if ts, ok := typeMap[t]; ok {
			byType = append(byType, ts)
		}
	}

	tagRows, err := r.db.Query(ctx, `
		SELECT
			tag,
			COUNT(*)                           AS count,
			COALESCE(SUM(duration_minutes), 0) AS total_minutes
		FROM activity_logs
		WHERE user_id   = $1
		  AND deleted_at IS NULL
		  AND COALESCE(started_at, created_at) >= $2
		  AND COALESCE(started_at, created_at) <  $3
		GROUP BY tag
		ORDER BY count DESC
		LIMIT 5
	`, q.UserID, q.From, q.To)
	if err != nil {
		return domain.StatsReport{}, fmt.Errorf("%s: tag query: %w", op, err)
	}
	defer tagRows.Close()

	var topTags []domain.TagSummary
	for tagRows.Next() {
		var ts domain.TagSummary
		if err := tagRows.Scan(&ts.Tag, &ts.Count, &ts.TotalMinutes); err != nil {
			return domain.StatsReport{}, fmt.Errorf("%s: tag scan: %w", op, err)
		}
		topTags = append(topTags, ts)
	}
	if err := tagRows.Err(); err != nil {
		return domain.StatsReport{}, fmt.Errorf("%s: tag rows: %w", op, err)
	}

	var byDay []domain.DaySummary
	if q.To.Sub(q.From) > 24*time.Hour {
		tz := q.Timezone
		if tz == "" {
			tz = "UTC"
		}
		dayRows, err := r.db.Query(ctx, `
			SELECT
				DATE(COALESCE(started_at, created_at) AT TIME ZONE $4) AS day,
				activity_type,
				COUNT(*)                                                AS count,
				COALESCE(SUM(duration_minutes), 0)                     AS total_minutes
			FROM activity_logs
			WHERE user_id   = $1
			  AND deleted_at IS NULL
			  AND COALESCE(started_at, created_at) >= $2
			  AND COALESCE(started_at, created_at) <  $3
			GROUP BY day, activity_type
			ORDER BY day
		`, q.UserID, q.From, q.To, tz)
		if err != nil {
			return domain.StatsReport{}, fmt.Errorf("%s: day query: %w", op, err)
		}
		defer dayRows.Close()

		dayMap := make(map[time.Time]*domain.DaySummary)
		var dayOrder []time.Time
		for dayRows.Next() {
			var day time.Time
			var ts domain.TypeSummary
			if err := dayRows.Scan(&day, &ts.Type, &ts.Count, &ts.TotalMinutes); err != nil {
				return domain.StatsReport{}, fmt.Errorf("%s: day scan: %w", op, err)
			}
			ds, exists := dayMap[day]
			if !exists {
				ds = &domain.DaySummary{Date: day}
				dayMap[day] = ds
				dayOrder = append(dayOrder, day)
			}
			ds.Count += ts.Count
			ds.TotalMinutes += ts.TotalMinutes
			ds.ByType = append(ds.ByType, ts)
		}
		if err := dayRows.Err(); err != nil {
			return domain.StatsReport{}, fmt.Errorf("%s: day rows: %w", op, err)
		}

		byDay = make([]domain.DaySummary, len(dayOrder))
		for i, d := range dayOrder {
			byDay[i] = *dayMap[d]
		}
	}
	return domain.StatsReport{
		From:            q.From,
		To:              q.To,
		FirstActivityAt: firstActivity,
		LastActivityAt:  lastActivity,
		TotalCount:      totalCount,
		TotalMinutes:    totalMinutes,
		UntrackedCount:  totalUntracked,
		ByType:          byType,
		TopTags:         topTags,
		ByDay:           byDay,
	}, nil
}
