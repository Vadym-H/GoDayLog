package tghandlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/Vadym-H/GoDayLog/internal/storage"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (tg *TgHandlers) HandleStart(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	user := update.Message.From
	chatID := update.Message.Chat.ID
	tg.clearAwaitingLog(chatID)

	err := tg.userService.RegisterUser(ctx, "telegram", strconv.FormatInt(user.ID, 10))
	if err != nil {
		if !errors.Is(err, storage.UserExists) {
			tg.log.Error("failed to create user", slog.String("error", err.Error()))
			_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "something went wrong, please try again",
			})
			if err != nil {
				tg.log.Error("failed to send error message", slog.String("error", err.Error()))
			}
			return
		}
	}

	tg.HandleMenu(ctx, bot, update)
}
