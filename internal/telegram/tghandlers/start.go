package tghandlers

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (tg *TgHandlers) HandleStart(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	log := logger.From(ctx, tg.log)

	user := update.Message.From
	chatID := update.Message.Chat.ID
	tg.clearAwaitingLog(chatID)
	tg.clearAwaitingContext(chatID)
	tg.clearAwaitingLocation(chatID)
	tg.clearOnboarding(chatID)

	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(user.ID, 10)}
	_, isNewUser, err := tg.userService.RegisterUser(ctx, identity)
	if err != nil {
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "something went wrong, please try again",
		})
		if sendErr != nil {
			log.Error("failed to send error message", slog.Any("error", sendErr))
		}
		return
	}

	if isNewUser {
		tg.setOnboarding(chatID)
		tg.setAwaitingContext(chatID)
		if err = tg.sendContextPrompt(ctx, bot, chatID); err != nil {
			log.Error("failed to send context prompt", slog.Any("error", err))
		}
		return
	}

	tg.HandleMenu(ctx, bot, update)
}
