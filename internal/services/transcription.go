package services

import (
	"context"
	"io"
	"log/slog"
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

func (s *TranscriptionService) Transcribe(ctx context.Context, id domain.Identity, audio io.Reader, sizeBytes int64) (string, error) {
	log := logger.From(ctx, s.log)

	if s.limits.MaxAudioBytes > 0 && sizeBytes > int64(s.limits.MaxAudioBytes) {
		return "", ai.ErrAudioTooLarge
	}

	userID, err := s.userCtxReader.GetUserID(ctx, id)
	if err != nil {
		log.Error("transcription: failed to resolve user id", slog.Any("error", err))
		return "", err
	}

	if s.limits.DailyBudgetAudioMinutes > 0 {
		now := time.Now()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		used, sumErr := s.audioLogRepo.SumAudioSecondsSince(ctx, userID, dayStart)
		if sumErr != nil {
			log.Warn("transcription: could not sum audio seconds, skipping budget check", slog.Any("error", sumErr))
		} else {
			budgetSeconds := s.limits.DailyBudgetAudioMinutes * 60
			// conservative pre-check: estimate duration from size at 16 000 bytes/s
			estimated := int(sizeBytes / 16000)
			if used+estimated > budgetSeconds {
				return "", ai.ErrAudioBudgetExceeded
			}
		}
	}

	response, err := s.ai.Transcribe(ctx, audio, "voice.ogg")
	if err != nil {
		log.Error("transcription: ai call failed", slog.Any("error", err))
		return "", err
	}

	if logErr := s.audioLogRepo.CreateTranscriptionLog(ctx, userID, response.Model, int(response.DurationSeconds)); logErr != nil {
		log.Warn("transcription: failed to log usage", slog.Any("error", logErr))
	}

	return response.Text, nil
}
