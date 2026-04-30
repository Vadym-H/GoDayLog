package tghandlers

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/logger"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	callbackActionPrefix  = "action:"
	callbackLogActivity   = callbackActionPrefix + "log"
	callbackStatsPicker   = callbackActionPrefix + "stats"
	callbackHelp          = callbackActionPrefix + "help"
	callbackHome          = callbackActionPrefix + "home"
	callbackCancelLog     = callbackActionPrefix + "cancel_log"
	callbackSkipContext   = callbackActionPrefix + "skip_context"
	callbackUpdateContext = callbackActionPrefix + "update_context"
)

// HandleMenu sends the home screen with inline actions.
func (tg *TgHandlers) HandleMenu(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	if err := tg.sendHomeMenu(ctx, bot, update.Message.Chat.ID); err != nil {
		logger.From(ctx, tg.log).Error("failed to send menu", slog.Any("error", err))
	}
}

func mainMenuMarkup() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Log activity", CallbackData: callbackLogActivity},
				{Text: "Statistics", CallbackData: callbackStatsPicker},
			},
			{
				{Text: "Help", CallbackData: callbackHelp},
				{Text: "Update context", CallbackData: callbackUpdateContext},
			},
		},
	}
}

func (tg *TgHandlers) sendHomeMenu(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "What would you like to do?",
		ReplyMarkup: mainMenuMarkup(),
	})
	return err
}
