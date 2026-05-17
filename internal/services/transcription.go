package services

import (
	"context"
	"io"
	"log/slog"
	"math"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
)

type transcriptionAI interface {
	Transcribe(ctx context.Context, audio io.Reader, filename string) (ai.TranscriptionResponse, error)
}

type audioLogRepo interface {
	CreateTranscriptionLog(ctx context.Context, userID, model string, audioSeconds int) error
	SumAudioSecondsSince(ctx context.Context, userID string, since time.Time) (int, error)
}

type TranscriptionService struct {
	log           *slog.Logger
	ai            transcriptionAI
	audioLogRepo  audioLogRepo
	userCtxReader UserContextReader
	limits        config.LLMUsageLimits
	proLimits     config.LLMUsageLimits
}

func NewTranscriptionService(log *slog.Logger, ai transcriptionAI, audioLogRepo audioLogRepo, userCtxReader UserContextReader, limits, proLimits config.LLMUsageLimits) *TranscriptionService {
	return &TranscriptionService{
		log:           log,
		ai:            ai,
		audioLogRepo:  audioLogRepo,
		userCtxReader: userCtxReader,
		limits:        limits,
		proLimits:     proLimits,
	}
}

func (s *TranscriptionService) Transcribe(ctx context.Context, id domain.Identity, audio io.Reader, sizeBytes int64, mimeType string) (string, error) {
	log := logger.From(ctx, s.log)

	if s.limits.MaxAudioBytes > 0 && sizeBytes > int64(s.limits.MaxAudioBytes) {
		return "", ai.ErrAudioTooLarge
	}

	userID, err := s.userCtxReader.GetUserID(ctx, id)
	if err != nil {
		log.Error("transcription: failed to resolve user id", slog.Any("error", err))
		return "", err
	}

	// Fix 5+8: pre-check using only historical usage (no size estimate);
	// day boundary anchored to the user's local timezone.
	if s.limits.DailyBudgetAudioMinutes > 0 {
		tzName, _ := s.userCtxReader.GetUserTimezone(ctx, id)
		loc, locErr := time.LoadLocation(tzName)
		if locErr != nil {
			loc = time.UTC
		}
		now := time.Now()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		used, sumErr := s.audioLogRepo.SumAudioSecondsSince(ctx, userID, dayStart)
		if sumErr != nil {
			log.Warn("transcription: could not sum audio seconds, skipping budget check", slog.Any("error", sumErr))
		} else if used >= s.limits.DailyBudgetAudioMinutes*60 {
			return "", ai.ErrAudioBudgetExceeded
		}
	}

	// Fix 6: derive filename from caller-supplied MIME type
	response, err := s.ai.Transcribe(ctx, audio, mimeToFilename(mimeType))
	if err != nil {
		log.Error("transcription: ai call failed", slog.Any("error", err))
		return "", err
	}

	// Fix 7: ceil to avoid undercounting fractional seconds
	actualSeconds := int(math.Ceil(response.DurationSeconds))

	if logErr := s.audioLogRepo.CreateTranscriptionLog(ctx, userID, response.Model, actualSeconds); logErr != nil {
		log.Warn("transcription: failed to log usage", slog.Any("error", logErr))
	}

	return response.Text, nil
}

func mimeToFilename(mimeType string) string {
	switch mimeType {
	case "audio/mpeg", "audio/mp3":
		return "audio.mp3"
	case "audio/mp4", "audio/m4a":
		return "audio.m4a"
	case "audio/wav":
		return "audio.wav"
	case "audio/webm":
		return "audio.webm"
	case "audio/flac":
		return "audio.flac"
	default:
		return "voice.ogg"
	}
}
