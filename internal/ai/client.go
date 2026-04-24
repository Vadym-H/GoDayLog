package ai

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/config"
)

//go:embed prompts/activity.txt
var activityPrompt string

type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
	model   string
}

func New(cfg config.LLMConfig) *Client {
	return &Client{
		http:    &http.Client{Timeout: cfg.Timeout},
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
	}
}

type ActivityExtraction struct {
	Valid           bool       `json:"valid"`
	Description     string     `json:"description"`
	Tag             string     `json:"tag"`
	IsUseful        bool       `json:"is_useful"`
	DurationMinutes *int       `json:"duration_minutes"`
	StartedAt       *time.Time `json:"started_at"`
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
}

func (c *Client) ExtractActivity(ctx context.Context, userContext, userMessage string) (ActivityExtraction, error) {
	messages := []chatMessage{
		{Role: "system", Content: activityPrompt},
	}
	if userContext != "" {
		messages = append(messages, chatMessage{Role: "user", Content: "My context: " + userContext})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userMessage})

	body, err := json.Marshal(completionRequest{Model: c.model, Messages: messages})
	if err != nil {
		return ActivityExtraction{}, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ActivityExtraction{}, fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return ActivityExtraction{}, fmt.Errorf("ai: request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return ActivityExtraction{}, fmt.Errorf("ai: unexpected status %d", resp.StatusCode)
	}

	var result completionResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ActivityExtraction{}, fmt.Errorf("ai: decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return ActivityExtraction{}, fmt.Errorf("ai: empty choices in response")
	}

	var extraction ActivityExtraction
	if err = json.Unmarshal([]byte(result.Choices[0].Message.Content), &extraction); err != nil {
		return ActivityExtraction{}, fmt.Errorf("ai: parse extraction: %w", err)
	}

	return extraction, nil
}
