package handlers

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/services"
)

type userService interface {
	RegisterUser(ctx context.Context, id domain.Identity) (string, bool, error)
	GetUserContext(ctx context.Context, id domain.Identity) (string, error)
	UpdateUserContext(ctx context.Context, id domain.Identity, llmContext string) error
	UpdateUserTimezone(ctx context.Context, id domain.Identity, timezone string) error
	GetUserID(ctx context.Context, id domain.Identity) (string, error)
	GetUserTimezone(ctx context.Context, id domain.Identity) (string, error)
}

type messageService interface {
	SaveMessage(ctx context.Context, id domain.Identity, externalMessageID, text string) (string, error)
}

type aiProcessor interface {
	ExtractActivities(ctx context.Context, id domain.Identity, messageID, text string) ([]domain.Activity, error)
	SaveActivities(ctx context.Context, messageID string, activities []domain.Activity) error
	CancelReview(ctx context.Context, messageID string) error
}

type statsGetter interface {
	GetStats(ctx context.Context, q services.StatsQuery) (services.StatsReport, error)
}

type statsAnalyser interface {
	Analyse(ctx context.Context, q services.StatsQuery, report services.StatsReport, userContext string) (string, error)
}

type Handlers struct {
	log           *slog.Logger
	cfg           *config.Config
	userSvc       userService
	messageSvc    messageService
	aiProc        aiProcessor
	statsSvc      statsGetter
	statsAnalyser statsAnalyser
}

func New(
	log *slog.Logger,
	cfg *config.Config,
	userSvc userService,
	messageSvc messageService,
	aiProc aiProcessor,
	statsSvc statsGetter,
	analyser statsAnalyser,
) *Handlers {
	return &Handlers{
		log:           log,
		cfg:           cfg,
		userSvc:       userSvc,
		messageSvc:    messageSvc,
		aiProc:        aiProc,
		statsSvc:      statsSvc,
		statsAnalyser: analyser,
	}
}
