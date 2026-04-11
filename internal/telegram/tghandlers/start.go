package tghandlers

import (
	"context"
	"log/slog"
	"strconv"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (tg *TgHandlers) HandleStart(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	const op = "telegram.tghandlers.HandleStart"

	if update.Message == nil {
		return
	}

	user := update.Message.From
	chatID := update.Message.Chat.ID
	tg.clearAwaitingLog(chatID)
	tg.clearAwaitingContext(chatID)

	_, isNewUser, err := tg.userService.RegisterUser(ctx, "telegram", strconv.FormatInt(user.ID, 10))
	if err != nil {
		tg.log.Error("failed to register user",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "something went wrong, please try again",
		})
		if err != nil {
			tg.log.Error("failed to send error message",
				slog.String("op", op),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	if isNewUser {
		// New users provide optional context once so later activity judgments are personalized.
		tg.setAwaitingContext(chatID)
		if err = tg.sendContextPrompt(ctx, bot, chatID); err != nil {
			tg.log.Error("failed to send context prompt",
				slog.String("op", op),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	tg.HandleMenu(ctx, bot, update)
}
