package tghandlers

import (
	"context"
	"log/slog"
	"sync"
	"time"

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

type timezoneLookup interface {
	GetTimezoneName(lng, lat float64) string
}

type statsService interface {
	GetStats(ctx context.Context, q services.StatsQuery) (services.StatsReport, error)
}

type statsAnalyser interface {
	Analyse(ctx context.Context, q services.StatsQuery, report services.StatsReport, userContext string) (string, error)
}

type pendingStatsState struct {
	from   time.Time
	to     time.Time
	loc    *time.Location
	tzName string
	report services.StatsReport
}

type messageService interface {
	SaveMessage(ctx context.Context, id domain.Identity, externalMessageID, text string) (string, error)
}

type messageAIProcessor interface {
	ExtractActivities(ctx context.Context, id domain.Identity, messageID, text string) ([]domain.Activity, error)
	SaveActivities(ctx context.Context, messageID string, activities []domain.Activity) error
	CancelReview(ctx context.Context, messageID string) error
}

// pendingReview holds the in-progress AI review state for one chat.
// mu guards activities and editIndex — both are mutated by concurrent handler goroutines.
type pendingReview struct {
	mu         sync.Mutex
	messageID  string
	identity   domain.Identity
	activities []domain.Activity
	editIndex  int // -1 = no activity selected for editing
}

type TgHandlers struct {
	log            *slog.Logger
	userService    userService
	messageService messageService
	aiProcessor    messageAIProcessor
	statsService   statsService
	statsAnalyser  statsAnalyser
	tzFinder       timezoneLookup

	pendingMu          sync.RWMutex
	pendingLog         map[int64]struct{}
	pendingLlmCtx      map[int64]struct{}
	pendingReview      map[int64]*pendingReview
	awaitingTag        map[int64]struct{}
	awaitingStartedAt  map[int64]struct{}
	awaitingStatsRange map[int64]struct{}
	awaitingLocation   map[int64]struct{}
	pendingOnboarding  map[int64]struct{}
	pendingStats       map[int64]*pendingStatsState
}

func New(log *slog.Logger, userService userService, messageService messageService, aiProcessor messageAIProcessor, statsService statsService, analyser statsAnalyser, tzFinder timezoneLookup) *TgHandlers {
	return &TgHandlers{
		log:                log,
		userService:        userService,
		messageService:     messageService,
		aiProcessor:        aiProcessor,
		statsService:       statsService,
		statsAnalyser:      analyser,
		tzFinder:           tzFinder,
		pendingLog:         make(map[int64]struct{}),
		pendingLlmCtx:      make(map[int64]struct{}),
		pendingReview:      make(map[int64]*pendingReview),
		awaitingTag:        make(map[int64]struct{}),
		awaitingStartedAt:  make(map[int64]struct{}),
		awaitingStatsRange: make(map[int64]struct{}),
		awaitingLocation:   make(map[int64]struct{}),
		pendingOnboarding:  make(map[int64]struct{}),
		pendingStats:       make(map[int64]*pendingStatsState),
	}
}

func (tg *TgHandlers) setAwaitingLog(chatID int64) {
	tg.pendingMu.Lock()
	tg.pendingLog[chatID] = struct{}{}
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) consumeAwaitingLog(chatID int64) bool {
	tg.pendingMu.Lock()
	defer tg.pendingMu.Unlock()
	if _, ok := tg.pendingLog[chatID]; !ok {
		return false
	}
	delete(tg.pendingLog, chatID)
	return true
}

func (tg *TgHandlers) clearAwaitingLog(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.pendingLog, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) isAwaitingLog(chatID int64) bool {
	tg.pendingMu.RLock()
	defer tg.pendingMu.RUnlock()
	_, ok := tg.pendingLog[chatID]
	return ok
}

func (tg *TgHandlers) setAwaitingContext(chatID int64) {
	tg.pendingMu.Lock()
	tg.pendingLlmCtx[chatID] = struct{}{}
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) consumeAwaitingContext(chatID int64) bool {
	tg.pendingMu.Lock()
	defer tg.pendingMu.Unlock()
	if _, ok := tg.pendingLlmCtx[chatID]; !ok {
		return false
	}
	delete(tg.pendingLlmCtx, chatID)
	return true
}

func (tg *TgHandlers) clearAwaitingContext(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.pendingLlmCtx, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) isAwaitingContext(chatID int64) bool {
	tg.pendingMu.RLock()
	defer tg.pendingMu.RUnlock()
	_, ok := tg.pendingLlmCtx[chatID]
	return ok
}

func (tg *TgHandlers) setPendingReview(chatID int64, r *pendingReview) {
	tg.pendingMu.Lock()
	tg.pendingReview[chatID] = r
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) getPendingReview(chatID int64) *pendingReview {
	tg.pendingMu.RLock()
	defer tg.pendingMu.RUnlock()
	return tg.pendingReview[chatID]
}

func (tg *TgHandlers) clearPendingReview(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.pendingReview, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) setAwaitingTag(chatID int64) {
	tg.pendingMu.Lock()
	tg.awaitingTag[chatID] = struct{}{}
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) consumeAwaitingTag(chatID int64) bool {
	tg.pendingMu.Lock()
	defer tg.pendingMu.Unlock()
	if _, ok := tg.awaitingTag[chatID]; !ok {
		return false
	}
	delete(tg.awaitingTag, chatID)
	return true
}

func (tg *TgHandlers) clearAwaitingTag(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.awaitingTag, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) setAwaitingStartedAt(chatID int64) {
	tg.pendingMu.Lock()
	tg.awaitingStartedAt[chatID] = struct{}{}
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) consumeAwaitingStartedAt(chatID int64) bool {
	tg.pendingMu.Lock()
	defer tg.pendingMu.Unlock()
	if _, ok := tg.awaitingStartedAt[chatID]; !ok {
		return false
	}
	delete(tg.awaitingStartedAt, chatID)
	return true
}

func (tg *TgHandlers) clearAwaitingStartedAt(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.awaitingStartedAt, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) setAwaitingStatsRange(chatID int64) {
	tg.pendingMu.Lock()
	tg.awaitingStatsRange[chatID] = struct{}{}
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) consumeAwaitingStatsRange(chatID int64) bool {
	tg.pendingMu.Lock()
	defer tg.pendingMu.Unlock()
	if _, ok := tg.awaitingStatsRange[chatID]; !ok {
		return false
	}
	delete(tg.awaitingStatsRange, chatID)
	return true
}

func (tg *TgHandlers) clearAwaitingStatsRange(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.awaitingStatsRange, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) setPendingStats(chatID int64, s *pendingStatsState) {
	tg.pendingMu.Lock()
	tg.pendingStats[chatID] = s
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) getPendingStats(chatID int64) *pendingStatsState {
	tg.pendingMu.RLock()
	defer tg.pendingMu.RUnlock()
	return tg.pendingStats[chatID]
}

func (tg *TgHandlers) clearPendingStats(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.pendingStats, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) setAwaitingLocation(chatID int64) {
	tg.pendingMu.Lock()
	tg.awaitingLocation[chatID] = struct{}{}
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) consumeAwaitingLocation(chatID int64) bool {
	tg.pendingMu.Lock()
	defer tg.pendingMu.Unlock()
	if _, ok := tg.awaitingLocation[chatID]; !ok {
		return false
	}
	delete(tg.awaitingLocation, chatID)
	return true
}

func (tg *TgHandlers) clearAwaitingLocation(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.awaitingLocation, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) isAwaitingLocation(chatID int64) bool {
	tg.pendingMu.RLock()
	defer tg.pendingMu.RUnlock()
	_, ok := tg.awaitingLocation[chatID]
	return ok
}

func (tg *TgHandlers) setOnboarding(chatID int64) {
	tg.pendingMu.Lock()
	tg.pendingOnboarding[chatID] = struct{}{}
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) clearOnboarding(chatID int64) {
	tg.pendingMu.Lock()
	delete(tg.pendingOnboarding, chatID)
	tg.pendingMu.Unlock()
}

func (tg *TgHandlers) isOnboarding(chatID int64) bool {
	tg.pendingMu.RLock()
	defer tg.pendingMu.RUnlock()
	_, ok := tg.pendingOnboarding[chatID]
	return ok
}
