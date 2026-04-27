package ai

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

//go:embed prompts/activity.txt
var activityPrompt string

type Client struct {
	http    *http.Client
	log     *slog.Logger
	baseURL string
	apiKey  string
	model   string
	limits  config.LLMUsageLimits
}

func New(log *slog.Logger, cfg config.LLMConfig) *Client {
	return &Client{
		http:    &http.Client{Timeout: cfg.Timeout},
		log:     log,
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		limits:  cfg.LLMLimits,
	}
}

type ActivityItem struct {
	Description     string     `json:"description"`
	Tag             string     `json:"tag"`
	ActivityType    string     `json:"activity_type"`
	DurationMinutes *int       `json:"duration_minutes"`
	StartedAt       *time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type ActivityResponse struct {
	Valid      bool           `json:"valid"`
	Activities []ActivityItem `json:"activities"`
	Usage      TokenUsage
	Model      string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type completionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type completionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *Client) ExtractActivity(ctx context.Context, today, userContext, userMessage string) (ActivityResponse, error) {
	systemPrompt := activityPrompt + "\n\nToday's date: " + today

	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
	}
	if userContext != "" {
		messages = append(messages, chatMessage{Role: "user", Content: "My context: " + userContext})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userMessage})

	if c.limits.Enabled && c.limits.MaxInputTokens > 0 {
		// chars/4 is a fast approximation; intentionally avoids a tokeniser dependency
		estimate := (len(systemPrompt) + len(userContext) + len(userMessage)) / 4
		logger.From(ctx, c.log).Debug("token estimate for input messages", slog.Int("estimate", estimate))
		if estimate > c.limits.MaxInputTokens {
			return ActivityResponse{}, ErrInputTooLong
		}
	}

	body, err := json.Marshal(completionRequest{Model: c.model, Messages: messages})
	if err != nil {
		return ActivityResponse{}, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ActivityResponse{}, fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return ActivityResponse{}, fmt.Errorf("ai: request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.From(ctx, c.log).Warn("ai: close response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return ActivityResponse{}, fmt.Errorf("ai: unexpected status %d: %s", resp.StatusCode, bytes.TrimSpace(errBody))
	}

	var result completionResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ActivityResponse{}, fmt.Errorf("ai: decode response: %w", err)
	}

	usage := TokenUsage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		TotalTokens:      result.Usage.TotalTokens,
	}

	if len(result.Choices) == 0 {
		return ActivityResponse{Usage: usage, Model: c.model}, fmt.Errorf("ai: empty choices in response")
	}

	var response ActivityResponse
	if err = json.Unmarshal([]byte(result.Choices[0].Message.Content), &response); err != nil {
		return ActivityResponse{Usage: usage, Model: c.model}, fmt.Errorf("ai: parse extraction: %w", err)
	}

	response.Usage = usage
	response.Model = c.model
	return response, nil
}
