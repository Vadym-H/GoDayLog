package tghandlers

import (
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/services"
)

type TgHandlers struct {
	log         *slog.Logger
	userService *services.UserService
}

func New(log *slog.Logger, userService *services.UserService) *TgHandlers {
	return &TgHandlers{log: log, userService: userService}
}
