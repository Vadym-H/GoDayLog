package tghandlers

import (
	"context"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (tg *TgHandlers) HandleLog(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	tg.setAwaitingLog(update.Message.Chat.ID)

	if err := tg.sendLogPrompt(ctx, bot, update.Message.Chat.ID); err != nil {
		tg.log.Error("failed to send log prompt", slog.String("error", err.Error()))
	}
}

func (tg *TgHandlers) HandleStats(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	tg.clearAwaitingLog(update.Message.Chat.ID)

	if err := tg.sendTodayStats(ctx, bot, update.Message.Chat.ID); err != nil {
		tg.log.Error("failed to send stats", slog.String("error", err.Error()))
	}
}

func (tg *TgHandlers) HandleHelp(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	tg.clearAwaitingLog(update.Message.Chat.ID)

	if err := tg.sendHelp(ctx, bot, update.Message.Chat.ID); err != nil {
		tg.log.Error("failed to send help", slog.String("error", err.Error()))
	}
}

func (tg *TgHandlers) HandleMenuAction(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	_, err := bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
	if err != nil {
		tg.log.Error("failed to answer callback query", slog.String("error", err.Error()))
	}

	if update.CallbackQuery.Message.Message == nil {
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID

	switch update.CallbackQuery.Data {
	case callbackLogActivity:
		tg.setAwaitingLog(chatID)
		err = tg.sendLogPrompt(ctx, bot, chatID)
	case callbackTodayStats:
		tg.clearAwaitingLog(chatID)
		err = tg.sendTodayStats(ctx, bot, chatID)
	case callbackHelp:
		tg.clearAwaitingLog(chatID)
		err = tg.sendHelp(ctx, bot, chatID)
	case callbackHome:
		tg.clearAwaitingLog(chatID)
		err = tg.sendHomeMenu(ctx, bot, chatID)
	case callbackCancelLog:
		tg.clearAwaitingLog(chatID)
		_, err = bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Log activity canceled.",
		})
	default:
		err = nil
	}

	if err != nil {
		tg.log.Error("failed to handle menu action", slog.String("error", err.Error()))
	}
}

func (tg *TgHandlers) HandlePendingLogInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	if update.Message == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingLog(chatID) {
		return false
	}

	text := strings.TrimSpace(update.Message.Text)
	if text == "" || strings.HasPrefix(text, "/") {
		tg.setAwaitingLog(chatID)
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Please describe what you did today or cancel the log activity.",
			ReplyMarkup: cancelLogMarkup(),
		})
		if err != nil {
			tg.log.Error("failed to ask for activity text", slog.String("error", err.Error()))
		}
		return true
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Your activity was saved.",
	})
	if err != nil {
		tg.log.Error("failed to send activity saved confirmation", slog.String("error", err.Error()))
	}

	return true
}

func cancelLogMarkup() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Cancel", CallbackData: callbackCancelLog},
			},
		},
	}
}

func (tg *TgHandlers) sendLogPrompt(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Please describe what you did today or cancel the log activity.",
		ReplyMarkup: cancelLogMarkup(),
	})
	return err
}

func (tg *TgHandlers) sendTodayStats(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Log activity", CallbackData: callbackLogActivity},
				{Text: "Refresh", CallbackData: callbackTodayStats},
			},
			{
				{Text: "Home", CallbackData: callbackHome},
			},
		},
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Today: 0 activities logged.\nUseful: 0%.\nTop tags: n/a.",
		ReplyMarkup: markup,
	})
	return err
}

func (tg *TgHandlers) sendHelp(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Log activity", CallbackData: callbackLogActivity},
				{Text: "Home", CallbackData: callbackHome},
			},
		},
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Use /start to open the home screen.\nUse /log to submit activity text.\nUse /stats for today's overview.",
		ReplyMarkup: markup,
	})
	return err
}
