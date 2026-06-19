package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/logger"
)

type TranscriptionResponse struct {
	Text            string
	DurationSeconds float64
	Model           string
}

type transcriptionAPIResponse struct {
	Text     string  `json:"text"`
	Duration float64 `json:"duration"`
}

func (c *Client) Transcribe(ctx context.Context, audio io.Reader, filename string) (TranscriptionResponse, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return TranscriptionResponse{}, fmt.Errorf("ai: create form file: %w", err)
	}
	if _, err = io.Copy(fw, audio); err != nil {
		return TranscriptionResponse{}, fmt.Errorf("ai: write audio: %w", err)
	}
	if err = mw.WriteField("model", c.whisperModel); err != nil {
		return TranscriptionResponse{}, fmt.Errorf("ai: write model field: %w", err)
	}
	if err = mw.WriteField("response_format", "verbose_json"); err != nil {
		return TranscriptionResponse{}, fmt.Errorf("ai: write response_format field: %w", err)
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return TranscriptionResponse{}, fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return TranscriptionResponse{}, fmt.Errorf("ai: request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.From(ctx, c.log).Warn("ai: close response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return TranscriptionResponse{}, fmt.Errorf("ai: unexpected status %d: %s", resp.StatusCode, bytes.TrimSpace(errBody))
	}

	var result transcriptionAPIResponse
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return TranscriptionResponse{}, fmt.Errorf("ai: decode response: %w", err)
	}

	return TranscriptionResponse{
		Text:            result.Text,
		DurationSeconds: result.Duration,
		Model:           c.whisperModel,
	}, nil
}
