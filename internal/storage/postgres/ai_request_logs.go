package storage

import (
	"context"
	"fmt"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIRequestLogRepo struct {
	db *pgxpool.Pool
}

func NewAIRequestLogRepo(s *Storage) *AIRequestLogRepo {
	return &AIRequestLogRepo{db: s.db}
}

func (r *AIRequestLogRepo) CreateAIRequestLog(ctx context.Context, messageID, model string, usage ai.TokenUsage) error {
	const op = "storage.postgres.CreateAIRequestLog"

	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_request_logs (user_id, message_id, prompt_tokens, completion_tokens, total_tokens, model)
		SELECT m.user_id, $1, $2, $3, $4, $5
		FROM messages m
		WHERE m.id = $1
	`, messageID, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, model)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
