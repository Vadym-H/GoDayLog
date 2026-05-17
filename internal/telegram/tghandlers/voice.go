package tghandlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Vadym-H/GoDayLog/internal/ai"
	"github.com/Vadym-H/GoDayLog/internal/domain"
	"github.com/Vadym-H/GoDayLog/internal/logger"
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

	file, err := bot.GetFile(ctx, &tgbot.GetFileParams{FileID: update.Message.Voice.FileID})
	if err != nil {
		log.Error("voice: get file failed", slog.Any("error", err))
		tg.setAwaitingLog(chatID)
		if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not transcribe your voice message. Please try again or type it.",
		}); sendErr != nil {
			log.Error("voice: send error reply", slog.Any("error", sendErr))
		}
		return true
	}

	downloadURL := bot.FileDownloadLink(file)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		log.Error("voice: build download request", slog.Any("error", err))
		tg.setAwaitingLog(chatID)
		if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not transcribe your voice message. Please try again or type it.",
		}); sendErr != nil {
			log.Error("voice: send error reply", slog.Any("error", sendErr))
		}
		return true
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("voice: download failed", slog.Any("error", err))
		tg.setAwaitingLog(chatID)
		if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not transcribe your voice message. Please try again or type it.",
		}); sendErr != nil {
			log.Error("voice: send error reply", slog.Any("error", sendErr))
		}
		return true
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err = io.Copy(&buf, io.LimitReader(resp.Body, tg.maxAudioBytes+1)); err != nil {
		log.Error("voice: read body failed", slog.Any("error", err))
		tg.setAwaitingLog(chatID)
		if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Could not transcribe your voice message. Please try again or type it.",
		}); sendErr != nil {
			log.Error("voice: send error reply", slog.Any("error", sendErr))
		}
		return true
	}

	identity := domain.Identity{Provider: "telegram", ExternalID: strconv.FormatInt(update.Message.From.ID, 10)}
	externalMessageID := strconv.Itoa(update.Message.ID)

	text, err := tg.transcribe.Transcribe(ctx, identity, &buf, int64(buf.Len()))
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrAudioTooLarge):
			if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Your voice message is too large. Please send a shorter one or type it.",
			}); sendErr != nil {
				log.Error("voice: send size error reply", slog.Any("error", sendErr))
			}
		case errors.Is(err, ai.ErrAudioBudgetExceeded):
			if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "You have reached your daily AI usage limit. Try again tomorrow.",
			}); sendErr != nil {
				log.Error("voice: send budget error reply", slog.Any("error", sendErr))
			}
		default:
			log.Error("voice: transcription failed", slog.Any("error", err))
			tg.setAwaitingLog(chatID)
			if _, sendErr := bot.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Could not transcribe your voice message. Please try again or type it.",
			}); sendErr != nil {
				log.Error("voice: send error reply", slog.Any("error", sendErr))
			}
		}
		return true
	}

	tg.processLogText(ctx, bot, chatID, identity, externalMessageID, text)
	return true
}
