package bot

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/services"
	storage "github.com/Vadym-H/GoDayLog/internal/storage/postgres"
	"github.com/Vadym-H/GoDayLog/internal/telegram/tghandlers"
	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Bot struct {
	tg         *tgbot.Bot
	log        *slog.Logger
	tgHandlers *tghandlers.TgHandlers
}

func New(token string, log *slog.Logger, storage *storage.Storage) (*Bot, error) {
	userService := services.NewUserService(log, storage)

	b := &Bot{
		log:        log,
		tgHandlers: tghandlers.New(log, userService),
	}

	tg, err := tgbot.New(token, tgbot.WithDefaultHandler(b.handleMessage))
	if err != nil {
		return nil, err
	}

	b.tg = tg

	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/ping", tgbot.MatchTypeExact, b.tgHandlers.HandlePing)
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeExact, b.tgHandlers.HandleStart)
	b.tg.RegisterHandler(tgbot.HandlerTypeMessageText, "/menu", tgbot.MatchTypeExact, b.tgHandlers.HandleMenu)

	return b, nil
}

func (b *Bot) Start(ctx context.Context) {
	b.log.Info("telegram bot started")
	b.tg.Start(ctx)
	b.log.Info("telegram bot stopped")
}

func (b *Bot) handleMessage(ctx context.Context, bot *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	_, err := bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Try /ping",
	})
	if err != nil {
		b.log.Error("failed to send default response", slog.String("error", err.Error()))
	}
}
