package tghandlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/services"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (tg *TgHandlers) HandleLog(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	log := logger.From(ctx, tg.log)

	tg.clearAwaitingContext(update.Message.Chat.ID)
	tg.setAwaitingLog(update.Message.Chat.ID)

	if err := tg.sendLogPrompt(ctx, bot, update.Message.Chat.ID); err != nil {
		log.Error("failed to send log prompt", slog.Any("error", err))
	}
}

func (tg *TgHandlers) HandleStats(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	log := logger.From(ctx, tg.log)

	tg.clearAwaitingLog(update.Message.Chat.ID)
	tg.clearAwaitingContext(update.Message.Chat.ID)

	if err := tg.sendTodayStats(ctx, bot, update.Message.Chat.ID); err != nil {
		log.Error("failed to send stats", slog.Any("error", err))
	}
}

func (tg *TgHandlers) HandleHelp(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	log := logger.From(ctx, tg.log)

	tg.clearAwaitingLog(update.Message.Chat.ID)
	tg.clearAwaitingContext(update.Message.Chat.ID)

	if err := tg.sendHelp(ctx, bot, update.Message.Chat.ID); err != nil {
		log.Error("failed to send help", slog.Any("error", err))
	}
}

func (tg *TgHandlers) HandleMenuAction(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	log := logger.From(ctx, tg.log)

	_, err := bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
	if err != nil {
		log.Error("failed to answer callback query", slog.Any("error", err))
	}

	if update.CallbackQuery.Message.Message == nil {
		return
	}

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	data := update.CallbackQuery.Data

	switch {
	case data == callbackLogActivity:
		tg.clearAwaitingContext(chatID)
		tg.setAwaitingLog(chatID)
		err = tg.sendLogPrompt(ctx, bot, chatID)

	case data == callbackTodayStats:
		tg.clearAwaitingLog(chatID)
		tg.clearAwaitingContext(chatID)
		err = tg.sendTodayStats(ctx, bot, chatID)

	case data == callbackHelp:
		tg.clearAwaitingLog(chatID)
		tg.clearAwaitingContext(chatID)
		err = tg.sendHelp(ctx, bot, chatID)

	case data == callbackHome:
		tg.clearAwaitingLog(chatID)
		tg.clearAwaitingContext(chatID)
		err = tg.sendHomeMenu(ctx, bot, chatID)

	case data == callbackUpdateContext:
		tg.clearAwaitingLog(chatID)
		tg.setAwaitingContext(chatID)
		err = tg.sendContextPrompt(ctx, bot, chatID)

	case data == callbackCancelLog:
		if !tg.isAwaitingLog(chatID) {
			return
		}
		tg.clearAwaitingLog(chatID)
		_, err = bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Log activity canceled.",
		})

	case data == callbackSkipContext:
		if !tg.isAwaitingContext(chatID) {
			return
		}
		tg.clearAwaitingContext(chatID)
		err = tg.finishSkippedContextFlow(ctx, bot, chatID)

	case data == callbackReviewAccept:
		err = tg.handleReviewAccept(ctx, bot, chatID)

	case data == callbackReviewCancel:
		err = tg.handleReviewCancel(ctx, bot, chatID)

	case data == callbackReviewModify:
		err = tg.handleReviewModify(ctx, bot, chatID)

	case data == callbackReviewModifyTag:
		err = tg.handleReviewModifyTag(ctx, bot, chatID)

	case data == callbackReviewModifyType:
		err = tg.handleReviewModifyType(ctx, bot, chatID)

	case data == callbackReviewModifyStartedAt:
		err = tg.handleReviewModifyStartedAt(ctx, bot, chatID)

	case data == callbackReviewBack:
		r := tg.getPendingReview(chatID)
		if r == nil {
			return
		}
		r.mu.Lock()
		r.editIndex = -1
		snapshot := append([]domain.Activity(nil), r.activities...)
		r.mu.Unlock()
		err = tg.sendReviewMessage(ctx, bot, chatID, snapshot)

	case strings.HasPrefix(data, callbackReviewPickPrefix):
		err = tg.handleReviewPick(ctx, bot, chatID, data)

	case strings.HasPrefix(data, callbackReviewSetTypePrefix):
		err = tg.handleReviewSetType(ctx, bot, chatID, data)
	}

	if err != nil {
		log.Error("failed to handle menu action", slog.Any("error", err))
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
	log := logger.From(ctx, tg.log)

	text := strings.TrimSpace(update.Message.Text)
	if text == "" || strings.HasPrefix(text, "/") {
		tg.setAwaitingLog(chatID)
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "Please describe what you did today or cancel.",
			ReplyMarkup: cancelLogMarkup(),
		})
		if err != nil {
			log.Error("failed to ask for activity text", slog.Any("error", err))
		}
		return true
	}

	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(update.Message.From.ID, 10)}
	externalMessageID := strconv.Itoa(update.Message.ID)

	messageID, err := tg.messageService.SaveMessage(ctx, identity, externalMessageID, text)
	if err != nil {
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not save your activity right now. Please try again.",
		})
		if sendErr != nil {
			log.Error("failed to send save error", slog.Any("error", sendErr))
		}
		return true
	}

	activities, err := tg.aiProcessor.ExtractActivities(ctx, identity, messageID, text)
	if err != nil {
		var replyText string
		switch {
		case errors.Is(err, ai.ErrInputTooLong):
			tg.setAwaitingLog(chatID)
			replyText = "Your message is too long to process. Please shorten it and try again."
		case errors.Is(err, ai.ErrDailyBudgetExceeded):
			replyText = "You have reached your daily AI usage limit. Try again tomorrow."
		default:
			tg.setAwaitingLog(chatID)
			log.Error("ai extraction failed", slog.String("message_id", messageID), slog.Any("error", err))
			replyText = "Could not classify your activity right now. Please try again."
		}
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   replyText,
		})
		if sendErr != nil {
			log.Error("failed to send extraction error", slog.Any("error", sendErr))
		}
		return true
	}

	if activities == nil {
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Your message does not look like a day activity. Please describe something you actually did.",
		})
		if sendErr != nil {
			log.Error("failed to send invalid activity message", slog.Any("error", sendErr))
		}
		return true
	}

	r := &pendingReview{
		messageID:  messageID,
		identity:   identity,
		activities: activities,
		editIndex:  -1,
	}
	tg.setPendingReview(chatID, r)

	if err = tg.sendReviewMessage(ctx, bot, chatID, activities); err != nil {
		log.Error("failed to send review message", slog.Any("error", err))
	}

	return true
}

func (tg *TgHandlers) HandlePendingTagInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	if update.Message == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingTag(chatID) {
		return false
	}
	log := logger.From(ctx, tg.log)

	tag := strings.ToLower(strings.TrimSpace(update.Message.Text))
	if tag == "" || strings.HasPrefix(tag, "/") || strings.ContainsAny(tag, " \t\n") {
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Tag must be a single lowercase word. Try again.",
		})
		tg.setAwaitingTag(chatID)
		if err != nil {
			log.Error("failed to ask for tag again", slog.Any("error", err))
		}
		return true
	}

	r := tg.getPendingReview(chatID)
	if r == nil {
		return true
	}

	r.mu.Lock()
	if r.editIndex < 0 || r.editIndex >= len(r.activities) {
		r.mu.Unlock()
		return true
	}
	r.activities[r.editIndex].Tag = tag
	r.editIndex = -1
	snapshot := append([]domain.Activity(nil), r.activities...)
	r.mu.Unlock()

	if err := tg.sendReviewMessage(ctx, bot, chatID, snapshot); err != nil {
		log.Error("failed to send review message after tag edit", slog.Any("error", err))
	}

	return true
}

func (tg *TgHandlers) HandlePendingContextInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	if update.Message == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingContext(chatID) {
		return false
	}
	log := logger.From(ctx, tg.log)

	text := strings.TrimSpace(update.Message.Text)
	if text == "" || strings.HasPrefix(text, "/") {
		tg.setAwaitingContext(chatID)
		if err := tg.sendContextPrompt(ctx, bot, chatID); err != nil {
			log.Error("failed to ask for context text", slog.Any("error", err))
		}
		return true
	}

	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(update.Message.From.ID, 10)}
	err := tg.userService.UpdateUserContext(ctx, identity, text)
	if err != nil {
		replyText := "Could not save your context right now. Please try again."
		var ctxErr *services.ErrContextTooLong
		if errors.As(err, &ctxErr) {
			replyText = fmt.Sprintf("Your context is too long (%d characters). Please shorten it to %d characters or less.", ctxErr.Len, ctxErr.Limit)
		}
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   replyText,
		})
		if sendErr != nil {
			log.Error("failed to send context save error", slog.Any("error", sendErr))
		}
		return true
	}

	if err = tg.finishSavedContextFlow(ctx, bot, chatID); err != nil {
		log.Error("failed to finish context flow", slog.Any("error", err))
	}

	return true
}

func (tg *TgHandlers) HandlePendingStartedAtInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	if update.Message == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingStartedAt(chatID) {
		return false
	}
	log := logger.From(ctx, tg.log)

	text := strings.TrimSpace(update.Message.Text)
	if text == "" || strings.HasPrefix(text, "/") {
		tg.setAwaitingStartedAt(chatID)
		_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please enter a valid time or cancel.",
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{
					{Text: "Cancel", CallbackData: callbackReviewCancel},
				}},
			},
		})
		if err != nil {
			log.Error("failed to ask for start time again", slog.Any("error", err))
		}
		return true
	}

	t, err := parseStartedAt(text, time.Now().UTC())
	if err != nil {
		tg.setAwaitingStartedAt(chatID)
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Could not parse %q.\nExamples: today 14:00 · yesterday · 28 april 10:00 · 2026-04-28 14:00", text),
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{
					{Text: "Cancel", CallbackData: callbackReviewCancel},
				}},
			},
		})
		if sendErr != nil {
			log.Error("failed to send parse error", slog.Any("error", sendErr))
		}
		return true
	}

	r := tg.getPendingReview(chatID)
	if r == nil {
		return true
	}

	r.mu.Lock()
	if r.editIndex < 0 || r.editIndex >= len(r.activities) {
		r.mu.Unlock()
		return true
	}
	r.activities[r.editIndex].StartedAt = &t
	r.editIndex = -1
	snapshot := append([]domain.Activity(nil), r.activities...)
	r.mu.Unlock()

	if err := tg.sendReviewMessage(ctx, bot, chatID, snapshot); err != nil {
		log.Error("failed to send review message after started_at edit", slog.Any("error", err))
	}
	return true
}
