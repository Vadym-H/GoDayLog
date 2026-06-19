package services

import (
	"context"
	"log/slog"
	"time"
)

type staleMessageRepo interface {
	MarkStalePendingMessagesFailed(ctx context.Context, cutoff time.Time) error
}

func RunStaleCleaner(ctx context.Context, log *slog.Logger, repo staleMessageRepo) {
	const staleness = 10 * time.Minute
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	clean := func() {
		if err := repo.MarkStalePendingMessagesFailed(ctx, time.Now().Add(-staleness)); err != nil {
			log.Warn("stale pending cleaner error", slog.Any("error", err))
		}
	}

	clean()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			clean()
		}
	}
}
