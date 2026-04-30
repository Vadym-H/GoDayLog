package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/logger"
)

type StatsQuery struct {
	UserID   string
	From     time.Time
	To       time.Time
	Timezone string
}

type TypeSummary struct {
	Type           string
	Count          int
	TotalMinutes   int
	UntrackedCount int
}

type TagSummary struct {
	Tag          string
	Count        int
	TotalMinutes int
}

type DaySummary struct {
	Date         time.Time
	Count        int
	TotalMinutes int
	ByType       []TypeSummary
}

type StatsReport struct {
	From           time.Time
	To             time.Time
	TotalCount     int
	TotalMinutes   int
	UntrackedCount int
	ByType         []TypeSummary
	TopTags        []TagSummary
	ByDay          []DaySummary
}

type StatsRepository interface {
	GetStats(ctx context.Context, q StatsQuery) (StatsReport, error)
}

type StatsService struct {
	log  *slog.Logger
	repo StatsRepository
}

func NewStatsService(log *slog.Logger, repo StatsRepository) *StatsService {
	return &StatsService{log: log, repo: repo}
}

func (s *StatsService) GetStats(ctx context.Context, q StatsQuery) (StatsReport, error) {
	report, err := s.repo.GetStats(ctx, q)
	if err != nil {
		logger.From(ctx, s.log).Error("stats query failed", slog.Any("error", err))
		return StatsReport{}, fmt.Errorf("stats: %w", err)
	}
	return report, nil
}
