package tghandlers

import (
	"context"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	callbackActionPrefix  = "action:"
	callbackLogActivity   = callbackActionPrefix + "log"
	callbackTodayStats    = callbackActionPrefix + "stats"
	callbackHelp          = callbackActionPrefix + "help"
	callbackHome          = callbackActionPrefix + "home"
	callbackCancelLog     = callbackActionPrefix + "cancel_log"
	callbackSkipContext   = callbackActionPrefix + "skip_context"
	callbackUpdateContext = callbackActionPrefix + "update_context"
)

// HandleMenu sends the home screen with inline actions.
func (tg *TgHandlers) HandleMenu(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	const op = "telegram.tghandlers.HandleMenu"

	if update.Message == nil {
		return
	}

	err := tg.sendHomeMenu(ctx, bot, update.Message.Chat.ID)
	if err != nil {
		tg.log.Error("failed to send menu",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}
}

func mainMenuMarkup() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Log activity", CallbackData: callbackLogActivity},
				{Text: "Today stats", CallbackData: callbackTodayStats},
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
