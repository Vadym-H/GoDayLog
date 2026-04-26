package tghandlers

import (
	"context"
	"errors"
	"strconv"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func cancelLogMarkup() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "Cancel", CallbackData: callbackCancelLog}},
		},
	}
}

func (tg *TgHandlers) sendLogPrompt(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "Please describe what you did today.\n\n" +
			"You can include an activity type to help the AI:\n" +
			"• growth — learning, working toward goals\n" +
			"• routine — chores, errands, cooking\n" +
			"• rest — relaxation, low-effort leisure\n" +
			"• drain — social media, games, passive entertainment\n\n" +
			"Example: \"watched YouTube for 2h — drain\"\n\n" +
			"AI may make mistakes. You will review the result before anything is saved.",
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
		Text:        "Today: 0 activities logged.\nTop types: n/a.",
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
	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(chatID, 10)}
	currentContext, err := tg.userService.GetUserContext(ctx, identity)
	if err != nil && !errors.Is(err, storage.ErrUserNotFound) {
		// non-fatal: proceed without showing current context
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
