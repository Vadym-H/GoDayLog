package tghandlers

import (
	"context"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// HandleMenu sends a simple inline keyboard menu for testing.
// Command: /menu
func (tg *TgHandlers) HandleMenu(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	keyboard := [][]models.InlineKeyboardButton{
		{
			{Text: "Option 1", CallbackData: "opt1"},
			{Text: "Option 2", CallbackData: "opt2"},
		},
		{
			{Text: "Option 3", CallbackData: "opt3"},
		},
	}

	markup := &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Choose an option:",
		ReplyMarkup: markup,
	})
	if err != nil {
		tg.log.Error("failed to send menu", slog.String("error", err.Error()))
	}
}
