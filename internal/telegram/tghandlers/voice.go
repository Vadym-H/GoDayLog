package tghandlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
	"github.com/Vadym-H/GoDayLog/internal/services"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (tg *TgHandlers) HandlePendingVoiceLogInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	if update.Message == nil || update.Message.Voice == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingLog(chatID) {
		return false
	}
	log := logger.From(ctx, tg.log)

	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(update.Message.From.ID, 10)}
	text, ok := tg.downloadAndTranscribe(ctx, bot, log, chatID, identity, update.Message.Voice, func() { tg.setAwaitingLog(chatID) })
	if !ok {
		return true
	}

	externalMessageID := strconv.Itoa(update.Message.ID)
	tg.processLogText(ctx, bot, chatID, identity, externalMessageID, text)
	return true
}

func (tg *TgHandlers) HandlePendingVoiceContextInput(ctx context.Context, bot *tgbot.Bot, update *models.Update) bool {
	if update.Message == nil || update.Message.Voice == nil {
		return false
	}

	chatID := update.Message.Chat.ID
	if !tg.consumeAwaitingContext(chatID) {
		return false
	}
	log := logger.From(ctx, tg.log)

	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(update.Message.From.ID, 10)}
	text, ok := tg.downloadAndTranscribe(ctx, bot, log, chatID, identity, update.Message.Voice, func() { tg.setAwaitingContext(chatID) })
	if !ok {
		return true
	}

	if err := tg.userService.UpdateUserContext(ctx, identity, text); err != nil {
		var ctxErr *services.ErrContextTooLong
		var replyText string
		if errors.As(err, &ctxErr) {
			tg.setAwaitingContext(chatID)
			replyText = fmt.Sprintf("Your context is too long (%d characters). Please shorten it to %d characters or less.", ctxErr.Len, ctxErr.Limit)
		} else {
			replyText = "Could not save your context right now. Please try again."
		}
		if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   replyText,
		}); sendErr != nil {
			log.Error("voice context: send save error reply", slog.Any("error", sendErr))
		}
		return true
	}

	if err := tg.finishSavedContextFlow(ctx, bot, chatID); err != nil {
		log.Error("voice context: finish flow failed", slog.Any("error", err))
	}
	return true
}

// downloadAndTranscribe downloads a Telegram voice file, transcribes it, and returns the
// text. On any failure it sends the appropriate reply, calls rearm to restore awaiting
// state where needed, and returns ok=false. rearm is called only when the user should
// be able to retry (transient errors, ErrAudioTooLarge); it is not called for budget
// exhaustion since the daily limit won't change until tomorrow.
func (tg *TgHandlers) downloadAndTranscribe(
	ctx context.Context,
	bot *tgbot.Bot,
	log *slog.Logger,
	chatID int64,
	identity domain.Identity,
	voice *models.Voice,
	rearm func(),
) (text string, ok bool) {
	file, err := bot.GetFile(ctx, &tgbot.GetFileParams{FileID: voice.FileID})
	if err != nil {
		log.Error("voice: get file failed", slog.Any("error", err))
		rearm()
		tg.sendVoiceError(ctx, bot, log, chatID, "Could not transcribe your voice message. Please try again or type it.")
		return "", false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bot.FileDownloadLink(file), nil)
	if err != nil {
		log.Error("voice: build download request", slog.Any("error", err))
		rearm()
		tg.sendVoiceError(ctx, bot, log, chatID, "Could not transcribe your voice message. Please try again or type it.")
		return "", false
	}

	// Fix 4: bounded client — no timeout on DefaultClient
	resp, err := tg.downloadClient.Do(req)
	if err != nil {
		log.Error("voice: download failed", slog.Any("error", err))
		rearm()
		tg.sendVoiceError(ctx, bot, log, chatID, "Could not transcribe your voice message. Please try again or type it.")
		return "", false
	}
	defer resp.Body.Close()

	// Fix 1: only cap with LimitReader when maxAudioBytes is set
	reader := io.Reader(resp.Body)
	if tg.maxAudioBytes > 0 {
		reader = io.LimitReader(resp.Body, tg.maxAudioBytes+1)
	}

	var buf bytes.Buffer
	if _, err = io.Copy(&buf, reader); err != nil {
		log.Error("voice: read body failed", slog.Any("error", err))
		rearm()
		tg.sendVoiceError(ctx, bot, log, chatID, "Could not transcribe your voice message. Please try again or type it.")
		return "", false
	}

	// Fix 6: pass actual MIME type so Whisper gets the right filename hint
	transcribed, err := tg.transcribe.Transcribe(ctx, identity, &buf, int64(buf.Len()), voice.MimeType)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrAudioTooLarge):
			// Fix 3: re-arm so user can send a shorter note or type
			rearm()
			tg.sendVoiceError(ctx, bot, log, chatID, "Your voice message is too large. Please send a shorter one or type it.")
		case errors.Is(err, ai.ErrAudioBudgetExceeded):
			tg.sendVoiceError(ctx, bot, log, chatID, "You have reached your daily AI usage limit. Try again tomorrow.")
		default:
			log.Error("voice: transcription failed", slog.Any("error", err))
			rearm()
			tg.sendVoiceError(ctx, bot, log, chatID, "Could not transcribe your voice message. Please try again or type it.")
		}
		return "", false
	}

	return transcribed, true
}

func (tg *TgHandlers) sendVoiceError(ctx context.Context, bot *tgbot.Bot, log *slog.Logger, chatID int64, text string) {
	if _, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}); err != nil {
		log.Error("voice: send reply", slog.Any("error", err))
	}
}
