package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

type StatsRepository interface {
	GetStats(ctx context.Context, q domain.StatsQuery) (domain.StatsReport, error)
}

type StatsService struct {
	log  *slog.Logger
	repo StatsRepository
}

func NewStatsService(log *slog.Logger, repo StatsRepository) *StatsService {
	return &StatsService{log: log, repo: repo}
}

func (s *StatsService) GetStats(ctx context.Context, q domain.StatsQuery) (domain.StatsReport, error) {
	report, err := s.repo.GetStats(ctx, q)
	if err != nil {
		logger.From(ctx, s.log).Error("stats query failed", slog.Any("error", err))
		return domain.StatsReport{}, fmt.Errorf("stats: %w", err)
	}
	return report, nil
}
