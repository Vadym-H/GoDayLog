package tghandlers

import (
	"log/slog"
	"sync"

	"github.com/Vadym-H/GoDayLog/internal/services"
)

type TgHandlers struct {
	log           *slog.Logger
	userService   *services.UserService
	pendingMu     sync.RWMutex
	pendingLog    map[int64]struct{}
	pendingLlmCtx map[int64]struct{}
}

func New(log *slog.Logger, userService *services.UserService) *TgHandlers {
	return &TgHandlers{
		log:           log,
		userService:   userService,
		pendingLog:    make(map[int64]struct{}),
		pendingLlmCtx: make(map[int64]struct{}),
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
