package tghandlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/storage"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	callbackReviewAccept          = callbackActionPrefix + "review:accept"
	callbackReviewCancel          = callbackActionPrefix + "review:cancel"
	callbackReviewModify          = callbackActionPrefix + "review:modify"
	callbackReviewModifyTag       = callbackActionPrefix + "review:modify_tag"
	callbackReviewModifyType      = callbackActionPrefix + "review:modify_type"
	callbackReviewModifyStartedAt = callbackActionPrefix + "review:modify_started_at"
	callbackReviewBack            = callbackActionPrefix + "review:back"
	callbackReviewPickPrefix      = callbackActionPrefix + "review:pick:"
	callbackReviewSetTypePrefix   = callbackActionPrefix + "review:set_type:"
)

func (tg *TgHandlers) handleReviewAccept(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	r.mu.Lock()
	messageID := r.messageID
	identity := r.identity
	activities := append([]domain.Activity(nil), r.activities...)
	r.mu.Unlock()

	if err := tg.aiProcessor.SaveActivities(ctx, identity, messageID, activities); err != nil {
		tg.clearAwaitingTag(chatID)
		tg.clearAwaitingStartedAt(chatID)
		tg.clearPendingReview(chatID)

		if errors.Is(err, storage.ErrMessageFailed) {
			_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Your session expired (pending too long). Please log your activity again.",
			})
			if sendErr != nil {
				return sendErr
			}
			return tg.sendHomeMenu(ctx, bot, chatID)
		}

		logger.From(ctx, tg.log).Error("failed to save activities", slog.Any("error", err))
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not save activities right now. Please try again.",
		})
		return sendErr
	}

	tg.clearAwaitingTag(chatID)
	tg.clearAwaitingStartedAt(chatID)
	tg.clearPendingReview(chatID)

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Activities saved.",
	})
	if err != nil {
		return err
	}

	return tg.sendHomeMenu(ctx, bot, chatID)
}

func (tg *TgHandlers) handleReviewCancel(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	r.mu.Lock()
	messageID := r.messageID
	identity := r.identity
	r.mu.Unlock()

	_ = tg.aiProcessor.CancelReview(ctx, identity, messageID)
	tg.clearAwaitingTag(chatID)
	tg.clearAwaitingStartedAt(chatID)
	tg.clearPendingReview(chatID)

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Activity log cancelled.",
	})
	if err != nil {
		return err
	}

	return tg.sendHomeMenu(ctx, bot, chatID)
}

func (tg *TgHandlers) handleReviewModify(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	r.mu.Lock()
	count := len(r.activities)
	if count == 1 {
		r.editIndex = 0
		activity := r.activities[0]
		r.mu.Unlock()
		return tg.sendModifyOptions(ctx, bot, chatID, activity, 0)
	}
	r.mu.Unlock()

	rows := make([][]models.InlineKeyboardButton, 0, count+1)
	for i := 0; i < count; i++ {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("Activity %d", i+1), CallbackData: callbackReviewPickPrefix + strconv.Itoa(i)},
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "Cancel", CallbackData: callbackReviewCancel},
	})

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Which activity do you want to modify?",
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: rows},
	})
	return err
}

func (tg *TgHandlers) handleReviewPick(ctx context.Context, bot *tgbot.Bot, chatID int64, data string) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	indexStr := strings.TrimPrefix(data, callbackReviewPickPrefix)
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return nil
	}

	r.mu.Lock()
	if index < 0 || index >= len(r.activities) {
		r.mu.Unlock()
		return nil
	}
	r.editIndex = index
	activity := r.activities[index]
	r.mu.Unlock()

	return tg.sendModifyOptions(ctx, bot, chatID, activity, index)
}

func (tg *TgHandlers) sendModifyOptions(ctx context.Context, bot *tgbot.Bot, chatID int64, a domain.Activity, index int) error {
	startedAt := "not set"
	if a.StartedAt != nil {
		startedAt = a.StartedAt.UTC().Format("2 Jan 2006 15:04 UTC")
	}
	text := fmt.Sprintf("Modify activity %d: %q\nTag: %s · Type: %s\nStarted: %s",
		index+1, a.Description, a.Tag, a.ActivityType, startedAt)

	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Modify tag", CallbackData: callbackReviewModifyTag},
				{Text: "Modify type", CallbackData: callbackReviewModifyType},
			},
			{
				{Text: "Modify started at", CallbackData: callbackReviewModifyStartedAt},
			},
			{
				{Text: "Back", CallbackData: callbackReviewBack},
				{Text: "Cancel", CallbackData: callbackReviewCancel},
			},
		},
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: markup,
	})
	return err
}

func (tg *TgHandlers) handleReviewModifyTag(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	r.mu.Lock()
	editIdx := r.editIndex
	r.mu.Unlock()
	if editIdx < 0 {
		return nil
	}

	tg.setAwaitingTag(chatID)

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Send the new tag for this activity (one word, lowercase).",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: "Cancel", CallbackData: callbackReviewCancel},
			}},
		},
	})
	return err
}

func (tg *TgHandlers) handleReviewModifyType(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	r.mu.Lock()
	editIdx := r.editIndex
	r.mu.Unlock()
	if editIdx < 0 {
		return nil
	}

	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "growth", CallbackData: callbackReviewSetTypePrefix + "growth"},
				{Text: "routine", CallbackData: callbackReviewSetTypePrefix + "routine"},
			},
			{
				{Text: "rest", CallbackData: callbackReviewSetTypePrefix + "rest"},
				{Text: "drain", CallbackData: callbackReviewSetTypePrefix + "drain"},
			},
			{
				{Text: "Back", CallbackData: callbackReviewBack},
				{Text: "Cancel", CallbackData: callbackReviewCancel},
			},
		},
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Select the new activity type:",
		ReplyMarkup: markup,
	})
	return err
}

func (tg *TgHandlers) handleReviewSetType(ctx context.Context, bot *tgbot.Bot, chatID int64, data string) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	value := strings.TrimPrefix(data, callbackReviewSetTypePrefix)
	switch value {
	case "growth", "routine", "rest", "drain":
	default:
		return nil
	}

	r.mu.Lock()
	if r.editIndex < 0 || r.editIndex >= len(r.activities) {
		r.mu.Unlock()
		return nil
	}
	r.activities[r.editIndex].ActivityType = value
	r.editIndex = -1
	snapshot := append([]domain.Activity(nil), r.activities...)
	r.mu.Unlock()

	return tg.sendReviewMessage(ctx, bot, chatID, snapshot)
}

func (tg *TgHandlers) sendReviewMessage(ctx context.Context, bot *tgbot.Bot, chatID int64, activities []domain.Activity) error {
	var sb strings.Builder
	sb.WriteString("Here is what AI extracted. Review and accept or adjust:\n")

	for i, a := range activities {
		sb.WriteString(fmt.Sprintf("\n%d. %s\n   Tag: %s · Type: %s", i+1, a.Description, a.Tag, a.ActivityType))
		if a.DurationMinutes != nil {
			sb.WriteString(" · " + fmtMinutes(*a.DurationMinutes))
		}
		if a.StartedAt != nil {
			sb.WriteString(fmt.Sprintf("\n   Started: %s", a.StartedAt.UTC().Format("2 Jan 2006 15:04 UTC")))
		} else {
			sb.WriteString("\n   Started: not set")
		}
		sb.WriteString("\n")
	}

	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Accept", CallbackData: callbackReviewAccept},
				{Text: "Modify", CallbackData: callbackReviewModify},
				{Text: "Cancel", CallbackData: callbackReviewCancel},
			},
		},
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        sb.String(),
		ReplyMarkup: markup,
	})
	return err
}

func (tg *TgHandlers) handleReviewModifyStartedAt(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	r.mu.Lock()
	editIdx := r.editIndex
	r.mu.Unlock()
	if editIdx < 0 {
		return nil
	}

	tg.setAwaitingStartedAt(chatID)

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Send the new start time.\nExamples: today 14:00 · yesterday · yesterday 09:30 · 3 days ago · 28 april 10:00 · 2026-04-28 14:00",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: "Cancel", CallbackData: callbackReviewCancel},
			}},
		},
	})
	return err
}
