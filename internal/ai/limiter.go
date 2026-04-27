package ai

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

type Extractor interface {
	ExtractActivity(ctx context.Context, today, userContext, userMessage string) (ActivityResponse, error)
}

type LimiterMiddleware struct {
	inner  Extractor
	log    *slog.Logger
	limits config.LLMUsageLimits
}

func NewLimiterMiddleware(inner Extractor, log *slog.Logger, limits config.LLMUsageLimits) *LimiterMiddleware {
	return &LimiterMiddleware{inner: inner, log: log, limits: limits}
}

func (l *LimiterMiddleware) ExtractActivity(ctx context.Context, today, userContext, userMessage string) (ActivityResponse, error) {
	if l.limits.Enabled && l.limits.MaxInputTokens > 0 {
		// chars/4 approximation; activityPrompt is fixed overhead included so the limit is meaningful
		estimate := (len(activityPrompt) + len(userContext) + len(userMessage)) / 4
		logger.From(ctx, l.log).Debug("token estimate for input", slog.Int("estimate", estimate))
		if estimate > l.limits.MaxInputTokens {
			return ActivityResponse{}, ErrInputTooLong
		}
	}
	return l.inner.ExtractActivity(ctx, today, userContext, userMessage)
}
