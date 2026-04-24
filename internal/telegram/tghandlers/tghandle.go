package tghandlers

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Vadym-H/GoDayLog/internal/domain"
)

type userService interface {
	RegisterUser(ctx context.Context, id domain.Identity) (string, bool, error)
	GetUserContext(ctx context.Context, id domain.Identity) (string, error)
	UpdateUserContext(ctx context.Context, id domain.Identity, llmContext string) error
}

type messageService interface {
	SaveMessage(ctx context.Context, id domain.Identity, externalMessageID, text string) (string, error)
}

type messageAIProcessor interface {
	ProcessSavedMessage(ctx context.Context, id domain.Identity, messageID, text string) error
}

type TgHandlers struct {
	log            *slog.Logger
	userService    userService
	messageService messageService
	aiProcessor    messageAIProcessor
	pendingMu      sync.RWMutex
	pendingLog     map[int64]struct{}
	pendingLlmCtx  map[int64]struct{}
}

func New(log *slog.Logger, userService userService, messageService messageService, aiProcessor messageAIProcessor) *TgHandlers {
	return &TgHandlers{
		log:            log,
		userService:    userService,
		messageService: messageService,
		aiProcessor:    aiProcessor,
		pendingLog:     make(map[int64]struct{}),
		pendingLlmCtx:  make(map[int64]struct{}),
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
