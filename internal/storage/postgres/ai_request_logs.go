package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIRequestLogRepo struct {
	db *pgxpool.Pool
}

func NewAIRequestLogRepo(s *Storage) *AIRequestLogRepo {
	return &AIRequestLogRepo{db: s.db}
}

func (r *AIRequestLogRepo) SumTokensSince(ctx context.Context, userID string, since time.Time) (int, error) {
	const op = "storage.postgres.SumTokensSince"

	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_tokens), 0)
		FROM ai_request_logs
		WHERE user_id = $1 AND created_at >= $2
	`, userID, since).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return total, nil
}

func (r *AIRequestLogRepo) CreateAIRequestLog(ctx context.Context, messageID, model string, usage ai.TokenUsage) error {
	const op = "storage.postgres.CreateAIRequestLog"

	result, err := r.db.Exec(ctx, `
		INSERT INTO ai_request_logs (user_id, message_id, prompt_tokens, completion_tokens, total_tokens, model)
		SELECT m.user_id, $1, $2, $3, $4, $5
		FROM messages m
		WHERE m.id = $1
	`, messageID, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, model)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrMessageNotFound)
	}

	return nil
}
