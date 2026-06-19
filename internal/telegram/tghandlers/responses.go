package tghandlers

import (
	"context"
	"errors"
	"fmt"
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
		Text: "Please describe what you did today — type it or send a voice message.\n\n" +
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

func fmtMinutes(m int) string {
	h := m / 60
	rem := m % 60
	if h == 0 {
		return fmt.Sprintf("%dmin", rem)
	}
	if rem == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dmin", h, rem)
}

func activityWord(n int) string {
	if n == 1 {
		return "1 activity"
	}
	return fmt.Sprintf("%d activities", n)
}

func (tg *TgHandlers) sendTodayStats(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	return tg.sendStatsPicker(ctx, bot, chatID)
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
		ChatID: chatID,
		Text: "GoDayLog tracks how you spend your time — just describe your day in plain text and the AI does the rest.\n\n" +
			"Commands:\n" +
			"/log — submit a description of your day or any activity\n" +
			"/stats — open the stats menu\n" +
			"/start — go back to the home screen\n\n" +
			"Logging:\n" +
			"Write naturally or send a voice message: \"worked on the project for 2 hours, then went for a run, watched Netflix\". The AI splits this into individual activities, assigns a type and duration, and asks you to confirm before saving anything.\n\n" +
			"Activity types:\n" +
			"• growth — working toward your goals\n" +
			"• routine — necessary daily tasks\n" +
			"• rest — deliberate rest and leisure\n" +
			"• drain — passive consumption\n\n" +
			"You can hint the type directly: \"played guitar — growth\".\n\n" +
			"Stats:\n" +
			"View breakdowns by day, week, or any custom range. Each range shows total activities, time tracked, type split, and top tags. Tap Analyse to get AI insights on your patterns.\n\n" +
			"Settings:\n" +
			"Set your timezone so dates and stats are shown in your local time. Add a personal context so the AI understands what matters to you and classifies activities more accurately.",
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
		"\n\nA few lines are enough. This helps the AI better understand which activities matter for you most." +
		"\n\nYou can type it or send a voice message."

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

	if tg.isOnboarding(chatID) {
		tg.clearOnboarding(chatID)
		tg.setAwaitingLocation(chatID)
		return tg.sendLocationPrompt(ctx, bot, chatID)
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

	if tg.isOnboarding(chatID) {
		tg.clearOnboarding(chatID)
		tg.setAwaitingLocation(chatID)
		return tg.sendLocationPrompt(ctx, bot, chatID)
	}
	return tg.sendHomeMenu(ctx, bot, chatID)
}

func (tg *TgHandlers) sendLocationPrompt(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Share your location so I can set your timezone automatically.",
		ReplyMarkup: &models.ReplyKeyboardMarkup{
			Keyboard: [][]models.KeyboardButton{
				{{Text: "Share my location", RequestLocation: true}},
			},
			ResizeKeyboard:  true,
			OneTimeKeyboard: true,
		},
	})
	if err != nil {
		return err
	}
	_, err = bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Or skip:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: "Skip", CallbackData: callbackSkipLocation}},
			},
		},
	})
	return err
}

func (tg *TgHandlers) finishSkippedLocationFlow(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "No problem. You can set your timezone from Settings anytime.",
		ReplyMarkup: &models.ReplyKeyboardRemove{RemoveKeyboard: true},
	})
	if err != nil {
		return err
	}
	return tg.sendHomeMenu(ctx, bot, chatID)
}
