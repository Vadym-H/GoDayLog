package handlers

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/config"
	"github.com/Vadym-H/GoDayLog/internal/domain"
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
	SaveActivities(ctx context.Context, id domain.Identity, messageID string, activities []domain.Activity) error
	CancelReview(ctx context.Context, id domain.Identity, messageID string) error
}

type statsGetter interface {
	GetStats(ctx context.Context, q domain.StatsQuery) (domain.StatsReport, error)
}

type statsAnalyser interface {
	Analyse(ctx context.Context, q domain.StatsQuery, report domain.StatsReport, userContext string) (string, error)
}

type activityLogService interface {
	GetHistory(ctx context.Context, id domain.Identity, limit, offset int) ([]domain.ActivityLog, error)
	GetMessageWithActivities(ctx context.Context, id domain.Identity, messageID string) (domain.MessageWithActivities, error)
	DeleteActivity(ctx context.Context, id domain.Identity, activityID string) error
	UpdateActivity(ctx context.Context, id domain.Identity, activityID string, fields domain.UpdateActivityFields) (domain.ActivityLog, error)
}

type Handlers struct {
	log           *slog.Logger
	cfg           *config.Config
	userSvc       userService
	messageSvc    messageService
	aiProc        aiProcessor
	statsSvc      statsGetter
	statsAnalyser statsAnalyser
	activitySvc   activityLogService
}

func New(
	log *slog.Logger,
	cfg *config.Config,
	userSvc userService,
	messageSvc messageService,
	aiProc aiProcessor,
	statsSvc statsGetter,
	analyser statsAnalyser,
	activitySvc activityLogService,
) *Handlers {
	return &Handlers{
		log:           log,
		cfg:           cfg,
		userSvc:       userSvc,
		messageSvc:    messageSvc,
		aiProc:        aiProc,
		statsSvc:      statsSvc,
		statsAnalyser: analyser,
		activitySvc:   activitySvc,
	}
}
