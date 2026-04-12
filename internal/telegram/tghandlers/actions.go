package tghandlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Vadym-H/GoDayLog/internal/storage"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (tg *TgHandlers) HandleLog(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	const op = "telegram.tghandlers.HandleLog"

	if update.Message == nil {
		return
	}

	tg.clearAwaitingContext(update.Message.Chat.ID)
	tg.setAwaitingLog(update.Message.Chat.ID)

	if err := tg.sendLogPrompt(ctx, bot, update.Message.Chat.ID); err != nil {
		tg.log.Error("failed to send log prompt",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}
}

func (tg *TgHandlers) HandleStats(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	const op = "telegram.tghandlers.HandleStats"

	if update.Message == nil {
		return
	}

	tg.clearAwaitingLog(update.Message.Chat.ID)
	tg.clearAwaitingContext(update.Message.Chat.ID)

	if err := tg.sendTodayStats(ctx, bot, update.Message.Chat.ID); err != nil {
		tg.log.Error("failed to send stats",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}
}

func (tg *TgHandlers) HandleHelp(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	const op = "telegram.tghandlers.HandleHelp"

	if update.Message == nil {
		return
	}

	tg.clearAwaitingLog(update.Message.Chat.ID)
	tg.clearAwaitingContext(update.Message.Chat.ID)

	if err := tg.sendHelp(ctx, bot, update.Message.Chat.ID); err != nil {
		tg.log.Error("failed to send help",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}
}

func (tg *TgHandlers) HandleMenuAction(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	const op = "telegram.tghandlers.HandleMenuAction"

	if update.CallbackQuery == nil {
		return
	}

	_, err := bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
	if err != nil {
		tg.log.Error("failed to answer callback query",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}

	if update.CallbackQuery.Message.Message == nil {
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID

	switch update.CallbackQuery.Data {
	case callbackLogActivity:
		tg.clearAwaitingContext(chatID)
		tg.setAwaitingLog(chatID)
		err = tg.sendLogPrompt(ctx, bot, chatID)
	case callbackTodayStats:
		tg.clearAwaitingLog(chatID)
		tg.clearAwaitingContext(chatID)
		err = tg.sendTodayStats(ctx, bot, chatID)
	case callbackHelp:
		tg.clearAwaitingLog(chatID)
		tg.clearAwaitingContext(chatID)
		err = tg.sendHelp(ctx, bot, chatID)
	case callbackHome:
		tg.clearAwaitingLog(chatID)
		tg.clearAwaitingContext(chatID)
		err = tg.sendHomeMenu(ctx, bot, chatID)
	case callbackUpdateContext:
		tg.clearAwaitingLog(chatID)
		tg.setAwaitingContext(chatID)
		err = tg.sendContextPrompt(ctx, bot, chatID)
	case callbackCancelLog:
		if !tg.isAwaitingLog(chatID) {
			return
		}
		tg.clearAwaitingLog(chatID)
		_, err = bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Log activity canceled.",
		})
	case callbackSkipContext:
		if !tg.isAwaitingContext(chatID) {
			return
		}
		tg.clearAwaitingContext(chatID)
		err = tg.finishSkippedContextFlow(ctx, bot, chatID)
	default:
		err = nil
	}

	if err != nil {
		tg.log.Error("failed to handle menu action",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}
}

func (tg *TgHandlers) HandlePendingLogInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	const op = "telegram.tghandlers.HandlePendingLogInput"

	if update.Message == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingLog(chatID) {
		return false
	}

	text := strings.TrimSpace(update.Message.Text)
	// Keep waiting state when user sends an empty value or another command.
	if text == "" || strings.HasPrefix(text, "/") {
		tg.setAwaitingLog(chatID)
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Please describe what you did today or cancel the log activity.",
			ReplyMarkup: cancelLogMarkup(),
		})
		if err != nil {
			tg.log.Error("failed to ask for activity text",
				slog.String("op", op),
				slog.String("error", err.Error()),
			)
		}
		return true
	}

	providerExternalID := strconv.FormatInt(update.Message.From.ID, 10)
	externalMessageID := strconv.Itoa(update.Message.ID)

	_, err := tg.messageService.SaveMessage(ctx, "telegram", providerExternalID, externalMessageID, text)
	if err != nil {
		tg.log.Error("failed to save activity message",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not save your activity right now. Please try again.",
		})
		if sendErr != nil {
			tg.log.Error("failed to send activity save error",
				slog.String("op", op),
				slog.String("error", sendErr.Error()),
			)
		}
		return true
	}

	_, err = bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Your activity was saved.",
	})
	if err != nil {
		tg.log.Error("failed to send activity saved confirmation",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
	}

	return true
}

func (tg *TgHandlers) HandlePendingContextInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	const op = "telegram.tghandlers.HandlePendingContextInput"

	if update.Message == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingContext(chatID) {
		return false
	}

	text := strings.TrimSpace(update.Message.Text)
	// Keep awaiting state until user provides non-empty free text or taps Skip.
	if text == "" || strings.HasPrefix(text, "/") {
		tg.setAwaitingContext(chatID)
		err := tg.sendContextPrompt(ctx, bot, chatID)
		if err != nil {
			tg.log.Error("failed to ask for context text",
				slog.String("op", op),
				slog.String("error", err.Error()),
			)
		}
		return true
	}

	providerExternalID := strconv.FormatInt(update.Message.From.ID, 10)
	err := tg.userService.UpdateUserContext(ctx, "telegram", providerExternalID, text)
	if err != nil {
		tg.log.Error("failed to save user context",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not save your context right now. Please try again.",
		})
		if sendErr != nil {
			tg.log.Error("failed to send context save error",
				slog.String("op", op),
				slog.String("error", sendErr.Error()),
			)
		}
		return true
	}

	err = tg.finishSavedContextFlow(ctx, bot, chatID)
	if err != nil {
		tg.log.Error("failed to finish context flow",
			slog.String("op", op),
			slog.String("error", err.Error()),
		)
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
		Text:        "Use /start to open the home screen.\nUse /log to submit activity text.\nUse /stats for today's overview.\nUse the Home menu to update your AI context.",
		ReplyMarkup: markup,
	})
	return err
}

func (tg *TgHandlers) sendContextPrompt(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	const op = "telegram.tghandlers.sendContextPrompt"
	currentContext, err := tg.userService.GetUserContext(ctx, "telegram", strconv.FormatInt(chatID, 10))
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			tg.log.Info("user not found when sending context prompt")
		} else {
			tg.log.Debug("failed to get user context",
				slog.String("op", op),
				slog.String("error", err.Error()),
			)
		}
	}

	text := "To personalize activity insights, please share a short context about you: your routine, priorities, and what feels useful (for example: work focus, fitness, study, family, or wellbeing)." +
		"\n\nA few lines are enough. This helps the AI better understand which activities matter for you most."

	if currentContext != "" {
		text += "\n\nYour current context: " + currentContext
	}

	_, err = bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "Skip", CallbackData: callbackSkipContext},
		}}},
	})
	return err
}

func (tg *TgHandlers) finishSkippedContextFlow(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Got it. You can share your context later anytime.",
	})
	if err != nil {
		return err
	}

	return tg.sendHomeMenu(ctx, bot, chatID)
}

func (tg *TgHandlers) finishSavedContextFlow(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Thanks! Your context is saved and will be used to personalize insights.",
	})
	if err != nil {
		return err
	}

	return tg.sendHomeMenu(ctx, bot, chatID)
}
