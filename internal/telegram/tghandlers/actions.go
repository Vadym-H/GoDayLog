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
	callbackReviewAccept        = callbackActionPrefix + "review:accept"
	callbackReviewCancel        = callbackActionPrefix + "review:cancel"
	callbackReviewModify        = callbackActionPrefix + "review:modify"
	callbackReviewModifyTag     = callbackActionPrefix + "review:modify_tag"
	callbackReviewModifyType    = callbackActionPrefix + "review:modify_type"
	callbackReviewBack          = callbackActionPrefix + "review:back"
	callbackReviewPickPrefix    = callbackActionPrefix + "review:pick:"
	callbackReviewSetTypePrefix = callbackActionPrefix + "review:set_type:"
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

	default:
		err = nil
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
		log.Error("ai extraction failed", slog.String("message_id", messageID), slog.Any("error", err))
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not classify your activity right now. Please try again.",
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
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not save your context right now. Please try again.",
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

// --- review handlers ---

func (tg *TgHandlers) handleReviewAccept(ctx context.Context, bot *tgbot.Bot, chatID int64) error {
	r := tg.getPendingReview(chatID)
	if r == nil {
		return nil
	}

	r.mu.Lock()
	messageID := r.messageID
	activities := append([]domain.Activity(nil), r.activities...)
	r.mu.Unlock()

	if err := tg.aiProcessor.SaveActivities(ctx, messageID, activities); err != nil {
		logger.From(ctx, tg.log).Error("failed to save activities", slog.Any("error", err))
		_, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not save activities right now. Please try again.",
		})
		return sendErr
	}

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

	_ = tg.aiProcessor.CancelReview(ctx, r.messageID)
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
	text := fmt.Sprintf("Modify activity %d: %q\nTag: %s · Type: %s",
		index+1, a.Description, a.Tag, a.ActivityType)

	markup := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Modify tag", CallbackData: callbackReviewModifyTag},
				{Text: "Modify type", CallbackData: callbackReviewModifyType},
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

// --- review message formatting ---

func (tg *TgHandlers) sendReviewMessage(ctx context.Context, bot *tgbot.Bot, chatID int64, activities []domain.Activity) error {
	var sb strings.Builder
	sb.WriteString("Here is what AI extracted. Review and accept or adjust:\n")

	for i, a := range activities {
		sb.WriteString(fmt.Sprintf("\n%d. %s\n   Tag: %s · Type: %s", i+1, a.Description, a.Tag, a.ActivityType))
		if a.DurationMinutes != nil {
			sb.WriteString(" · " + formatDuration(*a.DurationMinutes))
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

func formatDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dmin", h, m)
}

// --- other handlers ---

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
